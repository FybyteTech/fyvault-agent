package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scan for .env files and import secrets into FyVault",
	Long: `Scans the current directory for .env files, parses KEY=VALUE pairs,
and imports them into the appropriate FyVault environments.

Usage:
  fyvault init
  fyvault init --org=org_acme`,
	RunE: runInit,
}

// envFileMapping maps .env file suffixes to environment names.
var envFileMapping = map[string]string{
	".env":             "development",
	".env.local":       "development",
	".env.development": "development",
	".env.production":  "production",
	".env.staging":     "staging",
	".env.test":        "test",
}

// envFileOrder controls the display order when scanning.
var envFileOrder = []string{
	".env",
	".env.local",
	".env.development",
	".env.production",
	".env.staging",
	".env.test",
}

type envFileInfo struct {
	Path        string
	Filename    string
	Environment string
	SecretCount int
	Content     string
}

func runInit(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fmt.Println(bold("FyVault Init"))
	fmt.Printf("Scanning %s for .env files...\n\n", dim(cwd))

	// 1. Scan for .env files
	var found []envFileInfo
	for _, filename := range envFileOrder {
		fpath := filepath.Join(cwd, filename)
		data, readErr := os.ReadFile(fpath)
		if readErr != nil {
			continue // file doesn't exist
		}

		content := string(data)
		count := countEnvPairs(content)
		env := envFileMapping[filename]

		found = append(found, envFileInfo{
			Path:        fpath,
			Filename:    filename,
			Environment: env,
			SecretCount: count,
			Content:     content,
		})
	}

	if len(found) == 0 {
		fmt.Println(yellow("No .env files found in this directory."))
		fmt.Println("Create a .env file or use 'fyvault secrets:create' to add secrets manually.")
		return nil
	}

	// 2. Display summary
	fmt.Printf("Found %d .env file(s):\n\n", len(found))
	for i, f := range found {
		fmt.Printf("  %s [%d] %s → %s (%d secrets)\n",
			cyan(fmt.Sprintf("[%d]", i+1)),
			i+1,
			bold(f.Filename),
			f.Environment,
			f.SecretCount,
		)
	}
	fmt.Println()

	// 3. Ask which files to import
	answer := promptInput("Which files to import? (comma-separated numbers, or 'all'): ")
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = "all"
	}

	var selected []envFileInfo
	if strings.ToLower(answer) == "all" {
		selected = found
	} else {
		parts := strings.Split(answer, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			var idx int
			if _, scanErr := fmt.Sscanf(p, "%d", &idx); scanErr == nil && idx >= 1 && idx <= len(found) {
				selected = append(selected, found[idx-1])
			}
		}
	}

	if len(selected) == 0 {
		fmt.Println("No files selected. Aborting.")
		return nil
	}

	// 4. Fetch environments list to resolve IDs
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

	envIDMap := make(map[string]string)
	for _, e := range envs {
		envIDMap[e.Name] = e.EnvironmentID
	}

	// 5. Import each file
	totalSecrets := 0
	envSet := make(map[string]bool)

	for _, f := range selected {
		targetEnvID, ok := envIDMap[f.Environment]
		if !ok {
			// Try to create the environment
			createBody := map[string]string{"name": f.Environment}
			createResp, createErr := apiRequest("POST", "/orgs/"+oid+"/environments", createBody)
			if createErr != nil {
				fmt.Printf("  %s Skipping %s: environment '%s' not found and could not be created: %v\n",
					yellow("WARN"), f.Filename, f.Environment, createErr)
				continue
			}
			var created struct {
				EnvironmentID string `json:"environment_id"`
			}
			if jsonErr := json.Unmarshal(createResp.Data, &created); jsonErr == nil {
				targetEnvID = created.EnvironmentID
				envIDMap[f.Environment] = targetEnvID
				fmt.Printf("  %s Created environment: %s\n", green("OK"), f.Environment)
			} else {
				fmt.Printf("  %s Skipping %s: could not parse environment creation response\n",
					yellow("WARN"), f.Filename)
				continue
			}
		}

		body := map[string]string{
			"format":            "env",
			"content":           f.Content,
			"duplicateStrategy": "skip",
		}

		resp, importErr := apiRequest("POST", "/orgs/"+oid+"/environments/"+targetEnvID+"/import", body)
		if importErr != nil {
			fmt.Printf("  %s Failed to import %s: %v\n", red("ERROR"), f.Filename, importErr)
			continue
		}

		var result struct {
			Created     int `json:"created"`
			Skipped     int `json:"skipped"`
			Overwritten int `json:"overwritten"`
		}
		if jsonErr := json.Unmarshal(resp.Data, &result); jsonErr == nil {
			fmt.Printf("  %s %s → %s: %d created, %d skipped\n",
				green("OK"), f.Filename, f.Environment, result.Created, result.Skipped)
			totalSecrets += result.Created
			envSet[f.Environment] = true
		}
	}

	fmt.Printf("\n%s Imported %d secrets into %d environment(s)\n",
		green("OK"), totalSecrets, len(envSet))

	// 6. Offer to update .gitignore
	gitignorePath := filepath.Join(cwd, ".gitignore")
	if !gitignoreContainsEnv(gitignorePath) {
		if promptConfirm("Add '.env*' to .gitignore?") {
			if appendErr := appendToGitignore(gitignorePath); appendErr != nil {
				fmt.Printf("  %s Could not update .gitignore: %v\n", yellow("WARN"), appendErr)
			} else {
				fmt.Printf("  %s Added '.env*' to .gitignore\n", green("OK"))
			}
		}
	} else {
		fmt.Printf("  %s .gitignore already contains .env patterns\n", dim("INFO"))
	}

	// 7. Print next steps
	fmt.Printf("\n%s Done! Run %s to start\n",
		green("OK"),
		bold("fyvault run --env=development -- <your command>"))

	return nil
}

// countEnvPairs counts the number of KEY=VALUE pairs in .env content.
func countEnvPairs(content string) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "=") {
			count++
		}
	}
	return count
}

// gitignoreContainsEnv checks if .gitignore already has .env patterns.
func gitignoreContainsEnv(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, ".env") || strings.Contains(content, "*.env")
}

// appendToGitignore adds .env* pattern to .gitignore.
func appendToGitignore(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Check if file ends with newline
	info, _ := f.Stat()
	if info.Size() > 0 {
		// Read last byte to check for newline
		data, readErr := os.ReadFile(path)
		if readErr == nil && len(data) > 0 && data[len(data)-1] != '\n' {
			if _, writeErr := f.WriteString("\n"); writeErr != nil {
				return writeErr
			}
		}
	}

	_, err = f.WriteString("\n# FyVault — ignore .env files\n.env*\n")
	return err
}
