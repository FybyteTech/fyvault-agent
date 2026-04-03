package sync

import (
	"context"
	gosync "sync"
	"time"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/keyring"
	"github.com/fybyte/fyvault-agent/internal/proxy"
)

// SyncManager runs a background loop that heartbeats the cloud API
// and syncs any stale secrets back into the local keyring.
type SyncManager struct {
	client       *cloud.Client
	keyring      *keyring.Keyring
	proxyMgr     *proxy.Manager
	agentVersion string
	logger       *zap.Logger
	cancel       context.CancelFunc
	wg           gosync.WaitGroup
}

// New creates a SyncManager.
func New(client *cloud.Client, kr *keyring.Keyring, proxyMgr *proxy.Manager, agentVersion string, logger *zap.Logger) *SyncManager {
	return &SyncManager{
		client:       client,
		keyring:      kr,
		proxyMgr:     proxyMgr,
		agentVersion: agentVersion,
		logger:       logger,
	}
}

// Start launches the background sync loop at the given interval.
func (s *SyncManager) Start(interval time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go s.loop(ctx, interval)
}

func (s *SyncManager) loop(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *SyncManager) tick() {
	// 1. Heartbeat
	resp, err := s.client.Heartbeat(s.agentVersion)
	if err != nil {
		s.logger.Warn("heartbeat failed", zap.Error(err))
		return
	}

	// 2. Check if sync needed
	if !resp.NeedsSync || len(resp.StaleSecrets) == 0 {
		return
	}

	s.logger.Info("sync needed", zap.Int("stale_secrets", len(resp.StaleSecrets)))

	// 3. Collect stale secret names
	names := make([]string, len(resp.StaleSecrets))
	for i, ss := range resp.StaleSecrets {
		names[i] = ss.Name
	}

	// 4. Fetch updated secrets
	syncResp, err := s.client.Sync(names)
	if err != nil {
		s.logger.Warn("sync fetch failed", zap.Error(err))
		return
	}

	// 5. Update keyring
	for _, secret := range syncResp.Secrets {
		var value []byte
		if secret.EncryptionMode == "server" {
			value = []byte(secret.Value)
		} else {
			value = []byte(secret.DeviceEncryptedValue)
		}
		if err := s.keyring.Store(secret.Name, value); err != nil {
			s.logger.Error("failed to update secret in keyring",
				zap.String("name", secret.Name),
				zap.Error(err))
			continue
		}
		s.logger.Info("secret synced", zap.String("name", secret.Name))
	}
}

// Stop gracefully shuts down the sync loop.
func (s *SyncManager) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}
