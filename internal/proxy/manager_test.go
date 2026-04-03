package proxy

import (
	"encoding/json"
	"testing"

	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/keyring"
	"go.uber.org/zap"
)

func TestManagerConfigure(t *testing.T) {
	kr, _ := keyring.New("test-mgr")
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(kr, logger)

	httpCfg, _ := json.Marshal(httpProxyInjectionConfig{
		TargetHost: "api.example.com",
		TargetPort: 443,
	})
	dbCfg, _ := json.Marshal(dbProxyInjectionConfig{
		TargetHost: "db.example.com",
		TargetPort: 5432,
	})

	secrets := []cloud.BootSecret{
		{Name: "API_KEY", SecretType: "API_KEY", InjectionConfig: httpCfg},
		{Name: "DB_PASS", SecretType: "DB_CREDENTIAL", InjectionConfig: dbCfg},
		{Name: "GENERIC_KEY", SecretType: "GENERIC"},    // should be skipped
		{Name: "AWS_KEY", SecretType: "AWS_CREDENTIAL"},  // should be skipped
	}

	err := mgr.Configure(secrets)
	if err != nil {
		t.Fatal(err)
	}

	// Only API_KEY and DB_CREDENTIAL should create proxies
	if len(mgr.proxies) != 2 {
		t.Errorf("proxy count = %d, want 2", len(mgr.proxies))
	}
}

func TestManagerStartStopAll(t *testing.T) {
	kr, _ := keyring.New("test-mgr")
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(kr, logger)

	httpCfg, _ := json.Marshal(httpProxyInjectionConfig{
		TargetHost: "api.example.com",
		ProxyPort:  0,
	})

	secrets := []cloud.BootSecret{
		{Name: "KEY1", SecretType: "API_KEY", InjectionConfig: httpCfg},
	}

	mgr.Configure(secrets)

	if err := mgr.StartAll(); err != nil {
		t.Fatal(err)
	}

	// After start, targets should be available
	targets := mgr.Targets()
	if len(targets) != 1 {
		t.Errorf("targets count = %d, want 1", len(targets))
	}

	mgr.StopAll()
}

func TestManagerConfigureInvalidSecret(t *testing.T) {
	kr, _ := keyring.New("test-mgr")
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(kr, logger)

	// Invalid injection config should be skipped (logged as warning)
	secrets := []cloud.BootSecret{
		{Name: "BAD_KEY", SecretType: "API_KEY", InjectionConfig: json.RawMessage(`{invalid}`)},
	}

	err := mgr.Configure(secrets)
	if err != nil {
		t.Fatal(err)
	}

	// Invalid secrets should be skipped
	if len(mgr.proxies) != 0 {
		t.Errorf("proxy count = %d, want 0 (invalid should be skipped)", len(mgr.proxies))
	}
}

func TestManagerEmpty(t *testing.T) {
	kr, _ := keyring.New("test-mgr")
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(kr, logger)

	// Configure with no secrets
	mgr.Configure(nil)

	if err := mgr.StartAll(); err != nil {
		t.Fatal(err)
	}

	targets := mgr.Targets()
	if len(targets) != 0 {
		t.Errorf("targets count = %d, want 0", len(targets))
	}

	// StopAll on empty should not panic
	mgr.StopAll()
}
