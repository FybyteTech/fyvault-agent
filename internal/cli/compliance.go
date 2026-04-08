package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var complianceReportCmd = &cobra.Command{
	Use:   "compliance:report",
	Short: "Generate a compliance report",
	Long: `Generates a compliance report (SOC2, HIPAA, or ISO 27001) for the organization.

Usage:
  fyvault compliance:report --type=soc2 --period=90
  fyvault compliance:report --type=hipaa --period=180 --output=report.json`,
	RunE: runComplianceReport,
}

var (
	complianceType   string
	compliancePeriod int
	complianceOutput string
)

func init() {
	complianceReportCmd.Flags().StringVar(&complianceType, "type", "soc2", "Report type: soc2, hipaa, iso27001")
	complianceReportCmd.Flags().IntVar(&compliancePeriod, "period", 90, "Reporting period in days")
	complianceReportCmd.Flags().StringVar(&complianceOutput, "output", "", "Write report to file (JSON)")
}

func runComplianceReport(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	url := fmt.Sprintf("/orgs/%s/compliance/report?type=%s&period=%d",
		oid, complianceType, compliancePeriod)

	resp, err := apiRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to generate compliance report: %w", err)
	}

	var report map[string]interface{}
	if err := json.Unmarshal(resp.Data, &report); err != nil {
		return fmt.Errorf("failed to parse report: %w", err)
	}

	pretty, _ := json.MarshalIndent(report, "", "  ")

	// Write to file if --output is specified
	if complianceOutput != "" {
		if err := os.WriteFile(complianceOutput, pretty, 0644); err != nil {
			return fmt.Errorf("failed to write report to %s: %w", complianceOutput, err)
		}
		printSuccess(fmt.Sprintf("Report written to %s", bold(complianceOutput)))
		return nil
	}

	if format == "json" {
		fmt.Println(string(pretty))
		return nil
	}

	// Pretty-print summary
	fmt.Printf("%s %s compliance report generated\n\n", green("OK"), bold(complianceType))
	fmt.Printf("  Report type: %s\n", complianceType)
	fmt.Printf("  Period:      %d days\n", compliancePeriod)

	if sections, ok := report["sections"].(map[string]interface{}); ok {
		if audit, ok := sections["auditSummary"].(map[string]interface{}); ok {
			if total, ok := audit["totalEvents"].(float64); ok {
				fmt.Printf("  Audit events: %d\n", int(total))
			}
		}
		if secrets, ok := sections["secretsOverview"].(map[string]interface{}); ok {
			if total, ok := secrets["totalSecrets"].(float64); ok {
				fmt.Printf("  Secrets:      %d\n", int(total))
			}
		}
		if devices, ok := sections["deviceSecurity"].(map[string]interface{}); ok {
			if total, ok := devices["totalDevices"].(float64); ok {
				fmt.Printf("  Devices:      %d\n", int(total))
			}
		}
	}

	fmt.Printf("\n  Use %s to save the full report.\n",
		dim("--output=report.json"))

	return nil
}
