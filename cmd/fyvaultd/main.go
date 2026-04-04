package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/boot"
	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/config"
	"github.com/fybyte/fyvault-agent/internal/enclave"
	fvebpf "github.com/fybyte/fyvault-agent/internal/ebpf"
	"github.com/fybyte/fyvault-agent/internal/health"
	"github.com/fybyte/fyvault-agent/internal/keyring"
	"github.com/fybyte/fyvault-agent/internal/privilege"
	"github.com/fybyte/fyvault-agent/internal/proxy"
	fvsync "github.com/fybyte/fyvault-agent/internal/sync"
)

var (
	version    = "dev"
	configPath = flag.String("config", config.DefaultConfigPath(), "config file path")
)

func main() {
	flag.Parse()

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	cfg.Agent.Version = version

	// Initialise logger.
	var logger *zap.Logger
	if cfg.Agent.LogLevel == "debug" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("fyvaultd starting", zap.String("version", version))

	// Detect confidential computing environment.
	detection := enclave.Detect()
	logger.Info("confidential computing detection",
		zap.String("level", string(detection.Level)),
		zap.String("platform", detection.Platform))

	// Initialise kernel keyring.
	kr, err := keyring.New(cfg.Keyring.Namespace)
	if err != nil {
		logger.Fatal("failed to init keyring", zap.Error(err))
	}

	// Initialise cloud client.
	client, err := cloud.New(cfg, logger)
	if err != nil {
		logger.Fatal("failed to init cloud client", zap.Error(err))
	}

	// Run boot sequence (fetch secrets, load into keyring).
	orch := boot.New(cfg, client, kr, logger)
	bootResp, err := orch.Run()
	if err != nil {
		logger.Fatal("boot sequence failed", zap.Error(err))
	}

	// Start proxy manager.
	proxyMgr := proxy.NewManager(kr, logger)
	if err := proxyMgr.Configure(bootResp.Secrets); err != nil {
		logger.Fatal("proxy configuration failed", zap.Error(err))
	}
	if err := proxyMgr.StartAll(); err != nil {
		logger.Fatal("proxy start failed", zap.Error(err))
	}
	defer proxyMgr.StopAll()

	// Load eBPF program (Linux only - stub on macOS).
	ebpfProg, err := fvebpf.Load(cfg.Network.Interface, logger)
	if err != nil {
		logger.Warn("eBPF load failed (non-fatal on dev)", zap.Error(err))
	} else {
		defer ebpfProg.Close()

		// Resolve proxy targets to IPs and populate eBPF map.
		targets := proxyMgr.Targets()
		resolver := fvebpf.NewResolver(targets, ebpfProg, logger)
		if err := resolver.Start(); err != nil {
			logger.Warn("DNS resolver start failed", zap.Error(err))
		} else {
			defer resolver.Stop()
		}
	}

	// Drop elevated privileges now that eBPF is attached.
	if err := privilege.Drop(logger); err != nil {
		logger.Warn("privilege drop failed", zap.Error(err))
	}

	// Start health server on Unix socket.
	healthSrv := health.New(cfg.Agent.HealthSocket, kr, logger)
	if err := healthSrv.Start(); err != nil {
		logger.Fatal("failed to start health server", zap.Error(err))
	}

	// Start sync manager (replaces simple heartbeat loop).
	interval := time.Duration(bootResp.RefreshIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Duration(cfg.Agent.HeartbeatInterval) * time.Second
	}
	syncMgr := fvsync.New(client, kr, proxyMgr, cfg.Agent.Version, logger)
	syncMgr.Start(interval)
	defer syncMgr.Stop()

	logger.Info("fyvaultd ready", zap.Int("secrets", len(bootResp.Secrets)))

	// Block until shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh

	logger.Info("shutting down", zap.String("signal", sig.String()))
	healthSrv.Stop()
	kr.FlushAll()
	logger.Info("fyvaultd stopped")
}
