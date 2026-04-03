package cloud

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/config"
	"github.com/fybyte/fyvault-agent/internal/enclave"
)

// Client communicates with the FyVault cloud API.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	fingerprint string
	token       string
	logger      *zap.Logger
}

// BootResponse is the data payload from the boot endpoint.
type BootResponse struct {
	Secrets                []BootSecret `json:"secrets"`
	RefreshIntervalSeconds int          `json:"refreshIntervalSeconds"`
}

// BootSecret represents a single secret returned during boot.
type BootSecret struct {
	Name                 string          `json:"name"`
	SecretType           string          `json:"secretType"`
	InjectionConfig      json.RawMessage `json:"injectionConfig"`
	EncryptionMode       string          `json:"encryptionMode"`
	Value                string          `json:"value,omitempty"`
	DeviceEncryptedValue string          `json:"deviceEncryptedValue,omitempty"`
}

// HeartbeatResponse is the data payload from the heartbeat endpoint.
type HeartbeatResponse struct {
	Status       string        `json:"status"`
	NeedsSync    bool          `json:"needsSync"`
	StaleSecrets []StaleSecret `json:"staleSecrets"`
}

// StaleSecret describes a secret that has been updated and needs re-sync.
type StaleSecret struct {
	Name           string `json:"name"`
	SecretType     string `json:"secretType"`
	CurrentVersion int    `json:"currentVersion"`
	SyncedVersion  int    `json:"syncedVersion"`
}

// SyncResponse is the data payload from the sync endpoint.
type SyncResponse struct {
	Secrets []BootSecret `json:"secrets"`
}

type apiResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error,omitempty"`
}

// New creates a cloud client configured from the agent config.
// It supports mTLS (device cert/key) or Bearer token authentication.
func New(cfg *config.Config, logger *zap.Logger) (*Client, error) {
	c := &Client{
		baseURL: cfg.Cloud.URL,
		token:   cfg.Cloud.Token,
		logger:  logger,
	}

	tlsConfig := &tls.Config{}
	customTLS := false

	// Configure mTLS if device cert is provided.
	if cfg.Cloud.DeviceCert != "" && cfg.Cloud.DeviceKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.Cloud.DeviceCert, cfg.Cloud.DeviceKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load device certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		customTLS = true

		// Extract fingerprint from the device certificate.
		if len(cert.Certificate) > 0 {
			parsed, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				return nil, fmt.Errorf("failed to parse device certificate: %w", err)
			}
			hash := sha256.Sum256(parsed.Raw)
			c.fingerprint = hex.EncodeToString(hash[:])
		}
	}

	// Add custom CA if provided.
	if cfg.Cloud.CACert != "" {
		caCert, err := os.ReadFile(cfg.Cloud.CACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
		tlsConfig.RootCAs = pool
		customTLS = true
	}

	transport := &http.Transport{}
	if customTLS {
		transport.TLSClientConfig = tlsConfig
	}

	c.httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return c, nil
}

// Boot calls the boot endpoint and returns the secrets payload.
func (c *Client) Boot(agentVersion, hostname string, detection enclave.Detection, attestation *enclave.Attestation) (*BootResponse, error) {
	body := map[string]interface{}{
		"deviceCertFingerprint": c.fingerprint,
		"agentVersion":          agentVersion,
		"hostname":              hostname,
		"securityLevel":         string(detection.Level),
	}
	if attestation != nil {
		body["attestation"] = attestation
	}

	var resp apiResponse[BootResponse]
	if err := c.post("/boot", body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("boot API error: %s", resp.Error)
	}
	return &resp.Data, nil
}

// Heartbeat sends a heartbeat to the cloud API.
func (c *Client) Heartbeat(agentVersion string) (*HeartbeatResponse, error) {
	body := map[string]string{
		"deviceCertFingerprint": c.fingerprint,
		"agentVersion":          agentVersion,
	}

	var resp apiResponse[HeartbeatResponse]
	if err := c.post("/heartbeat", body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("heartbeat API error: %s", resp.Error)
	}
	return &resp.Data, nil
}

// Sync fetches updated secrets from the cloud API.
func (c *Client) Sync(secretNames []string) (*SyncResponse, error) {
	body := map[string]interface{}{
		"deviceCertFingerprint": c.fingerprint,
		"secretNames":           secretNames,
	}

	var resp apiResponse[SyncResponse]
	if err := c.post("/sync", body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("sync API error: %s", resp.Error)
	}
	return &resp.Data, nil
}

func (c *Client) post(path string, body interface{}, result interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use Bearer token if no mTLS fingerprint.
	if c.token != "" && c.fingerprint == "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}
