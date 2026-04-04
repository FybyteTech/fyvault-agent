package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "List secrets in the current organization",
	RunE:  runSecretsList,
}

var secretsCreateCmd = &cobra.Command{
	Use:   "secrets:create",
	Short: "Create a new secret (interactive)",
	RunE:  runSecretsCreate,
}

var secretsGetCmd = &cobra.Command{
	Use:   "secrets:get [name]",
	Short: "Get secret metadata",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsGet,
}

var secretsSetCmd = &cobra.Command{
	Use:   "secrets:set [name] [value]",
	Short: "Update a secret's value",
	Args:  cobra.ExactArgs(2),
	RunE:  runSecretsSet,
}

var secretsDeleteCmd = &cobra.Command{
	Use:   "secrets:delete [name]",
	Short: "Delete a secret",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsDelete,
}

var secretsVersionsCmd = &cobra.Command{
	Use:   "secrets:versions [name]",
	Short: "List secret versions",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsVersions,
}

type secretItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SecretType      string `json:"secretType"`
	Version         int    `json:"version"`
	DeviceCount     int    `json:"deviceCount"`
	EncryptionMode  string `json:"encryptionMode"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

func runSecretsList(_ *cobra.Command, _ []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	listURL := "/orgs/" + oid + "/secrets"
	if envName != "" {
		listURL += "?environment=" + envName
	}
	resp, err := apiRequest("GET", listURL, nil)
	if err != nil {
		return err
	}

	var secrets []secretItem
	if err := json.Unmarshal(resp.Data, &secrets); err != nil {
		return fmt.Errorf("failed to parse secrets: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(secrets, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	headers := []string{"NAME", "TYPE", "VERSION", "ENCRYPTION", "DEVICES"}
	rows := make([][]string, len(secrets))
	for i, s := range secrets {
		rows[i] = []string{
			s.Name,
			s.SecretType,
			fmt.Sprintf("v%d", s.Version),
			s.EncryptionMode,
			fmt.Sprintf("%d", s.DeviceCount),
		}
	}

	printTable(headers, rows)
	fmt.Printf("\n%s secrets in organization\n", bold(fmt.Sprintf("%d", len(secrets))))
	return nil
}

func runSecretsCreate(_ *cobra.Command, _ []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	fmt.Println(bold("Create a new secret"))
	fmt.Println()

	// 1. Name
	name := promptInput("Secret name (e.g. DATABASE_URL): ")
	if name == "" {
		return fmt.Errorf("secret name is required")
	}
	name = strings.ToUpper(strings.ReplaceAll(name, " ", "_"))

	// 2. Type
	types := []string{"API_KEY", "DB_CREDENTIAL", "AWS_CREDENTIAL", "GENERIC"}
	_, secretType := promptSelect("Secret type:", types)

	// 3. Value
	value := promptPassword("Secret value: ")
	if value == "" {
		return fmt.Errorf("secret value is required")
	}

	// 4. Encryption mode
	modes := []string{"CLOUD", "DEVICE"}
	_, encryptionMode := promptSelect("Encryption mode:", modes)

	// 5. Build injection config based on type
	injectionConfig := buildInjectionConfig(secretType)

	body := map[string]interface{}{
		"name":             name,
		"secretType":       secretType,
		"value":            value,
		"encryptionMode":   encryptionMode,
		"injectionConfig":  injectionConfig,
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/secrets", body)
	if err != nil {
		return err
	}

	var created struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Data, &created); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Secret '%s' created (ID: %s)", bold(created.Name), created.ID))
	return nil
}

func buildInjectionConfig(secretType string) map[string]interface{} {
	config := make(map[string]interface{})

	switch secretType {
	case "API_KEY":
		envVar := promptInput("Environment variable name (e.g. API_KEY): ")
		if envVar != "" {
			config["envVar"] = envVar
		}
		headerName := promptInput("HTTP header name (optional, e.g. X-API-Key): ")
		if headerName != "" {
			config["headerName"] = headerName
		}

	case "DB_CREDENTIAL":
		host := promptInput("Database host (optional): ")
		if host != "" {
			config["host"] = host
		}
		port := promptInput("Database port (optional): ")
		if port != "" {
			config["port"] = port
		}
		dbName := promptInput("Database name (optional): ")
		if dbName != "" {
			config["database"] = dbName
		}

	case "AWS_CREDENTIAL":
		region := promptInput("AWS region (optional, e.g. us-east-1): ")
		if region != "" {
			config["region"] = region
		}
		service := promptInput("AWS service (optional, e.g. s3): ")
		if service != "" {
			config["service"] = service
		}

	case "GENERIC":
		envVar := promptInput("Environment variable name (optional): ")
		if envVar != "" {
			config["envVar"] = envVar
		}
	}

	return config
}

func runSecretsGet(_ *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	// First list secrets to find by name
	secret, err := findSecretByName(oid, args[0])
	if err != nil {
		return err
	}

	if format == "json" {
		out, _ := json.MarshalIndent(secret, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("  Name:       %s\n", bold(secret.Name))
	fmt.Printf("  Type:       %s\n", secret.SecretType)
	fmt.Printf("  Version:    v%d\n", secret.Version)
	fmt.Printf("  Encryption: %s\n", secret.EncryptionMode)
	fmt.Printf("  Devices:    %d\n", secret.DeviceCount)
	fmt.Printf("  ID:         %s\n", dim(secret.ID))
	return nil
}

func runSecretsSet(_ *cobra.Command, args []string) error {
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

	body := map[string]string{"value": args[1]}
	_, err = apiRequest("PATCH", "/orgs/"+oid+"/secrets/"+secret.ID, body)
	if err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Secret '%s' updated (now v%d)", bold(secret.Name), secret.Version+1))
	return nil
}

func runSecretsDelete(_ *cobra.Command, args []string) error {
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

	if !promptConfirm(fmt.Sprintf("Delete secret '%s'? This cannot be undone.", secret.Name)) {
		fmt.Println("Aborted.")
		return nil
	}

	_, err = apiRequest("DELETE", "/orgs/"+oid+"/secrets/"+secret.ID, nil)
	if err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Secret '%s' deleted", bold(secret.Name)))
	return nil
}

func runSecretsVersions(_ *cobra.Command, args []string) error {
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

	resp, err := apiRequest("GET", "/orgs/"+oid+"/secrets/"+secret.ID+"/versions", nil)
	if err != nil {
		return err
	}

	var versions []struct {
		Version   int    `json:"version"`
		CreatedAt string `json:"createdAt"`
		CreatedBy string `json:"createdBy"`
	}
	if err := json.Unmarshal(resp.Data, &versions); err != nil {
		return fmt.Errorf("failed to parse versions: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(versions, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	headers := []string{"VERSION", "CREATED AT", "CREATED BY"}
	rows := make([][]string, len(versions))
	for i, v := range versions {
		rows[i] = []string{
			fmt.Sprintf("v%d", v.Version),
			v.CreatedAt,
			v.CreatedBy,
		}
	}

	printTable(headers, rows)
	return nil
}

// findSecretByName fetches the secrets list and finds one by name.
func findSecretByName(orgID, name string) (*secretItem, error) {
	findURL := "/orgs/" + orgID + "/secrets"
	if envName != "" {
		findURL += "?environment=" + envName
	}
	resp, err := apiRequest("GET", findURL, nil)
	if err != nil {
		return nil, err
	}

	var secrets []secretItem
	if err := json.Unmarshal(resp.Data, &secrets); err != nil {
		return nil, fmt.Errorf("failed to parse secrets: %w", err)
	}

	upperName := strings.ToUpper(name)
	for _, s := range secrets {
		if strings.ToUpper(s.Name) == upperName {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("secret '%s' not found", name)
}

// ── Rotate ─────────────────────────────────────────────

var secretsRotateCmd = &cobra.Command{
	Use:   "secrets:rotate [name]",
	Short: "Rotate a secret (generate new value)",
	Args:  cobra.ExactArgs(1),
	RunE:  runSecretsRotate,
}

func runSecretsRotate(cmd *cobra.Command, args []string) error {
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

	rotateURL := "/orgs/" + oid + "/secrets/" + secret.ID + "/rotate"
	if envName != "" {
		rotateURL += "?environment=" + envName
	}

	resp, err := apiRequest("POST", rotateURL, map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to rotate secret: %w", err)
	}

	var result struct {
		SecretID string `json:"secretId"`
		Name     string `json:"name"`
		Version  int    `json:"version"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("%s Rotated %s → version %d\n", green("OK"), result.Name, result.Version)
	return nil
}
