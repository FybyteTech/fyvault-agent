package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [platform]",
	Short: "Sync secrets to an external platform",
	Long: `Push FyVault secrets to external hosting and deployment platforms.

Supported platforms:
  vercel     Sync to Vercel project environment variables
  netlify    Sync to Netlify site environment variables
  railway    Sync to Railway service variables
  heroku     Sync to Heroku config vars
  fly        Sync to Fly.io app secrets
  render     Sync to Render service env vars

Usage:
  fyvault sync vercel --env=production --token=$VERCEL_TOKEN --project-id=prj_xxx
  fyvault sync heroku --env=production --token=$HEROKU_API_KEY --app=my-app
  fyvault sync fly --env=staging --token=$FLY_API_TOKEN --app=my-app`,
	Args: cobra.ExactArgs(1),
	RunE: runSync,
}

var (
	syncToken     string
	syncProjectID string
	syncAppName   string
	syncServiceID string
)

func init() {
	syncCmd.Flags().StringVar(&syncToken, "token", "", "Platform API token (required)")
	syncCmd.Flags().StringVar(&syncProjectID, "project-id", "", "Project/service ID (Vercel, Railway)")
	syncCmd.Flags().StringVar(&syncAppName, "app", "", "App name (Heroku, Fly.io)")
	syncCmd.Flags().StringVar(&syncServiceID, "service-id", "", "Service ID (Render)")
	_ = syncCmd.MarkFlagRequired("token")
}

func runSync(cmd *cobra.Command, args []string) error {
	platform := args[0]

	if envName == "" {
		return fmt.Errorf("--env flag is required. Example: fyvault sync %s --env=production --token=...", platform)
	}

	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
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

	// Build config
	config := map[string]string{
		"token": syncToken,
	}
	switch platform {
	case "vercel":
		if syncProjectID == "" {
			return fmt.Errorf("--project-id is required for Vercel")
		}
		config["projectId"] = syncProjectID
	case "railway":
		if syncProjectID == "" {
			return fmt.Errorf("--project-id is required for Railway")
		}
		config["projectId"] = syncProjectID
		config["environmentId"] = targetEnvID
	case "heroku", "fly":
		if syncAppName == "" {
			return fmt.Errorf("--app is required for %s", platform)
		}
		config["appName"] = syncAppName
	case "render":
		if syncServiceID == "" {
			return fmt.Errorf("--service-id is required for Render")
		}
		config["serviceId"] = syncServiceID
	case "netlify":
		if syncServiceID == "" {
			return fmt.Errorf("--service-id (site ID) is required for Netlify")
		}
		config["siteId"] = syncServiceID
	default:
		return fmt.Errorf("unsupported platform: %s. Supported: vercel, netlify, railway, heroku, fly, render", platform)
	}

	body := map[string]interface{}{
		"platform":      platform,
		"environmentId": targetEnvID,
		"config":        config,
	}

	fmt.Printf("%s Syncing secrets from '%s' to %s...\n", cyan(">>"), envName, platform)

	resp, err := apiRequest("POST", "/orgs/"+oid+"/integrations/sync", body)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	var result struct {
		Platform string   `json:"platform"`
		Synced   int      `json:"synced"`
		Failed   int      `json:"failed"`
		Errors   []string `json:"errors"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Failed > 0 {
		fmt.Printf("%s Synced %d, failed %d to %s\n", yellow("WARN"), result.Synced, result.Failed, result.Platform)
		for _, e := range result.Errors {
			fmt.Printf("  %s %s\n", red("✖"), e)
		}
	} else {
		fmt.Printf("%s Synced %d secret(s) to %s\n", green("OK"), result.Synced, result.Platform)
	}

	return nil
}

// ── Generate config ────────────────────────────────────

var generateCmd = &cobra.Command{
	Use:   "generate [format]",
	Short: "Generate config files from secrets",
	Long: `Generate infrastructure config files from FyVault secrets.

Supported formats:
  k8s              Kubernetes Secret manifest
  docker           Docker .env file
  docker-compose   Docker Compose snippet
  terraform        Terraform tfvars
  ansible          Ansible vars file
  pulumi           Pulumi config
  github-actions   GitHub Actions env block
  gitlab-ci        GitLab CI variables
  circleci         CircleCI context

Usage:
  fyvault generate k8s --env=production --name=app-secrets
  fyvault generate terraform --env=staging > terraform.tfvars
  fyvault generate docker --env=dev > .env.docker`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerate,
}

var (
	genName      string
	genNamespace string
	genService   string
)

func init() {
	generateCmd.Flags().StringVar(&genName, "name", "app-secrets", "Resource name (K8s, Pulumi)")
	generateCmd.Flags().StringVar(&genNamespace, "namespace", "default", "K8s namespace")
	generateCmd.Flags().StringVar(&genService, "service", "app", "Service name (Docker Compose)")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	format := args[0]

	if envName == "" {
		return fmt.Errorf("--env flag is required. Example: fyvault generate %s --env=production", format)
	}

	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected")
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

	body := map[string]interface{}{
		"format":        format,
		"environmentId": targetEnvID,
		"options": map[string]string{
			"name":        genName,
			"namespace":   genNamespace,
			"serviceName": genService,
		},
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/integrations/generate", body)
	if err != nil {
		return fmt.Errorf("generate failed: %w", err)
	}

	var result struct {
		Content string `json:"content"`
		Format  string `json:"format"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Print(result.Content)
	return nil
}
