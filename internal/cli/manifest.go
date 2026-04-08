package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ── Manifest types ────────────────────────────────────

// Manifest represents the .env.fyvault YAML file.
type Manifest struct {
	Version  int              `yaml:"version"`
	Org      string           `yaml:"org"`
	Defaults ManifestDefaults `yaml:"defaults"`
	Secrets  []ManifestSecret `yaml:"secrets"`
}

// ManifestDefaults holds default settings for the manifest.
type ManifestDefaults struct {
	Environment string `yaml:"environment"`
}

// ManifestSecret represents a single secret entry in the manifest.
type ManifestSecret struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default,omitempty"`
}

// ── pull ──────────────────────────────────────────────

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch secrets listed in .env.fyvault and output as KEY=VALUE",
	Long: `Reads the .env.fyvault manifest file, fetches the listed secrets
from the FyVault API, and outputs them as KEY=VALUE pairs.

Usage:
  fyvault pull                     # output to stdout
  fyvault pull --export            # write to .env file
  fyvault pull --env=production    # override environment
  eval $(fyvault pull)             # inject into current shell`,
	RunE: runPull,
}

var pullExport bool

func init() {
	pullCmd.Flags().BoolVar(&pullExport, "export", false, "Write to .env file instead of stdout")
}

func runPull(cmd *cobra.Command, args []string) error {
	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	creds, err := loadCredentials()
	if err != nil {
		return err
	}

	// Use manifest org if not overridden
	oid := getOrgID(creds)
	if oid == "" && manifest.Org != "" {
		oid = manifest.Org
	}
	if oid == "" {
		return fmt.Errorf("no organization selected. Set 'org' in .env.fyvault or run 'fyvault use <org-id>'")
	}

	// Determine environment
	targetEnv := envName
	if targetEnv == "" {
		targetEnv = manifest.Defaults.Environment
	}
	if targetEnv == "" {
		targetEnv = "development"
	}

	var lines []string
	fetched := 0
	missing := 0

	for _, s := range manifest.Secrets {
		// Fetch secret value by name
		valResp, valErr := apiRequest("GET",
			"/orgs/"+oid+"/secrets/by-name/"+s.Name+"/value?environment="+targetEnv, nil)

		if valErr != nil {
			if s.Default != "" {
				lines = append(lines, s.Name+"="+s.Default)
				fetched++
				continue
			}
			if s.Required {
				fmt.Fprintf(os.Stderr, "%s Required secret %s not found\n", red("ERROR"), bold(s.Name))
				missing++
			} else {
				fmt.Fprintf(os.Stderr, "%s Optional secret %s not found, skipping\n", dim("INFO"), s.Name)
			}
			continue
		}

		var valData struct {
			Value string `json:"value"`
		}
		if jsonErr := json.Unmarshal(valResp.Data, &valData); jsonErr == nil && valData.Value != "" {
			lines = append(lines, s.Name+"="+valData.Value)
			fetched++
		} else if s.Default != "" {
			lines = append(lines, s.Name+"="+s.Default)
			fetched++
		} else if s.Required {
			fmt.Fprintf(os.Stderr, "%s Required secret %s has no value\n", red("ERROR"), bold(s.Name))
			missing++
		}
	}

	if missing > 0 {
		return fmt.Errorf("%d required secret(s) missing", missing)
	}

	output := strings.Join(lines, "\n") + "\n"

	if pullExport {
		if writeErr := os.WriteFile(".env", []byte(output), 0600); writeErr != nil {
			return fmt.Errorf("failed to write .env: %w", writeErr)
		}
		fmt.Fprintf(os.Stderr, "%s Wrote %d secret(s) to .env (environment: %s)\n",
			green("OK"), fetched, targetEnv)
	} else {
		fmt.Print(output)
		fmt.Fprintf(os.Stderr, "%s Pulled %d secret(s) from environment '%s'\n",
			green("OK"), fetched, targetEnv)
	}

	return nil
}

// ── manifest validate ─────────────────────────────────

var manifestValidateCmd = &cobra.Command{
	Use:   "manifest:validate",
	Short: "Validate that all required secrets in .env.fyvault exist",
	Long: `Reads the .env.fyvault manifest file and checks that every required
secret exists in the specified environment.

Usage:
  fyvault manifest:validate
  fyvault manifest:validate --env=production`,
	RunE: runManifestValidate,
}

func runManifestValidate(cmd *cobra.Command, args []string) error {
	manifest, err := loadManifest()
	if err != nil {
		return err
	}

	creds, err := loadCredentials()
	if err != nil {
		return err
	}

	oid := getOrgID(creds)
	if oid == "" && manifest.Org != "" {
		oid = manifest.Org
	}
	if oid == "" {
		return fmt.Errorf("no organization selected. Set 'org' in .env.fyvault or run 'fyvault use <org-id>'")
	}

	targetEnv := envName
	if targetEnv == "" {
		targetEnv = manifest.Defaults.Environment
	}
	if targetEnv == "" {
		targetEnv = "development"
	}

	fmt.Printf("Validating manifest against environment: %s\n\n", bold(targetEnv))

	// Fetch secrets list for this environment
	resp, err := apiRequest("GET", "/orgs/"+oid+"/secrets?environment="+targetEnv, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}

	var secrets []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Data, &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	// Build set of existing secret names
	existing := make(map[string]bool)
	for _, s := range secrets {
		existing[s.Name] = true
	}

	passed := 0
	failed := 0

	for _, s := range manifest.Secrets {
		if existing[s.Name] {
			printCheck(true, fmt.Sprintf("%s — found", s.Name))
			passed++
		} else if s.Default != "" {
			printCheck(true, fmt.Sprintf("%s — using default", s.Name))
			passed++
		} else if s.Required {
			printCheck(false, fmt.Sprintf("%s — MISSING (required)", s.Name))
			failed++
		} else {
			fmt.Printf("  %s %s — not found (optional)\n", dim("[SKIP]"), s.Name)
		}
	}

	fmt.Printf("\n%d passed, %d failed out of %d secrets\n", passed, failed, len(manifest.Secrets))

	if failed > 0 {
		return fmt.Errorf("validation failed: %d required secret(s) missing", failed)
	}

	fmt.Printf("\n%s All required secrets are present\n", green("OK"))
	return nil
}

// ── helpers ───────────────────────────────────────────

// loadManifest reads and parses the .env.fyvault file from the current directory.
func loadManifest() (*Manifest, error) {
	data, err := os.ReadFile(".env.fyvault")
	if err != nil {
		return nil, fmt.Errorf("no .env.fyvault file found in current directory. Create one to define your secrets manifest")
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse .env.fyvault: %w", err)
	}

	if manifest.Version == 0 {
		manifest.Version = 1
	}

	return &manifest, nil
}

// manifestExists checks if .env.fyvault exists in the current directory.
func manifestExists() bool {
	_, err := os.Stat(".env.fyvault")
	return err == nil
}

// loadManifestEnvironment returns the default environment from the manifest, or empty string.
func loadManifestEnvironment() string {
	manifest, err := loadManifest()
	if err != nil {
		return ""
	}
	return manifest.Defaults.Environment
}

// loadManifestSecretNames returns the list of secret names from the manifest.
func loadManifestSecretNames() []string {
	manifest, err := loadManifest()
	if err != nil {
		return nil
	}
	names := make([]string, len(manifest.Secrets))
	for i, s := range manifest.Secrets {
		names[i] = s.Name
	}
	return names
}
