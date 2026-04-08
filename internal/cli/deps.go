package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// ── secrets:deps — list dependencies ──────────────────

var secretsDepsCmd = &cobra.Command{
	Use:   "secrets:deps [name]",
	Short: "List dependencies for a secret",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsDeps,
}

func runSecretsDeps(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	secret, err := findSecretByName(oid, args[0])
	if err != nil {
		return err
	}

	resp, err := apiRequest("GET", "/orgs/"+oid+"/secrets/"+secret.ID+"/dependencies", nil)
	if err != nil {
		return err
	}

	var deps struct {
		Upstream   []depItem `json:"upstream"`
		Downstream []depItem `json:"downstream"`
	}
	if err := json.Unmarshal(resp.Data, &deps); err != nil {
		return fmt.Errorf("failed to parse dependencies: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(deps, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("Dependencies for %s\n\n", bold(secret.Name))

	if len(deps.Upstream) > 0 {
		fmt.Println(bold("Upstream (depends on):"))
		headers := []string{"ID", "SOURCE", "TYPE", "CASCADE"}
		rows := make([][]string, len(deps.Upstream))
		for i, d := range deps.Upstream {
			rows[i] = []string{d.ID, d.SourceSecretID, d.Type, fmt.Sprintf("%v", d.AutoCascade)}
		}
		printTable(headers, rows)
		fmt.Println()
	}

	if len(deps.Downstream) > 0 {
		fmt.Println(bold("Downstream (dependents):"))
		headers := []string{"ID", "TARGET", "TYPE", "CASCADE"}
		rows := make([][]string, len(deps.Downstream))
		for i, d := range deps.Downstream {
			rows[i] = []string{d.ID, d.TargetSecretID, d.Type, fmt.Sprintf("%v", d.AutoCascade)}
		}
		printTable(headers, rows)
		fmt.Println()
	}

	if len(deps.Upstream) == 0 && len(deps.Downstream) == 0 {
		fmt.Println("No dependencies found")
	}

	return nil
}

// ── secrets:deps:add — add dependency ─────────────────

var secretsDepsAddCmd = &cobra.Command{
	Use:   "secrets:deps:add [name]",
	Short: "Add a dependency to a secret",
	Long: `Add a dependency from one secret to another.

Usage:
  fyvault secrets:deps:add DB_URL --depends-on=DB_PASSWORD --type=rotates_with --cascade`,
	Args: cobra.ExactArgs(1),
	RunE: runSecretsDepsAdd,
}

var (
	depsDependsOn string
	depsType      string
	depsCascade   bool
)

func init() {
	secretsDepsAddCmd.Flags().StringVar(&depsDependsOn, "depends-on", "", "Secret name this depends on (required)")
	secretsDepsAddCmd.Flags().StringVar(&depsType, "type", "rotates_with", "Dependency type")
	secretsDepsAddCmd.Flags().BoolVar(&depsCascade, "cascade", false, "Auto-cascade rotations")
	_ = secretsDepsAddCmd.MarkFlagRequired("depends-on")
}

func runSecretsDepsAdd(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	// Find source secret (the one that others depend ON)
	source, err := findSecretByName(oid, depsDependsOn)
	if err != nil {
		return fmt.Errorf("source secret: %w", err)
	}

	// Find target secret (the one THAT depends)
	target, err := findSecretByName(oid, args[0])
	if err != nil {
		return fmt.Errorf("target secret: %w", err)
	}

	body := map[string]interface{}{
		"targetSecretId": target.ID,
		"type":           depsType,
		"autoCascade":    depsCascade,
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/secrets/"+source.ID+"/dependencies", body)
	if err != nil {
		return fmt.Errorf("failed to add dependency: %w", err)
	}

	var dep depItem
	if err := json.Unmarshal(resp.Data, &dep); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("%s Dependency added: %s → %s (%s)\n",
		green("OK"), bold(depsDependsOn), bold(args[0]), depsType)
	if depsCascade {
		fmt.Printf("  %s Rotation cascade is enabled\n", yellow("NOTE"))
	}

	return nil
}

// ── secrets:deps:remove — remove dependency ───────────

var secretsDepsRemoveCmd = &cobra.Command{
	Use:   "secrets:deps:remove [dep-id]",
	Short: "Remove a secret dependency",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsDepsRemove,
}

func runSecretsDepsRemove(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	// We need a secret_id for the URL. Use a dummy — the dep_id is the key.
	// The route is /secrets/:secret_id/dependencies/:dep_id but the service ignores secret_id.
	_, err = apiRequest("DELETE", "/orgs/"+oid+"/secrets/_/dependencies/"+args[0], nil)
	if err != nil {
		return fmt.Errorf("failed to remove dependency: %w", err)
	}

	printSuccess(fmt.Sprintf("Dependency %s removed", bold(args[0])))
	return nil
}

// ── Shared types ──────────────────────────────────────

type depItem struct {
	ID             string `json:"id"`
	SourceSecretID string `json:"sourceSecretId"`
	TargetSecretID string `json:"targetSecretId"`
	Type           string `json:"type"`
	AutoCascade    bool   `json:"autoCascade"`
}
