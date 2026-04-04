package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// ── Import ─────────────────────────────────────────────

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import secrets from a file into an environment",
	Long: `Import secrets from a .env or JSON file into the specified environment.

Usage:
  fyvault import --env=dev --file=.env
  fyvault import --env=staging --format=json --file=secrets.json
  fyvault import --env=dev --file=.env --duplicates=overwrite`,
	RunE: runImport,
}

var (
	importFile       string
	importFormat     string
	importDuplicates string
)

func init() {
	importCmd.Flags().StringVar(&importFile, "file", "", "Path to file to import (required)")
	importCmd.Flags().StringVar(&importFormat, "format", "env", "File format: env, json")
	importCmd.Flags().StringVar(&importDuplicates, "duplicates", "skip", "Duplicate strategy: skip, overwrite, error")
	_ = importCmd.MarkFlagRequired("file")
}

func runImport(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	if envName == "" {
		return fmt.Errorf("--env flag is required for import. Example: fyvault import --env=dev --file=.env")
	}

	// Read file
	content, err := os.ReadFile(importFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Find environment ID
	envResp, err := apiRequest("GET", "/orgs/"+oid+"/environments", nil)
	if err != nil {
		return fmt.Errorf("failed to fetch environments: %w", err)
	}

	var envs []struct {
		EnvironmentID string `json:"environment_id"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(envResp.Data, &envs); err != nil {
		return fmt.Errorf("failed to parse environments: %w", err)
	}

	var targetEnvID string
	for _, e := range envs {
		if e.Name == envName {
			targetEnvID = e.EnvironmentID
			break
		}
	}
	if targetEnvID == "" {
		return fmt.Errorf("environment '%s' not found", envName)
	}

	body := map[string]string{
		"format":            importFormat,
		"content":           string(content),
		"duplicateStrategy": importDuplicates,
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/environments/"+targetEnvID+"/import", body)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	var result struct {
		Created     int `json:"created"`
		Skipped     int `json:"skipped"`
		Overwritten int `json:"overwritten"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("%s Import complete: %d created, %d skipped, %d overwritten\n",
		green("OK"), result.Created, result.Skipped, result.Overwritten)
	return nil
}

// ── Export ──────────────────────────────────────────────

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export secrets from an environment",
	Long: `Export all secrets from the specified environment.

Usage:
  fyvault export --env=production --format=env
  fyvault export --env=staging --format=json > secrets.json
  fyvault export --env=dev --format=yaml`,
	RunE: runExport,
}

var exportFormat string

func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "env", "Output format: env, json, yaml")
}

func runExport(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	if envName == "" {
		return fmt.Errorf("--env flag is required for export. Example: fyvault export --env=production --format=env")
	}

	// Find environment ID
	envResp, err := apiRequest("GET", "/orgs/"+oid+"/environments", nil)
	if err != nil {
		return fmt.Errorf("failed to fetch environments: %w", err)
	}

	var envs []struct {
		EnvironmentID string `json:"environment_id"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(envResp.Data, &envs); err != nil {
		return fmt.Errorf("failed to parse environments: %w", err)
	}

	var targetEnvID string
	for _, e := range envs {
		if e.Name == envName {
			targetEnvID = e.EnvironmentID
			break
		}
	}
	if targetEnvID == "" {
		return fmt.Errorf("environment '%s' not found", envName)
	}

	resp, err := apiRequest("GET", "/orgs/"+oid+"/environments/"+targetEnvID+"/export?format="+exportFormat, nil)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	var result struct {
		Content string `json:"content"`
		Format  string `json:"format"`
		Count   int    `json:"count"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Output to stdout (can be piped to file)
	fmt.Print(result.Content)
	fmt.Fprintf(os.Stderr, "\n%s Exported %d secrets from '%s' as %s\n",
		green("OK"), result.Count, envName, result.Format)
	return nil
}
