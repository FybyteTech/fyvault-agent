package boot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/config"
)

// TokenFileData is the JSON structure written to the token file.
// The SDK reads this to authenticate without needing FYVAULT_API_KEY in .env.
type TokenFileData struct {
	Token       string `json:"token"`
	OrgID       string `json:"org_id"`
	Environment string `json:"environment,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	AgentPID    int    `json:"agent_pid"`
}

// tokenFilePaths returns the paths where the token file should be written.
// Multiple paths for different SDK discovery scenarios.
func tokenFilePaths() []string {
	paths := []string{}

	switch runtime.GOOS {
	case "linux":
		paths = append(paths,
			"/var/run/fyvault/token",
			"/tmp/fyvault-token",
		)
	case "darwin":
		paths = append(paths,
			"/tmp/fyvault-token",
		)
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			paths = append(paths,
				filepath.Join(appdata, "fyvault", "token"),
			)
		}
		paths = append(paths,
			filepath.Join(os.TempDir(), "fyvault-token"),
		)
	default:
		paths = append(paths,
			"/tmp/fyvault-token",
		)
	}

	// Also write to user's home directory
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".fyvault", "token"))
	}

	return paths
}

// WriteTokenFile writes the agent's credentials to a local file that SDKs
// can discover automatically via FyVault.auto().
//
// This eliminates the need for FYVAULT_API_KEY in .env when the agent is
// running on the same machine.
func WriteTokenFile(cfg *config.Config, logger *zap.Logger) {
	// The token is the device cert fingerprint — the same credential the agent
	// uses to authenticate with the cloud API. SDKs can use this to call the
	// API on behalf of the device.
	//
	// Write the device token from config. The SDK can use this to authenticate
	// with the FyVault API when running on the same machine as the agent.
	token := cfg.Cloud.Token
	if token == "" {
		logger.Debug("no device token configured, skipping token file")
		return
	}

	data := TokenFileData{
		Token:    token,
		AgentPID: os.Getpid(),
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Warn("failed to marshal token file", zap.Error(err))
		return
	}

	paths := tokenFilePaths()
	written := 0

	for _, p := range paths {
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0700); err != nil {
			logger.Debug("cannot create token dir", zap.String("path", dir), zap.Error(err))
			continue
		}

		if err := os.WriteFile(p, jsonBytes, 0600); err != nil {
			logger.Debug("cannot write token file", zap.String("path", p), zap.Error(err))
			continue
		}

		logger.Info("wrote SDK token file", zap.String("path", p))
		written++
	}

	if written == 0 {
		logger.Warn("could not write token file to any location")
	}
}

// RemoveTokenFiles cleans up token files on shutdown.
func RemoveTokenFiles(logger *zap.Logger) {
	for _, p := range tokenFilePaths() {
		if err := os.Remove(p); err == nil {
			logger.Debug("removed token file", zap.String("path", p))
		}
	}
}

// EnsureTokenDir creates /var/run/fyvault with correct permissions (Linux only).
func EnsureTokenDir() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	dir := "/var/run/fyvault"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}
	return nil
}
