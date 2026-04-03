package boot

import (
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/config"
	"github.com/fybyte/fyvault-agent/internal/enclave"
	"github.com/fybyte/fyvault-agent/internal/keyring"
)

// Orchestrator runs the agent boot sequence: fetch secrets and load them into the keyring.
type Orchestrator struct {
	cfg     *config.Config
	client  *cloud.Client
	keyring *keyring.Keyring
	logger  *zap.Logger
}

// New creates a boot orchestrator.
func New(cfg *config.Config, client *cloud.Client, kr *keyring.Keyring, logger *zap.Logger) *Orchestrator {
	return &Orchestrator{
		cfg:     cfg,
		client:  client,
		keyring: kr,
		logger:  logger,
	}
}

// Run executes the boot sequence: calls the cloud boot endpoint and stores
// each returned secret in the kernel keyring.
func (o *Orchestrator) Run() (*cloud.BootResponse, error) {
	// 1. Detect security environment
	detection := enclave.Detect()
	o.logger.Info("security environment detected",
		zap.String("level", string(detection.Level)),
		zap.String("platform", detection.Platform))

	// 2. Generate attestation if on confidential hardware
	attestation := enclave.GenerateAttestation(detection, "")

	// 3. Boot with security info
	hostname, _ := os.Hostname()

	resp, err := o.client.Boot(o.cfg.Agent.Version, hostname, detection, attestation)
	if err != nil {
		return nil, fmt.Errorf("boot failed: %w", err)
	}

	for _, secret := range resp.Secrets {
		var value []byte
		switch secret.EncryptionMode {
		case "server":
			value = []byte(secret.Value)
		case "client":
			// Client-encrypted secrets need device private key decryption.
			// Phase 2 will add proper decryption; store raw for now.
			value = []byte(secret.DeviceEncryptedValue)
		default:
			o.logger.Warn("unknown encryption mode, skipping",
				zap.String("name", secret.Name),
				zap.String("mode", secret.EncryptionMode))
			continue
		}

		if err := o.keyring.Store(secret.Name, value); err != nil {
			o.logger.Error("failed to store secret",
				zap.String("name", secret.Name),
				zap.Error(err))
			continue
		}
		o.logger.Info("stored secret",
			zap.String("name", secret.Name),
			zap.String("type", secret.SecretType))
	}

	o.logger.Info("boot complete", zap.Int("secrets", len(resp.Secrets)))
	return resp, nil
}
