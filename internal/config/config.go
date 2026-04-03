package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config holds the complete agent configuration.
type Config struct {
	Cloud   CloudConfig
	Agent   AgentConfig
	Keyring KeyringConfig
	Network NetworkConfig
}

// CloudConfig holds cloud API connection settings.
type CloudConfig struct {
	URL        string `toml:"url"`
	DeviceCert string `toml:"device_cert"`
	DeviceKey  string `toml:"device_key"`
	CACert     string `toml:"ca_cert"`
	Token      string `toml:"token"` // device token for non-mTLS auth
}

// AgentConfig holds agent runtime settings.
type AgentConfig struct {
	HeartbeatInterval int    `toml:"heartbeat_interval"` // seconds
	LogLevel          string `toml:"log_level"`
	HealthSocket      string `toml:"health_socket"`
	Version           string // set at build time, not from config
}

// KeyringConfig holds kernel keyring settings.
type KeyringConfig struct {
	Namespace string `toml:"namespace"`
}

// NetworkConfig holds network filtering settings.
type NetworkConfig struct {
	Interface string `toml:"interface"`
}

// Load reads and parses a TOML config file, applying defaults for unset values.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	applyDefaults(cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Agent.HeartbeatInterval == 0 {
		cfg.Agent.HeartbeatInterval = 300
	}
	if cfg.Agent.LogLevel == "" {
		cfg.Agent.LogLevel = "info"
	}
	if cfg.Agent.HealthSocket == "" {
		cfg.Agent.HealthSocket = "/var/run/fyvault/health.sock"
	}
	if cfg.Keyring.Namespace == "" {
		cfg.Keyring.Namespace = "fyvault"
	}
}
