package ebpf

import (
	"context"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/proxy"
)

// Resolver periodically resolves target hostnames to IP addresses and
// populates the eBPF redirect map.
type Resolver struct {
	targets []proxy.Target
	program *Program
	logger  *zap.Logger
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewResolver creates a Resolver that maps proxy targets into the eBPF program.
func NewResolver(targets []proxy.Target, program *Program, logger *zap.Logger) *Resolver {
	return &Resolver{targets: targets, program: program, logger: logger}
}

// Start resolves all target hosts to IPs and populates the eBPF map,
// then starts a background goroutine to re-resolve every 60 seconds.
func (r *Resolver) Start() error {
	// Initial resolve
	if err := r.resolve(); err != nil {
		return err
	}

	// Background re-resolution
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.resolve(); err != nil {
					r.logger.Warn("DNS re-resolution failed", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

func (r *Resolver) resolve() error {
	for _, t := range r.targets {
		ips, err := net.LookupIP(t.DestHost)
		if err != nil {
			r.logger.Warn("DNS lookup failed",
				zap.String("host", t.DestHost), zap.Error(err))
			continue
		}

		for _, ip := range ips {
			ip4 := ip.To4()
			if ip4 == nil {
				continue // skip IPv6 for now
			}

			if err := r.program.AddTarget(ip4, t.DestPort, t.ProxyPort); err != nil {
				r.logger.Warn("failed to add eBPF target",
					zap.String("ip", ip4.String()), zap.Error(err))
			} else {
				r.logger.Debug("eBPF target added",
					zap.String("host", t.DestHost),
					zap.String("ip", ip4.String()),
					zap.Uint16("port", t.DestPort),
					zap.Uint16("proxy", t.ProxyPort))
			}
		}
	}
	return nil
}

// Stop cancels the background re-resolution goroutine and waits for it to exit.
func (r *Resolver) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}
