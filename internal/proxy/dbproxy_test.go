package proxy

import (
	"encoding/json"
	"testing"

	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/keyring"
	"go.uber.org/zap"
)

func TestDBProxyCreation(t *testing.T) {
	kr, _ := keyring.New("test-db")
	logger, _ := zap.NewDevelopment()

	injCfg, _ := json.Marshal(dbProxyInjectionConfig{
		DBType:     "postgresql",
		TargetHost: "db.example.com",
		TargetPort: 5432,
		ProxyPort:  0,
		Username:   "app_user",
		Database:   "mydb",
	})

	secret := cloud.BootSecret{
		Name:            "DB_PASS",
		SecretType:      "DB_CREDENTIAL",
		InjectionConfig: injCfg,
	}

	proxy, err := NewDBProxy(secret, kr, logger)
	if err != nil {
		t.Fatal(err)
	}

	if proxy.config.TargetHost != "db.example.com" {
		t.Errorf("TargetHost = %q", proxy.config.TargetHost)
	}
	if proxy.config.TargetPort != 5432 {
		t.Errorf("TargetPort = %d", proxy.config.TargetPort)
	}
	if proxy.config.Username != "app_user" {
		t.Errorf("Username = %q", proxy.config.Username)
	}
	if proxy.config.Database != "mydb" {
		t.Errorf("Database = %q", proxy.config.Database)
	}
	if proxy.config.DBType != "postgresql" {
		t.Errorf("DBType = %q", proxy.config.DBType)
	}
	if proxy.Name() != "db-proxy-DB_PASS" {
		t.Errorf("Name() = %q", proxy.Name())
	}
}

func TestDBProxyDefaults(t *testing.T) {
	kr, _ := keyring.New("test-db")
	logger, _ := zap.NewDevelopment()

	injCfg, _ := json.Marshal(dbProxyInjectionConfig{
		TargetHost: "db.example.com",
		// DBType and TargetPort omitted
	})

	secret := cloud.BootSecret{
		Name:            "DB_KEY",
		SecretType:      "DB_CREDENTIAL",
		InjectionConfig: injCfg,
	}

	proxy, err := NewDBProxy(secret, kr, logger)
	if err != nil {
		t.Fatal(err)
	}

	if proxy.config.DBType != "postgresql" {
		t.Errorf("default DBType = %q, want postgresql", proxy.config.DBType)
	}
	if proxy.config.TargetPort != 5432 {
		t.Errorf("default TargetPort = %d, want 5432", proxy.config.TargetPort)
	}
}

func TestDBProxyMissingTargetHost(t *testing.T) {
	kr, _ := keyring.New("test-db")
	logger, _ := zap.NewDevelopment()

	injCfg, _ := json.Marshal(dbProxyInjectionConfig{
		// TargetHost omitted
		TargetPort: 5432,
	})

	secret := cloud.BootSecret{
		Name:            "DB_KEY",
		SecretType:      "DB_CREDENTIAL",
		InjectionConfig: injCfg,
	}

	_, err := NewDBProxy(secret, kr, logger)
	if err == nil {
		t.Error("expected error for missing target_host")
	}
}

func TestDBProxyInvalidInjectionConfig(t *testing.T) {
	kr, _ := keyring.New("test-db")
	logger, _ := zap.NewDevelopment()

	secret := cloud.BootSecret{
		Name:            "DB_KEY",
		SecretType:      "DB_CREDENTIAL",
		InjectionConfig: json.RawMessage(`not valid json`),
	}

	_, err := NewDBProxy(secret, kr, logger)
	if err == nil {
		t.Error("expected error for invalid injection config JSON")
	}
}

func TestDBProxyStartStop(t *testing.T) {
	kr, _ := keyring.New("test-db")
	logger, _ := zap.NewDevelopment()

	injCfg, _ := json.Marshal(dbProxyInjectionConfig{
		TargetHost: "db.example.com",
		TargetPort: 5432,
		ProxyPort:  0,
	})

	secret := cloud.BootSecret{
		Name:            "DB_KEY",
		SecretType:      "DB_CREDENTIAL",
		InjectionConfig: injCfg,
	}

	proxy, err := NewDBProxy(secret, kr, logger)
	if err != nil {
		t.Fatal(err)
	}

	if err := proxy.Start(); err != nil {
		t.Fatal(err)
	}

	addr := proxy.ListenAddr()
	if addr == "" {
		t.Error("ListenAddr is empty after Start")
	}
	if addr == "127.0.0.1:0" {
		t.Error("ProxyPort was not updated after Start")
	}

	if err := proxy.Stop(); err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
}

func TestPGMD5Password(t *testing.T) {
	// Verify the MD5 password hash matches known PostgreSQL behavior
	// PostgreSQL MD5: "md5" + md5(md5(password + user) + salt)
	salt := []byte{0x01, 0x02, 0x03, 0x04}
	result := pgMD5Password("testuser", "testpass", salt)

	// Should start with "md5"
	if len(result) < 3 || result[:3] != "md5" {
		t.Errorf("pgMD5Password result = %q, should start with 'md5'", result)
	}
	// Should be "md5" + 32 hex chars = 35 chars total
	if len(result) != 35 {
		t.Errorf("pgMD5Password result length = %d, want 35", len(result))
	}

	// Same inputs should produce same output
	result2 := pgMD5Password("testuser", "testpass", salt)
	if result != result2 {
		t.Error("pgMD5Password is not deterministic")
	}

	// Different salt should produce different output
	salt2 := []byte{0x05, 0x06, 0x07, 0x08}
	result3 := pgMD5Password("testuser", "testpass", salt2)
	if result == result3 {
		t.Error("different salts produced same hash")
	}
}

func TestExtractPGErrorMessage(t *testing.T) {
	// Build a simple ErrorResponse body with S(severity) and M(message) fields
	var body []byte
	body = append(body, 'S')
	body = append(body, []byte("ERROR")...)
	body = append(body, 0)
	body = append(body, 'M')
	body = append(body, []byte("test error message")...)
	body = append(body, 0)
	body = append(body, 0) // terminator

	msg := extractPGErrorMessage(body)
	if msg != "test error message" {
		t.Errorf("extractPGErrorMessage = %q, want %q", msg, "test error message")
	}
}

func TestExtractPGErrorMessageMissingField(t *testing.T) {
	// Body with only severity, no message
	var body []byte
	body = append(body, 'S')
	body = append(body, []byte("ERROR")...)
	body = append(body, 0)
	body = append(body, 0) // terminator

	msg := extractPGErrorMessage(body)
	if msg != "unknown error" {
		t.Errorf("extractPGErrorMessage = %q, want %q", msg, "unknown error")
	}
}
