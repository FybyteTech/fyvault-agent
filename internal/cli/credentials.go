package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Credentials holds authentication state persisted to disk.
type Credentials struct {
	APIUrl       string `json:"apiUrl"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	OrgID        string `json:"orgId"`
	Email        string `json:"email"`
}

func credentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fyvault", "credentials.json")
}

func loadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		return nil, fmt.Errorf("not logged in. Run 'fyvault login' first")
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("corrupt credentials file: %w", err)
	}
	return &creds, nil
}

func saveCredentials(creds *Credentials) error {
	dir := filepath.Dir(credentialsPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(credentialsPath(), data, 0600)
}

func deleteCredentials() error {
	return os.Remove(credentialsPath())
}

// getAPIURL resolves the API URL from flag > env > credentials > default.
func getAPIURL(creds *Credentials) string {
	if apiURL != "" {
		return apiURL
	}
	if v := os.Getenv("FYVAULT_API_URL"); v != "" {
		return v
	}
	if creds != nil && creds.APIUrl != "" {
		return creds.APIUrl
	}
	return "http://localhost:4000/api/v1"
}

// getOrgID resolves the org ID from flag > env > credentials.
func getOrgID(creds *Credentials) string {
	if orgID != "" {
		return orgID
	}
	if v := os.Getenv("FYVAULT_ORG"); v != "" {
		return v
	}
	if creds != nil && creds.OrgID != "" {
		return creds.OrgID
	}
	return ""
}

// apiResponse is the standard JSON envelope from the FyVault API.
type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

// apiRequest makes an authenticated API request and returns the parsed response.
func apiRequest(method, path string, body interface{}) (*apiResponse, error) {
	creds, err := loadCredentials()
	if err != nil {
		return nil, err
	}

	base := getAPIURL(creds)
	url := base + path

	var bodyReader io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("invalid API response (status %d): %w", resp.StatusCode, err)
	}

	if !result.Success {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errMsg)
	}

	return &result, nil
}

// apiRequestUnauth makes an unauthenticated API request (for login).
func apiRequestUnauth(baseURL, method, path string, body interface{}) (*apiResponse, error) {
	url := baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("invalid API response (status %d): %w", resp.StatusCode, err)
	}

	if !result.Success {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errMsg)
	}

	return &result, nil
}

// apiRequestWithToken makes an authenticated request using a specific token (for post-login flows).
func apiRequestWithToken(baseURL, token, method, path string, body interface{}) (*apiResponse, error) {
	url := baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("invalid API response (status %d): %w", resp.StatusCode, err)
	}

	if !result.Success {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API error: %s", errMsg)
	}

	return &result, nil
}
