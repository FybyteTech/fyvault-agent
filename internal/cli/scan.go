package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan text or files for leaked secrets",
	Long: `Scan text, files, or repository content for leaked secrets.

Usage:
  fyvault scan --file=.env.backup
  fyvault scan --text="AWS_KEY=AKIA..."
  cat logs.txt | fyvault scan --stdin`,
	RunE: runScan,
}

var (
	scanFile   string
	scanText   string
	scanStdin  bool
	scanStaged bool
)

func init() {
	scanCmd.Flags().StringVar(&scanFile, "file", "", "Path to file to scan")
	scanCmd.Flags().StringVar(&scanText, "text", "", "Text to scan directly")
	scanCmd.Flags().BoolVar(&scanStdin, "stdin", false, "Read from stdin")
	scanCmd.Flags().BoolVar(&scanStaged, "staged", false, "Scan git staged files (for pre-commit hooks)")
}

func runScan(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	var content string

	switch {
	case scanStaged:
		stagedContent, stagedErr := readStagedFiles()
		if stagedErr != nil {
			return stagedErr
		}
		content = stagedContent
	case scanFile != "":
		data, err := os.ReadFile(scanFile)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		content = string(data)
	case scanText != "":
		content = scanText
	case scanStdin:
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		content = string(data)
	default:
		return fmt.Errorf("specify --file, --text, --stdin, or --staged")
	}

	sourceRef := scanFile
	if sourceRef == "" {
		sourceRef = "cli-scan"
	}

	body := map[string]string{
		"text":      content,
		"sourceRef": sourceRef,
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/scan/text", body)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	var result struct {
		Findings []struct {
			PatternName string `json:"pattern_name"`
			MatchedText string `json:"matched_text"`
			LineNumber  int    `json:"line_number"`
			Confidence  string `json:"confidence"`
		} `json:"findings"`
		TotalFindings int `json:"total_findings"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.TotalFindings == 0 {
		fmt.Printf("%s No secrets detected\n", green("OK"))
		return nil
	}

	fmt.Printf("%s Found %d potential secret(s):\n\n", yellow("WARN"), result.TotalFindings)

	for _, f := range result.Findings {
		conf := f.Confidence
		switch conf {
		case "high":
			conf = red("HIGH")
		case "medium":
			conf = yellow("MEDIUM")
		default:
			conf = cyan("LOW")
		}
		fmt.Printf("  Line %d: [%s] %s — %s\n", f.LineNumber, conf, f.PatternName, f.MatchedText)
	}

	fmt.Printf("\n  %s Consider rotating any exposed secrets immediately.\n", yellow(">>"))

	// In --staged mode, block commit if HIGH confidence findings exist
	if scanStaged {
		hasHigh := false
		for _, f := range result.Findings {
			if strings.ToLower(f.Confidence) == "high" {
				hasHigh = true
				break
			}
		}
		if hasHigh {
			fmt.Printf("\n  %s Commit blocked — HIGH confidence secrets detected in staged files.\n", red("BLOCKED"))
			os.Exit(1)
		}
		fmt.Printf("\n  %s Allowing commit — only MEDIUM/LOW findings detected.\n", yellow("PASS"))
	}

	return nil
}

// readStagedFiles uses git to read staged file contents and concatenate them.
func readStagedFiles() (string, error) {
	// Get list of staged files
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get staged files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(files) == 0 || (len(files) == 1 && files[0] == "") {
		return "", fmt.Errorf("no staged files found")
	}

	var allContent strings.Builder
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		data, readErr := os.ReadFile(f)
		if readErr != nil {
			continue // file might have been deleted
		}
		allContent.WriteString(fmt.Sprintf("# File: %s\n", f))
		allContent.Write(data)
		allContent.WriteString("\n")
	}

	if allContent.Len() == 0 {
		return "", fmt.Errorf("no readable staged files found")
	}

	return allContent.String(), nil
}
