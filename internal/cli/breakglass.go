package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var breakGlassCmd = &cobra.Command{
	Use:   "break-glass",
	Short: "Create a break-glass emergency access session",
	Long: `Creates a short-lived break-glass token for emergency access.

Requires OWNER role. The token grants full access and auto-revokes
after the specified TTL.

Usage:
  fyvault break-glass --reason="incident-001"
  fyvault break-glass --env=production --reason="incident-001" --ttl=60`,
	RunE: runBreakGlass,
}

var (
	breakGlassReason string
	breakGlassTTL    int
)

func init() {
	breakGlassCmd.Flags().StringVar(&breakGlassReason, "reason", "", "Reason for break-glass access (required)")
	breakGlassCmd.Flags().IntVar(&breakGlassTTL, "ttl", 60, "TTL in minutes (1-240)")
	_ = breakGlassCmd.MarkFlagRequired("reason")
}

func runBreakGlass(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	body := map[string]interface{}{
		"reason":     breakGlassReason,
		"ttlMinutes": breakGlassTTL,
	}
	if envName != "" {
		body["environmentId"] = envName
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/break-glass", body)
	if err != nil {
		return fmt.Errorf("failed to create break-glass session: %w", err)
	}

	var result struct {
		Token        string `json:"token"`
		SessionID    string `json:"sessionId"`
		AutoRevokeAt string `json:"autoRevokeAt"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("%s Break-glass session created\n\n", green("OK"))
	fmt.Printf("  Session ID:  %s\n", result.SessionID)
	fmt.Printf("  Token:       %s\n", bold(result.Token))
	fmt.Printf("  Auto-revoke: %s\n", result.AutoRevokeAt)
	fmt.Printf("  Reason:      %s\n\n", breakGlassReason)
	fmt.Printf("  %s This token grants full OWNER access. Store it securely.\n",
		yellow("WARNING"))

	return nil
}
