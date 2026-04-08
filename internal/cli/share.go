package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var secretsShareCmd = &cobra.Command{
	Use:   "secrets:share [name]",
	Short: "Create a one-time share link for a secret",
	Long: `Creates a one-time, time-limited share link for a secret.

The link can be viewed exactly once and expires after the specified TTL.

Usage:
  fyvault secrets:share DATABASE_URL
  fyvault secrets:share API_KEY --ttl=1h
  fyvault secrets:share STRIPE_KEY --ttl=30m --env=production`,
	Args: cobra.ExactArgs(1),
	RunE: runSecretsShare,
}

var shareTTL string

func init() {
	secretsShareCmd.Flags().StringVar(&shareTTL, "ttl", "24h", "Time-to-live for the share link (e.g. 1h, 30m, 7d)")
}

func runSecretsShare(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	// Find the secret by name
	secret, err := findSecretByName(oid, args[0])
	if err != nil {
		return err
	}

	// Parse TTL
	ttlSeconds, err := parseTTL(shareTTL)
	if err != nil {
		return fmt.Errorf("invalid TTL: %w", err)
	}

	body := map[string]interface{}{
		"ttlSeconds": ttlSeconds,
	}
	if envName != "" {
		body["environmentId"] = envName
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/secrets/"+secret.ID+"/share", body)
	if err != nil {
		return fmt.Errorf("failed to create share link: %w", err)
	}

	var result struct {
		ShareURL   string `json:"share_url"`
		ShareToken string `json:"share_token"`
		ExpiresAt  string `json:"expires_at"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	baseURL := getAPIURL(creds)
	fullURL := baseURL + result.ShareURL

	fmt.Printf("%s Share link created for %s\n\n", green("OK"), bold(secret.Name))
	fmt.Printf("  URL:     %s\n", bold(fullURL))
	fmt.Printf("  Token:   %s\n", dim(result.ShareToken))
	fmt.Printf("  Expires: %s\n\n", result.ExpiresAt)
	fmt.Printf("  %s This link can be viewed %s and will expire after the TTL.\n",
		yellow("NOTE"), bold("exactly once"))

	return nil
}

// parseTTL parses duration strings like "24h", "30m", "7d" into seconds.
func parseTTL(s string) (int, error) {
	// Handle day suffix specially since Go's time.ParseDuration doesn't support "d"
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &days); err == nil && days > 0 {
			return days * 86400, nil
		}
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return int(d.Seconds()), nil
}
