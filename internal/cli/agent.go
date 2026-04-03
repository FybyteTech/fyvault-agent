package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var agentStatusCmd = &cobra.Command{
	Use:   "agent:status",
	Short: "Check if the FyVault agent is running",
	RunE:  runAgentStatus,
}

var agentLogsCmd = &cobra.Command{
	Use:   "agent:logs",
	Short: "Tail the FyVault agent logs",
	Long:  "Shows recent log output from the FyVault agent (via journalctl).",
	RunE:  runAgentLogs,
}

var agentRestartCmd = &cobra.Command{
	Use:   "agent:restart",
	Short: "Restart the FyVault agent",
	RunE:  runAgentRestart,
}

func runAgentStatus(_ *cobra.Command, _ []string) error {
	fmt.Println(bold("FyVault Agent Status"))
	fmt.Println()

	// Try the health socket
	conn, err := net.Dial("unix", "/var/run/fyvault/health.sock")
	if err != nil {
		// Try systemd status as fallback
		out, svcErr := exec.Command("systemctl", "is-active", "fyvault-agent").Output()
		if svcErr != nil {
			fmt.Printf("  Status:  %s\n", red("not running"))
			fmt.Println()
			fmt.Println(dim("  The agent service is not active. Start it with:"))
			fmt.Println(dim("  sudo systemctl start fyvault-agent"))
			return nil
		}
		status := strings.TrimSpace(string(out))
		if status == "active" {
			fmt.Printf("  Status:  %s\n", green("running"))
		} else {
			fmt.Printf("  Status:  %s\n", yellow(status))
		}
		return nil
	}
	defer conn.Close()

	// Read health response
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("  Status:  %s\n", green("running"))
		fmt.Println("  (health socket connected but no response)")
		return nil
	}

	var health struct {
		Status         string `json:"status"`
		Uptime         string `json:"uptime"`
		SecretsLoaded  int    `json:"secretsLoaded"`
		DevicesManaged int    `json:"devicesManaged"`
		Version        string `json:"version"`
	}
	if jsonErr := json.Unmarshal(buf[:n], &health); jsonErr == nil {
		fmt.Printf("  Status:   %s\n", green(health.Status))
		if health.Version != "" {
			fmt.Printf("  Version:  %s\n", health.Version)
		}
		if health.Uptime != "" {
			fmt.Printf("  Uptime:   %s\n", health.Uptime)
		}
		fmt.Printf("  Secrets:  %d loaded\n", health.SecretsLoaded)
	} else {
		fmt.Printf("  Status:  %s\n", green("running"))
		fmt.Printf("  Health:  %s\n", string(buf[:n]))
	}

	return nil
}

func runAgentLogs(_ *cobra.Command, _ []string) error {
	// Use journalctl to tail logs
	cmd := exec.Command("journalctl", "-u", "fyvault-agent", "-n", "50", "--no-pager", "-o", "short-iso")
	cmd.Stdout = nil
	cmd.Stderr = nil

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to read logs (is journalctl available?): %w\n%s", err, string(out))
	}

	fmt.Print(string(out))
	return nil
}

func runAgentRestart(_ *cobra.Command, _ []string) error {
	fmt.Println("Restarting FyVault agent...")

	cmd := exec.Command("sudo", "systemctl", "restart", "fyvault-agent")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart agent: %w\n%s", err, string(out))
	}

	printSuccess("Agent restarted successfully")
	return nil
}
