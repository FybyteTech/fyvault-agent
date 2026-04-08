package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run -- [command] [args...]",
	Short: "Run a command with secrets injected as environment variables",
	Long: `Fetches secrets from FyVault and injects them as environment variables,
then executes the given command.

Usage:
  fyvault run -- node server.js
  fyvault run -- python manage.py runserver
  fyvault run -- env  (to see injected variables)

Secrets are injected based on their injection config 'envVar' field.
If no envVar is configured, the secret name is used as the variable name.`,
	RunE:              runRun,
	TraverseChildren:  true,
	Args:              cobra.ArbitraryArgs,
}

func runRun(cmd *cobra.Command, args []string) error {
	// Cobra passes everything after "--" via cmd.ArgsLenAtDash()
	dashAt := cmd.ArgsLenAtDash()
	var cmdArgs []string
	if dashAt >= 0 {
		cmdArgs = args[dashAt:]
	} else {
		// No "--" found; treat all args as the command
		cmdArgs = args
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command specified. Usage: fyvault run -- <command> [args...]")
	}

	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	// If .env.fyvault exists and no --env flag, use manifest for environment and secret list
	effectiveEnv := envName
	var manifestSecretNames []string

	if effectiveEnv == "" && manifestExists() {
		mEnv := loadManifestEnvironment()
		if mEnv != "" {
			effectiveEnv = mEnv
			fmt.Fprintf(os.Stderr, "%s Using environment '%s' from .env.fyvault manifest\n", dim("INFO"), effectiveEnv)
		}
		manifestSecretNames = loadManifestSecretNames()
	}

	// Fetch secrets list (scoped to environment if --env is set)
	secretsURL := "/orgs/" + oid + "/secrets"
	if effectiveEnv != "" {
		secretsURL += "?environment=" + effectiveEnv
	}
	resp, err := apiRequest("GET", secretsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}

	var secrets []struct {
		ID              string          `json:"id"`
		Name            string          `json:"name"`
		Value           string          `json:"value"`
		InjectionConfig json.RawMessage `json:"injectionConfig"`
	}
	if err := json.Unmarshal(resp.Data, &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	// Build a set of manifest secret names for filtering (if manifest is in use)
	manifestFilter := make(map[string]bool)
	if len(manifestSecretNames) > 0 {
		for _, n := range manifestSecretNames {
			manifestFilter[strings.ToUpper(n)] = true
		}
	}

	// Build environment: inherit current env + inject secrets
	env := os.Environ()
	injected := 0

	for _, s := range secrets {
		// If manifest is in use, only inject secrets listed in the manifest
		if len(manifestFilter) > 0 && !manifestFilter[strings.ToUpper(s.Name)] {
			continue
		}
		if s.Value == "" {
			// If values aren't returned in list, try fetching individually
			valURL := "/orgs/" + oid + "/secrets/" + s.ID + "/value"
			if effectiveEnv != "" {
				valURL += "?environment=" + effectiveEnv
			}
			valResp, valErr := apiRequest("GET", valURL, nil)
			if valErr != nil {
				fmt.Fprintf(os.Stderr, "%s Skipping %s: value not available via API\n", yellow("WARN"), s.Name)
				continue
			}
			var valData struct {
				Value string `json:"value"`
			}
			if jsonErr := json.Unmarshal(valResp.Data, &valData); jsonErr == nil {
				s.Value = valData.Value
			}
		}

		if s.Value == "" {
			continue
		}

		// Determine env var name from injection config or secret name
		envVarName := s.Name
		if len(s.InjectionConfig) > 0 {
			var config struct {
				EnvVar string `json:"envVar"`
			}
			if jsonErr := json.Unmarshal(s.InjectionConfig, &config); jsonErr == nil && config.EnvVar != "" {
				envVarName = config.EnvVar
			}
		}

		env = append(env, envVarName+"="+s.Value)
		injected++
	}

	if injected > 0 {
		fmt.Fprintf(os.Stderr, "%s Injected %d secret(s) as environment variables\n", green("OK"), injected)
	} else {
		fmt.Fprintf(os.Stderr, "%s No secret values available for injection. Secrets may require the FyVault agent for value access.\n", yellow("WARN"))
	}

	fmt.Fprintf(os.Stderr, "%s Running: %s\n", cyan(">>"), strings.Join(cmdArgs, " "))

	// Execute the command
	binary, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", cmdArgs[0])
	}

	proc := exec.Command(binary, cmdArgs[1:]...)
	proc.Env = env
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	return proc.Run()
}
