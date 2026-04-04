package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// ── List environments ──────────────────────────────────

var envsCmd = &cobra.Command{
	Use:   "envs",
	Short: "List environments",
	RunE:  runEnvsList,
}

func runEnvsList(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	resp, err := apiRequest("GET", "/orgs/"+oid+"/environments", nil)
	if err != nil {
		return fmt.Errorf("failed to fetch environments: %w", err)
	}

	var envs []struct {
		EnvironmentID string `json:"environment_id"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		IsDefault     bool   `json:"is_default"`
		Count         struct {
			SecretValues int `json:"secret_values"`
			Devices      int `json:"devices"`
		} `json:"_count"`
	}
	if err := json.Unmarshal(resp.Data, &envs); err != nil {
		return fmt.Errorf("failed to parse environments: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(envs, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDEFAULT\tSECRETS\tDEVICES\tDESCRIPTION")
	for _, e := range envs {
		def := ""
		if e.IsDefault {
			def = "✓"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\n",
			e.Name, def, e.Count.SecretValues, e.Count.Devices, e.Description)
	}
	return w.Flush()
}

// ── Create environment ─────────────────────────────────

var envsCreateCmd = &cobra.Command{
	Use:   "envs:create [name]",
	Short: "Create a new environment",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvsCreate,
}

func runEnvsCreate(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	body := map[string]string{"name": args[0]}
	resp, err := apiRequest("POST", "/orgs/"+oid+"/environments", body)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	var env struct {
		EnvironmentID string `json:"environment_id"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(resp.Data, &env); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("%s Created environment: %s (%s)\n", green("OK"), env.Name, env.EnvironmentID)
	return nil
}

// ── Pull environment secrets ───────────────────────────

var envsPullCmd = &cobra.Command{
	Use:   "envs:pull [env-name]",
	Short: "Pull all secret values for an environment (outputs KEY=VALUE)",
	Long: `Fetches all secrets for the given environment and outputs them as KEY=VALUE pairs.

Usage:
  fyvault envs:pull development
  fyvault envs:pull staging > .env.staging
  eval $(fyvault envs:pull production)`,
	Args: cobra.ExactArgs(1),
	RunE: runEnvsPull,
}

func runEnvsPull(cmd *cobra.Command, args []string) error {
	targetEnv := args[0]

	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	// Fetch secrets list for this environment
	resp, err := apiRequest("GET", "/orgs/"+oid+"/secrets?environment="+targetEnv, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}

	var secrets []struct {
		SecretID string `json:"secret_id"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(resp.Data, &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	pulled := 0
	for _, s := range secrets {
		valResp, valErr := apiRequest("GET", "/orgs/"+oid+"/secrets/"+s.SecretID+"/value?environment="+targetEnv, nil)
		if valErr != nil {
			fmt.Fprintf(os.Stderr, "%s Skipping %s: %v\n", yellow("WARN"), s.Name, valErr)
			continue
		}
		var valData struct {
			Value string `json:"value"`
		}
		if jsonErr := json.Unmarshal(valResp.Data, &valData); jsonErr == nil && valData.Value != "" {
			fmt.Printf("%s=%s\n", s.Name, valData.Value)
			pulled++
		}
	}

	fmt.Fprintf(os.Stderr, "%s Pulled %d secret(s) from environment '%s'\n", green("OK"), pulled, targetEnv)
	return nil
}
