package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var orgsCmd = &cobra.Command{
	Use:   "orgs",
	Short: "List your organizations",
	RunE:  runOrgsList,
}

var orgsCreateCmd = &cobra.Command{
	Use:   "orgs:create [name]",
	Short: "Create a new organization",
	Args:  cobra.ExactArgs(1),
	RunE:  runOrgsCreate,
}

var useCmd = &cobra.Command{
	Use:   "use [org-id]",
	Short: "Set the current organization",
	Long:  "Switch the active organization for all subsequent commands.",
	Args:  cobra.ExactArgs(1),
	RunE:  runUse,
}

func runOrgsList(_ *cobra.Command, _ []string) error {
	resp, err := apiRequest("GET", "/auth/me/orgs", nil)
	if err != nil {
		return err
	}

	var orgs []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Role        string `json:"role"`
		SecretCount int    `json:"secretCount"`
		DeviceCount int    `json:"deviceCount"`
	}
	if err := json.Unmarshal(resp.Data, &orgs); err != nil {
		return fmt.Errorf("failed to parse orgs: %w", err)
	}

	creds, _ := loadCredentials()
	currentOrg := getOrgID(creds)

	if format == "json" {
		out, _ := json.MarshalIndent(orgs, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	headers := []string{"NAME", "ID", "ROLE", "CURRENT"}
	rows := make([][]string, len(orgs))
	for i, o := range orgs {
		current := ""
		if o.ID == currentOrg {
			current = green("*")
		}
		rows[i] = []string{o.Name, o.ID, o.Role, current}
	}

	printTable(headers, rows)
	return nil
}

func runOrgsCreate(_ *cobra.Command, args []string) error {
	name := args[0]

	resp, err := apiRequest("POST", "/orgs", map[string]string{"name": name})
	if err != nil {
		return err
	}

	var org struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Data, &org); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Auto-set as current org
	creds, loadErr := loadCredentials()
	if loadErr == nil {
		creds.OrgID = org.ID
		_ = saveCredentials(creds)
	}

	printSuccess(fmt.Sprintf("Organization '%s' created (ID: %s)", bold(org.Name), org.ID))
	fmt.Println("  Set as current organization.")
	return nil
}

func runUse(_ *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}

	creds.OrgID = args[0]
	if err := saveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	printSuccess(fmt.Sprintf("Now using organization: %s", bold(args[0])))
	return nil
}
