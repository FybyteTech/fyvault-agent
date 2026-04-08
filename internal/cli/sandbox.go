package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// ── sandbox create ────────────────────────────────────

var sandboxCreateCmd = &cobra.Command{
	Use:   "sandbox:create",
	Short: "Create an ephemeral sandbox environment",
	Long: `Creates a short-lived sandbox environment by cloning secrets from a parent.

Usage:
  fyvault sandbox:create --from=production --secrets=DB_URL,STRIPE_KEY --ttl=30`,
	RunE: runSandboxCreate,
}

var (
	sandboxFrom    string
	sandboxSecrets string
	sandboxTTL     int
)

func init() {
	sandboxCreateCmd.Flags().StringVar(&sandboxFrom, "from", "", "Parent environment to clone from (required)")
	sandboxCreateCmd.Flags().StringVar(&sandboxSecrets, "secrets", "", "Comma-separated secret names to include (required)")
	sandboxCreateCmd.Flags().IntVar(&sandboxTTL, "ttl", 30, "TTL in minutes")
	_ = sandboxCreateCmd.MarkFlagRequired("from")
	_ = sandboxCreateCmd.MarkFlagRequired("secrets")
}

func runSandboxCreate(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	// Resolve parent environment ID
	parentEnv, err := findEnvironmentByName(oid, sandboxFrom)
	if err != nil {
		return err
	}

	secretNames := strings.Split(sandboxSecrets, ",")
	for i, s := range secretNames {
		secretNames[i] = strings.TrimSpace(s)
	}

	body := map[string]interface{}{
		"parentEnvId": parentEnv.ID,
		"secretNames": secretNames,
		"ttlMinutes":  sandboxTTL,
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/environments/sandbox", body)
	if err != nil {
		return fmt.Errorf("failed to create sandbox: %w", err)
	}

	var result struct {
		EnvironmentID string `json:"environmentId"`
		Name          string `json:"name"`
		AutoDestroyAt string `json:"autoDestroyAt"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("%s Sandbox created from %s\n\n", green("OK"), bold(sandboxFrom))
	fmt.Printf("  Environment: %s\n", bold(result.Name))
	fmt.Printf("  ID:          %s\n", result.EnvironmentID)
	fmt.Printf("  Destroys at: %s\n", result.AutoDestroyAt)
	fmt.Printf("  Secrets:     %s\n", strings.Join(secretNames, ", "))

	return nil
}

// ── sandbox list ──────────────────────────────────────

var sandboxListCmd = &cobra.Command{
	Use:   "sandbox:list",
	Short: "List active ephemeral sandboxes",
	RunE:  runSandboxList,
}

type sandboxItem struct {
	EnvironmentID string `json:"environment_id"`
	Name          string `json:"name"`
	AutoDestroyAt string `json:"auto_destroy_at"`
	CreatedAt     string `json:"created_at"`
}

func runSandboxList(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	resp, err := apiRequest("GET", "/orgs/"+oid+"/environments/sandbox", nil)
	if err != nil {
		return err
	}

	var sandboxes []sandboxItem
	if err := json.Unmarshal(resp.Data, &sandboxes); err != nil {
		return fmt.Errorf("failed to parse sandboxes: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(sandboxes, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	if len(sandboxes) == 0 {
		fmt.Println("No active sandboxes")
		return nil
	}

	headers := []string{"NAME", "ID", "AUTO-DESTROY", "CREATED"}
	rows := make([][]string, len(sandboxes))
	for i, s := range sandboxes {
		rows[i] = []string{s.Name, s.EnvironmentID, s.AutoDestroyAt, s.CreatedAt}
	}

	printTable(headers, rows)
	fmt.Printf("\n%s active sandboxes\n", bold(fmt.Sprintf("%d", len(sandboxes))))
	return nil
}

// ── sandbox destroy ───────────────────────────────────

var sandboxDestroyCmd = &cobra.Command{
	Use:   "sandbox:destroy [env-id]",
	Short: "Destroy an ephemeral sandbox early",
	Args:  cobra.ExactArgs(1),
	RunE:  runSandboxDestroy,
}

func runSandboxDestroy(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	_, err = apiRequest("DELETE", "/orgs/"+oid+"/environments/sandbox/"+args[0], nil)
	if err != nil {
		return fmt.Errorf("failed to destroy sandbox: %w", err)
	}

	printSuccess(fmt.Sprintf("Sandbox %s destroyed", bold(args[0])))
	return nil
}

// ── Helper: find environment by name ──────────────────

type envItem struct {
	ID   string `json:"environment_id"`
	Name string `json:"name"`
}

func findEnvironmentByName(orgID, name string) (*envItem, error) {
	resp, err := apiRequest("GET", "/orgs/"+orgID+"/environments", nil)
	if err != nil {
		return nil, err
	}

	var envs []envItem
	if err := json.Unmarshal(resp.Data, &envs); err != nil {
		return nil, fmt.Errorf("failed to parse environments: %w", err)
	}

	for _, e := range envs {
		if strings.EqualFold(e.Name, name) {
			return &e, nil
		}
	}

	return nil, fmt.Errorf("environment '%s' not found", name)
}
