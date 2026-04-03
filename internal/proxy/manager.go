package proxy

import (
	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/keyring"
)

// Manager manages the lifecycle of all proxy instances.
type Manager struct {
	proxies []Proxy
	keyring *keyring.Keyring
	logger  *zap.Logger
}

// Proxy is the interface that all proxy types must implement.
type Proxy interface {
	Start() error
	Stop() error
	Name() string
	ListenAddr() string
}

// Target represents a destination-to-proxy mapping for eBPF redirect rules.
type Target struct {
	DestHost  string
	DestPort  uint16
	ProxyPort uint16
}

// NewManager creates a proxy lifecycle manager.
func NewManager(kr *keyring.Keyring, logger *zap.Logger) *Manager {
	return &Manager{
		keyring: kr,
		logger:  logger,
	}
}

// Configure parses boot secrets and creates appropriate proxy instances.
func (m *Manager) Configure(secrets []cloud.BootSecret) error {
	for _, s := range secrets {
		switch s.SecretType {
		case "API_KEY":
			p, err := NewHTTPProxy(s, m.keyring, m.logger)
			if err != nil {
				m.logger.Warn("failed to create HTTP proxy",
					zap.String("secret", s.Name), zap.Error(err))
				continue
			}
			m.proxies = append(m.proxies, p)
		case "DB_CREDENTIAL":
			p, err := NewDBProxy(s, m.keyring, m.logger)
			if err != nil {
				m.logger.Warn("failed to create DB proxy",
					zap.String("secret", s.Name), zap.Error(err))
				continue
			}
			m.proxies = append(m.proxies, p)
		// AWS_CREDENTIAL and GENERIC don't need proxies.
		}
	}
	return nil
}

// StartAll starts every configured proxy.
func (m *Manager) StartAll() error {
	for _, p := range m.proxies {
		if err := p.Start(); err != nil {
			m.logger.Error("proxy start failed",
				zap.String("name", p.Name()), zap.Error(err))
			return err
		}
		m.logger.Info("proxy started",
			zap.String("name", p.Name()),
			zap.String("addr", p.ListenAddr()))
	}
	return nil
}

// StopAll stops every running proxy.
func (m *Manager) StopAll() {
	for _, p := range m.proxies {
		p.Stop()
		m.logger.Info("proxy stopped", zap.String("name", p.Name()))
	}
}

// Targets returns destination IP:port to local proxy port mappings for eBPF.
func (m *Manager) Targets() []Target {
	var targets []Target
	for _, p := range m.proxies {
		if hp, ok := p.(*HTTPProxy); ok {
			targets = append(targets, hp.Targets()...)
		}
	}
	return targets
}
