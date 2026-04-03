package cli

import (
	"github.com/spf13/cobra"
)

var (
	apiURL  string
	orgID   string
	format  string // "table" or "json"
	envName string // environment name (e.g. "development", "staging", "production")
)

var rootCmd = &cobra.Command{
	Use:   "fyvault",
	Short: "FyVault -- Secrets that never leave the kernel",
	Long: `FyVault CLI for managing secrets, devices, and organizations.

Commands:
  login / logout     Authenticate with FyVault
  whoami             Show the current user
  orgs               List organizations
  orgs:create        Create an organization
  use                Set the active organization
  secrets            List secrets
  secrets:create     Create a secret (interactive)
  secrets:get        Get secret metadata
  secrets:set        Update a secret value
  secrets:delete     Delete a secret
  secrets:versions   List secret versions
  devices            List devices
  devices:register   Register a new device
  devices:assign     Assign a secret to a device
  run                Run a command with secrets as env vars
  doctor             Check system requirements
  agent:status       Check agent status
  agent:logs         View agent logs
  agent:restart      Restart the agent`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "FyVault API URL (overrides credentials and env)")
	rootCmd.PersistentFlags().StringVar(&orgID, "org", "", "Organization ID (overrides credentials and env)")
	rootCmd.PersistentFlags().StringVar(&format, "format", "table", "Output format: table, json")
	rootCmd.PersistentFlags().StringVar(&envName, "env", "", "Environment name (e.g. development, staging, production)")

	// Auth
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)

	// Orgs
	rootCmd.AddCommand(orgsCmd)
	rootCmd.AddCommand(orgsCreateCmd)
	rootCmd.AddCommand(useCmd)

	// Secrets
	rootCmd.AddCommand(secretsCmd)
	rootCmd.AddCommand(secretsCreateCmd)
	rootCmd.AddCommand(secretsGetCmd)
	rootCmd.AddCommand(secretsSetCmd)
	rootCmd.AddCommand(secretsDeleteCmd)
	rootCmd.AddCommand(secretsVersionsCmd)

	// Environments
	rootCmd.AddCommand(envsCmd)
	rootCmd.AddCommand(envsCreateCmd)
	rootCmd.AddCommand(envsPullCmd)

	// Devices
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(devicesRegisterCmd)
	rootCmd.AddCommand(devicesAssignCmd)

	// Run
	rootCmd.AddCommand(runCmd)

	// System
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(agentStatusCmd)
	rootCmd.AddCommand(agentLogsCmd)
	rootCmd.AddCommand(agentRestartCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
