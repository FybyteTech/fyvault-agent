package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fybyte/fyvault-agent/internal/config"
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
	Long:  "Shows recent log output from the FyVault agent.",
	RunE:  runAgentLogs,
}

var agentRestartCmd = &cobra.Command{
	Use:   "agent:restart",
	Short: "Restart the FyVault agent",
	RunE:  runAgentRestart,
}

func healthAddr() string {
	return config.DefaultHealthAddr()
}

func runAgentStatus(_ *cobra.Command, _ []string) error {
	fmt.Println(bold("FyVault Agent Status"))
	fmt.Println()

	addr := healthAddr()
	network := "unix"
	if runtime.GOOS == "windows" {
		network = "tcp"
	}

	conn, err := net.Dial(network, addr)
	if err != nil {
		return agentStatusFallback()
	}
	defer conn.Close()

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

func agentStatusFallback() error {
	switch runtime.GOOS {
	case "linux":
		out, err := exec.Command("systemctl", "is-active", "fyvault-agent").Output()
		if err != nil {
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

	case "darwin":
		out, err := exec.Command("launchctl", "list", "com.fybyte.fyvault-agent").Output()
		if err != nil {
			fmt.Printf("  Status:  %s\n", red("not running"))
			fmt.Println()
			fmt.Println(dim("  The agent service is not loaded. Load it with:"))
			fmt.Println(dim("  sudo launchctl load /Library/LaunchDaemons/com.fybyte.fyvault-agent.plist"))
			return nil
		}
		if strings.Contains(string(out), "\"PID\"") {
			fmt.Printf("  Status:  %s\n", green("running"))
		} else {
			fmt.Printf("  Status:  %s\n", yellow("loaded but not running"))
		}

	case "windows":
		out, err := exec.Command("sc.exe", "query", "fyvault-agent").CombinedOutput()
		if err != nil {
			fmt.Printf("  Status:  %s\n", red("not running"))
			fmt.Println()
			fmt.Println(dim("  The agent service is not installed or not running. Start it with:"))
			fmt.Println(dim("  sc.exe start fyvault-agent"))
			return nil
		}
		outStr := string(out)
		if strings.Contains(outStr, "RUNNING") {
			fmt.Printf("  Status:  %s\n", green("running"))
		} else if strings.Contains(outStr, "STOPPED") {
			fmt.Printf("  Status:  %s\n", yellow("stopped"))
		} else {
			fmt.Printf("  Status:  %s\n", yellow("unknown"))
		}

	default:
		fmt.Printf("  Status:  %s\n", red("not running"))
		fmt.Println(dim("  Unsupported platform for service management"))
	}

	return nil
}

func runAgentLogs(_ *cobra.Command, _ []string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("journalctl", "-u", "fyvault-agent", "-n", "50", "--no-pager", "-o", "short-iso")
	case "darwin":
		cmd = exec.Command("log", "show", "--predicate",
			`subsystem == "com.fybyte.fyvault-agent"`,
			"--last", "5m", "--style", "compact")
	case "windows":
		cmd = exec.Command("powershell", "-Command",
			`Get-EventLog -LogName Application -Source "fyvault-agent" -Newest 50 | Format-Table -AutoSize`)
	default:
		return fmt.Errorf("agent log viewing is not supported on %s", runtime.GOOS)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to read logs: %w\n%s", err, string(out))
	}

	fmt.Print(string(out))
	return nil
}

func runAgentRestart(_ *cobra.Command, _ []string) error {
	fmt.Println("Restarting FyVault agent...")

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("sudo", "systemctl", "restart", "fyvault-agent")
	case "darwin":
		cmd = exec.Command("sudo", "launchctl", "kickstart", "-k",
			"system/com.fybyte.fyvault-agent")
	case "windows":
		cmd = exec.Command("powershell", "-Command",
			"Restart-Service -Name fyvault-agent -Force")
	default:
		return fmt.Errorf("agent restart is not supported on %s", runtime.GOOS)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart agent: %w\n%s", err, string(out))
	}

	printSuccess("Agent restarted successfully")
	return nil
}
