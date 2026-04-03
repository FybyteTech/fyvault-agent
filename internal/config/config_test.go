package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	os.WriteFile(path, []byte(`
[cloud]
url = "https://test.fyvault.io/api/v1"
token = "test-token-123"
device_cert = "/etc/fyvault/cert.pem"
device_key = "/etc/fyvault/key.pem"
ca_cert = "/etc/fyvault/ca.pem"

[agent]
heartbeat_interval = 60
log_level = "debug"
health_socket = "/tmp/fyvault.sock"

[keyring]
namespace = "test"

[network]
interface = "eth0"
`), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Cloud.URL != "https://test.fyvault.io/api/v1" {
		t.Errorf("Cloud.URL = %q, want %q", cfg.Cloud.URL, "https://test.fyvault.io/api/v1")
	}
	if cfg.Cloud.Token != "test-token-123" {
		t.Errorf("Cloud.Token = %q, want %q", cfg.Cloud.Token, "test-token-123")
	}
	if cfg.Cloud.DeviceCert != "/etc/fyvault/cert.pem" {
		t.Errorf("Cloud.DeviceCert = %q", cfg.Cloud.DeviceCert)
	}
	if cfg.Cloud.DeviceKey != "/etc/fyvault/key.pem" {
		t.Errorf("Cloud.DeviceKey = %q", cfg.Cloud.DeviceKey)
	}
	if cfg.Cloud.CACert != "/etc/fyvault/ca.pem" {
		t.Errorf("Cloud.CACert = %q", cfg.Cloud.CACert)
	}
	if cfg.Agent.HeartbeatInterval != 60 {
		t.Errorf("Agent.HeartbeatInterval = %d, want 60", cfg.Agent.HeartbeatInterval)
	}
	if cfg.Agent.LogLevel != "debug" {
		t.Errorf("Agent.LogLevel = %q, want %q", cfg.Agent.LogLevel, "debug")
	}
	if cfg.Agent.HealthSocket != "/tmp/fyvault.sock" {
		t.Errorf("Agent.HealthSocket = %q", cfg.Agent.HealthSocket)
	}
	if cfg.Keyring.Namespace != "test" {
		t.Errorf("Keyring.Namespace = %q, want %q", cfg.Keyring.Namespace, "test")
	}
	if cfg.Network.Interface != "eth0" {
		t.Errorf("Network.Interface = %q, want %q", cfg.Network.Interface, "eth0")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minimal.toml")
	os.WriteFile(path, []byte(`
[cloud]
url = "https://api.fyvault.io"
`), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.HeartbeatInterval != 300 {
		t.Errorf("default HeartbeatInterval = %d, want 300", cfg.Agent.HeartbeatInterval)
	}
	if cfg.Agent.LogLevel != "info" {
		t.Errorf("default LogLevel = %q, want %q", cfg.Agent.LogLevel, "info")
	}
	if cfg.Agent.HealthSocket != "/var/run/fyvault/health.sock" {
		t.Errorf("default HealthSocket = %q", cfg.Agent.HealthSocket)
	}
	if cfg.Keyring.Namespace != "fyvault" {
		t.Errorf("default Keyring.Namespace = %q, want %q", cfg.Keyring.Namespace, "fyvault")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path.toml")
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	os.WriteFile(path, []byte(`this is not valid toml {{{{`), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}

func TestLoadConfigEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.toml")
	os.WriteFile(path, []byte(``), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// All defaults should be applied
	if cfg.Agent.HeartbeatInterval != 300 {
		t.Errorf("expected default heartbeat interval 300, got %d", cfg.Agent.HeartbeatInterval)
	}
	if cfg.Keyring.Namespace != "fyvault" {
		t.Errorf("expected default namespace, got %q", cfg.Keyring.Namespace)
	}
}
