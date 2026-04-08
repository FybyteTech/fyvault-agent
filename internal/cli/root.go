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
  init               Scan .env files and import into FyVault
  secrets:share      Create a one-time share link for a secret
  hook:install       Install git pre-commit hook for leak scanning
  hook:uninstall     Remove the FyVault pre-commit hook
  pull               Fetch secrets from .env.fyvault manifest
  manifest:validate  Check all required secrets exist
  break-glass        Create emergency break-glass access session
  sandbox:create     Create an ephemeral sandbox environment
  sandbox:list       List active sandboxes
  sandbox:destroy    Destroy a sandbox early
  compliance:report  Generate a compliance report
  secrets:deps       List secret dependencies
  secrets:deps:add   Add a secret dependency
  secrets:deps:remove Remove a secret dependency
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
	rootCmd.AddCommand(secretsRotateCmd)

	// Environments
	rootCmd.AddCommand(envsCmd)
	rootCmd.AddCommand(envsCreateCmd)
	rootCmd.AddCommand(envsPullCmd)

	// Devices
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(devicesRegisterCmd)
	rootCmd.AddCommand(devicesAssignCmd)

	// Import/Export
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)

	// Sync & Generate
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(generateCmd)

	// Agent Credentials
	rootCmd.AddCommand(agentCredsCmd)
	rootCmd.AddCommand(agentCredsCreateCmd)
	rootCmd.AddCommand(agentCredsRevokeCmd)

	// Scan
	rootCmd.AddCommand(scanCmd)

	// Run
	rootCmd.AddCommand(runCmd)

	// Init
	rootCmd.AddCommand(initCmd)

	// Share
	rootCmd.AddCommand(secretsShareCmd)

	// Hook
	rootCmd.AddCommand(hookInstallCmd)
	rootCmd.AddCommand(hookUninstallCmd)

	// Manifest
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(manifestValidateCmd)

	// Break-glass
	rootCmd.AddCommand(breakGlassCmd)

	// Sandbox
	rootCmd.AddCommand(sandboxCreateCmd)
	rootCmd.AddCommand(sandboxListCmd)
	rootCmd.AddCommand(sandboxDestroyCmd)

	// Compliance
	rootCmd.AddCommand(complianceReportCmd)

	// Dependencies
	rootCmd.AddCommand(secretsDepsCmd)
	rootCmd.AddCommand(secretsDepsAddCmd)
	rootCmd.AddCommand(secretsDepsRemoveCmd)

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
