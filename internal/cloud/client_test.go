package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fybyte/fyvault-agent/internal/config"
	"github.com/fybyte/fyvault-agent/internal/enclave"
	"go.uber.org/zap"
)

// stdDetection returns a standard (non-confidential) detection for tests.
func stdDetection() enclave.Detection {
	return enclave.Detection{Level: enclave.LevelStandard, Platform: "none"}
}

func testClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &config.Config{
		Cloud: config.CloudConfig{URL: server.URL, Token: "test-token"},
	}
	logger, _ := zap.NewDevelopment()
	client, err := New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	return client, server
}

func TestClientBoot(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/boot" {
			t.Errorf("path = %s, want /boot", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s", r.Header.Get("Content-Type"))
		}

		// Verify Bearer token is sent (since no mTLS fingerprint)
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"secrets": []map[string]interface{}{
					{
						"name":            "TEST_KEY",
						"secretType":      "API_KEY",
						"encryptionMode":  "server",
						"value":           "sk-123",
						"injectionConfig": map[string]string{"target_host": "api.test.com"},
					},
				},
				"refreshIntervalSeconds": 300,
			},
		})
	}))
	defer server.Close()

	resp, err := client.Boot("0.1.0", "test-host", stdDetection(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Secrets) != 1 {
		t.Fatalf("secrets count = %d, want 1", len(resp.Secrets))
	}
	if resp.Secrets[0].Name != "TEST_KEY" {
		t.Errorf("secret name = %q, want %q", resp.Secrets[0].Name, "TEST_KEY")
	}
	if resp.Secrets[0].Value != "sk-123" {
		t.Errorf("secret value = %q, want %q", resp.Secrets[0].Value, "sk-123")
	}
	if resp.Secrets[0].SecretType != "API_KEY" {
		t.Errorf("secret type = %q, want %q", resp.Secrets[0].SecretType, "API_KEY")
	}
	if resp.RefreshIntervalSeconds != 300 {
		t.Errorf("refresh interval = %d, want 300", resp.RefreshIntervalSeconds)
	}
}

func TestClientBootMultipleSecrets(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"secrets": []map[string]interface{}{
					{"name": "KEY_A", "secretType": "API_KEY", "value": "a"},
					{"name": "KEY_B", "secretType": "DB_CREDENTIAL", "value": "b"},
					{"name": "KEY_C", "secretType": "GENERIC", "value": "c"},
				},
				"refreshIntervalSeconds": 120,
			},
		})
	}))
	defer server.Close()

	resp, err := client.Boot("0.1.0", "host", stdDetection(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Secrets) != 3 {
		t.Errorf("secrets count = %d, want 3", len(resp.Secrets))
	}
}

func TestClientBootAPIError(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "device not found",
		})
	}))
	defer server.Close()

	_, err := client.Boot("0.1.0", "host", stdDetection(), nil)
	if err == nil {
		t.Error("expected error for API failure, got nil")
	}
}

func TestClientBootHTTPError(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	_, err := client.Boot("0.1.0", "host", stdDetection(), nil)
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestClientHeartbeat(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/heartbeat" {
			t.Errorf("path = %s, want /heartbeat", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"status":       "ok",
				"needsSync":    false,
				"staleSecrets": []interface{}{},
			},
		})
	}))
	defer server.Close()

	resp, err := client.Heartbeat("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
	if resp.NeedsSync {
		t.Error("NeedsSync should be false")
	}
	if len(resp.StaleSecrets) != 0 {
		t.Errorf("stale secrets count = %d, want 0", len(resp.StaleSecrets))
	}
}

func TestClientHeartbeatNeedsSync(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"status":    "ok",
				"needsSync": true,
				"staleSecrets": []map[string]interface{}{
					{"name": "OLD_KEY", "secretType": "API_KEY", "currentVersion": 3, "syncedVersion": 1},
				},
			},
		})
	}))
	defer server.Close()

	resp, err := client.Heartbeat("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.NeedsSync {
		t.Error("NeedsSync should be true")
	}
	if len(resp.StaleSecrets) != 1 {
		t.Fatalf("stale secrets count = %d, want 1", len(resp.StaleSecrets))
	}
	if resp.StaleSecrets[0].Name != "OLD_KEY" {
		t.Errorf("stale secret name = %q", resp.StaleSecrets[0].Name)
	}
	if resp.StaleSecrets[0].CurrentVersion != 3 {
		t.Errorf("current version = %d, want 3", resp.StaleSecrets[0].CurrentVersion)
	}
}

func TestClientSync(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sync" {
			t.Errorf("path = %s, want /sync", r.URL.Path)
		}

		// Verify the request body contains secretNames
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		names, ok := body["secretNames"].([]interface{})
		if !ok || len(names) != 2 {
			t.Errorf("expected 2 secret names in body")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"secrets": []map[string]interface{}{
					{"name": "KEY_A", "secretType": "API_KEY", "value": "new-a"},
					{"name": "KEY_B", "secretType": "GENERIC", "value": "new-b"},
				},
			},
		})
	}))
	defer server.Close()

	resp, err := client.Sync([]string{"KEY_A", "KEY_B"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Secrets) != 2 {
		t.Errorf("synced secrets count = %d, want 2", len(resp.Secrets))
	}
}

func TestClientServerDown(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{URL: "http://127.0.0.1:1", Token: "t"},
	}
	logger, _ := zap.NewDevelopment()
	client, err := New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Boot("0.1.0", "host", stdDetection(), nil)
	if err == nil {
		t.Error("expected error when server is unreachable")
	}
}
