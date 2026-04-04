package cli

import (
	"encoding/json"
	"fmt"
	"os"

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
	scanFile  string
	scanText  string
	scanStdin bool
)

func init() {
	scanCmd.Flags().StringVar(&scanFile, "file", "", "Path to file to scan")
	scanCmd.Flags().StringVar(&scanText, "text", "", "Text to scan directly")
	scanCmd.Flags().BoolVar(&scanStdin, "stdin", false, "Read from stdin")
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
		return fmt.Errorf("specify --file, --text, or --stdin")
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
	return nil
}
