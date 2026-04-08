package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// ── List agent credentials ────────────────────────────────

var agentCredsCmd = &cobra.Command{
	Use:   "agent-creds",
	Short: "List agent credentials",
	RunE:  runAgentCredsList,
}

func runAgentCredsList(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	resp, err := apiRequest("GET", "/orgs/"+oid+"/agent-credentials", nil)
	if err != nil {
		return fmt.Errorf("failed to fetch agent credentials: %w", err)
	}

	var items []struct {
		CredentialID string   `json:"credential_id"`
		Name         string   `json:"name"`
		AgentType    string   `json:"agent_type"`
		Scopes       []string `json:"scopes"`
		LastUsedAt   *string  `json:"last_used_at"`
		CreatedAt    string   `json:"created_at"`
	}
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		return fmt.Errorf("failed to parse agent credentials: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(items, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	headers := []string{"NAME", "TYPE", "SCOPES", "LAST USED", "CREATED"}
	var rows [][]string
	for _, item := range items {
		lastUsed := "Never"
		if item.LastUsedAt != nil {
			lastUsed = *item.LastUsedAt
		}
		rows = append(rows, []string{
			item.Name,
			item.AgentType,
			strings.Join(item.Scopes, ", "),
			lastUsed,
			item.CreatedAt,
		})
	}
	printTable(headers, rows)
	return nil
}

// ── Create agent credential ───────────────────────────────

var agentCredsCreateCmd = &cobra.Command{
	Use:   "agent-creds:create",
	Short: "Create a new agent credential (interactive)",
	RunE:  runAgentCredsCreate,
}

func runAgentCredsCreate(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	name := promptInput("Credential name: ")
	if name == "" {
		return fmt.Errorf("name is required")
	}

	agentTypes := []string{"ai_assistant", "ci_bot", "service", "custom"}
	_, agentType := promptSelect("Agent type:", agentTypes)

	allScopes := []string{"SECRETS_READ", "SECRETS_WRITE", "DEVICES_READ", "DEVICES_WRITE", "AUDIT_READ", "BOOT"}
	fmt.Println("Select scopes (comma-separated numbers):")
	for i, s := range allScopes {
		fmt.Printf("  %s %s\n", cyan(fmt.Sprintf("[%d]", i+1)), s)
	}
	scopeInput := promptInput("Scopes: ")
	var selectedScopes []string
	for _, part := range strings.Split(scopeInput, ",") {
		part = strings.TrimSpace(part)
		var idx int
		if _, scanErr := fmt.Sscanf(part, "%d", &idx); scanErr == nil && idx >= 1 && idx <= len(allScopes) {
			selectedScopes = append(selectedScopes, allScopes[idx-1])
		}
	}
	if len(selectedScopes) == 0 {
		return fmt.Errorf("at least one scope is required")
	}

	body := map[string]interface{}{
		"name":      name,
		"agentType": agentType,
		"scopes":    selectedScopes,
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/agent-credentials", body)
	if err != nil {
		return fmt.Errorf("failed to create agent credential: %w", err)
	}

	var result struct {
		Credential   string `json:"credential"`
		CredentialID string `json:"credential_id"`
		Name         string `json:"name"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Created agent credential: %s (%s)", result.Name, result.CredentialID))
	fmt.Println()
	fmt.Println(bold("Credential (save this now — it will not be shown again):"))
	fmt.Println()
	fmt.Println("  " + result.Credential)
	fmt.Println()

	return nil
}

// ── Revoke agent credential ──────────────────────────────

var agentCredsRevokeCmd = &cobra.Command{
	Use:   "agent-creds:revoke <credential_id>",
	Short: "Revoke an agent credential",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentCredsRevoke,
}

func runAgentCredsRevoke(cmd *cobra.Command, args []string) error {
	credentialID := args[0]

	if !promptConfirm(fmt.Sprintf("Revoke agent credential %s?", credentialID)) {
		fmt.Println("Cancelled.")
		return nil
	}

	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	_, err = apiRequest("DELETE", "/orgs/"+oid+"/agent-credentials/"+credentialID, nil)
	if err != nil {
		return fmt.Errorf("failed to revoke agent credential: %w", err)
	}

	printSuccess("Agent credential revoked")
	return nil
}
