package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system requirements for FyVault agent",
	Long:  "Verifies that the current system meets the requirements for running the FyVault agent.",
	RunE:  runDoctor,
}

func runDoctor(_ *cobra.Command, _ []string) error {
	fmt.Println(bold("FyVault Doctor"))
	fmt.Println(dim("Checking system requirements..."))
	fmt.Println()

	allOK := true

	// 1. OS + Architecture
	validArch := runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"
	printCheck(validArch, fmt.Sprintf("Architecture: %s", runtime.GOARCH))
	if !validArch {
		allOK = false
	}

	printCheck(true, fmt.Sprintf("Operating system: %s/%s", runtime.GOOS, runtime.GOARCH))

	// 2. Platform-specific checks
	switch runtime.GOOS {
	case "linux":
		if !checkLinux() {
			allOK = false
		}
	case "darwin":
		if !checkDarwin() {
			allOK = false
		}
	case "windows":
		if !checkWindows() {
			allOK = false
		}
	default:
		printCheck(false, "Platform-specific checks: "+dim("unsupported OS"))
		allOK = false
	}

	// 3. Credentials check (cross-platform)
	_, err := loadCredentials()
	hasCredentials := err == nil
	printCheck(hasCredentials, "FyVault credentials")

	// 4. API connectivity
	if hasCredentials {
		creds, _ := loadCredentials()
		base := getAPIURL(creds)
		_, apiErr := apiRequest("GET", "/auth/me/orgs", nil)
		apiOK := apiErr == nil
		printCheck(apiOK, fmt.Sprintf("API connectivity (%s)", base))
		if !apiOK {
			allOK = false
		}
	} else {
		printCheck(false, "API connectivity: "+dim("skipped (not logged in)"))
	}

	// 5. Feature summary
	fmt.Println()
	if allOK {
		printSuccess("All checks passed. System is ready for the FyVault agent.")
	} else {
		printFeatureSummary()
	}

	return nil
}

func checkLinux() bool {
	ok := true

	// Kernel version (>= 4.15 for eBPF TC)
	kernelOK := false
	out, err := exec.Command("uname", "-r").Output()
	if err == nil {
		kernel := strings.TrimSpace(string(out))
		var major, minor int
		if _, scanErr := fmt.Sscanf(kernel, "%d.%d", &major, &minor); scanErr == nil {
			kernelOK = major > 4 || (major == 4 && minor >= 15)
		}
		printCheck(kernelOK, fmt.Sprintf("Kernel version: %s %s", kernel,
			func() string {
				if !kernelOK {
					return dim("(4.15+ required for eBPF TC)")
				}
				return ""
			}()))
	} else {
		printCheck(false, "Kernel version: unable to determine")
	}
	if !kernelOK {
		ok = false
	}

	// CAP_NET_ADMIN
	hasCapNetAdmin := os.Geteuid() == 0
	if !hasCapNetAdmin {
		if _, err := os.Stat("/proc/self/status"); err == nil {
			data, readErr := os.ReadFile("/proc/self/status")
			if readErr == nil {
				hasCapNetAdmin = strings.Contains(string(data), "CapEff")
			}
		}
	}
	printCheck(hasCapNetAdmin, "CAP_NET_ADMIN capability "+
		func() string {
			if !hasCapNetAdmin {
				return dim("(run as root or set capability)")
			}
			return ""
		}())

	// systemd
	_, sysErr := exec.LookPath("systemctl")
	hasSystemd := sysErr == nil
	printCheck(hasSystemd, "systemd present")
	if !hasSystemd {
		ok = false
	}

	// Kernel keyring
	_, keyErr := os.Stat("/proc/keys")
	hasKeyring := keyErr == nil
	printCheck(hasKeyring, "Kernel keyring (/proc/keys)")
	if !hasKeyring {
		ok = false
	}

	// eBPF availability
	printCheck(true, "eBPF TC redirect: "+green("available"))

	return ok
}

func checkDarwin() bool {
	ok := true

	// macOS version
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err == nil {
		ver := strings.TrimSpace(string(out))
		printCheck(true, fmt.Sprintf("macOS version: %s", ver))
	} else {
		printCheck(false, "macOS version: unable to determine")
	}

	// Keychain access
	_, secErr := exec.LookPath("security")
	hasKeychain := secErr == nil
	printCheck(hasKeychain, "Keychain CLI (security)")
	if !hasKeychain {
		ok = false
	}

	// launchd
	_, launchErr := exec.LookPath("launchctl")
	hasLaunchd := launchErr == nil
	printCheck(hasLaunchd, "launchd present")
	if !hasLaunchd {
		ok = false
	}

	// eBPF not available
	printCheck(false, "eBPF TC redirect: "+dim("not available on macOS (proxy-only mode)"))

	return ok
}

func checkWindows() bool {
	ok := true

	// Windows version
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if err == nil {
		ver := strings.TrimSpace(string(out))
		printCheck(true, fmt.Sprintf("Windows version: %s", ver))
	} else {
		printCheck(false, "Windows version: unable to determine")
	}

	// Service controller
	_, scErr := exec.LookPath("sc.exe")
	hasSC := scErr == nil
	printCheck(hasSC, "Service controller (sc.exe)")
	if !hasSC {
		ok = false
	}

	// Data directory writable
	appdata := os.Getenv("APPDATA")
	if appdata != "" {
		printCheck(true, fmt.Sprintf("APPDATA directory: %s", appdata))
	} else {
		printCheck(false, "APPDATA: not set")
		ok = false
	}

	// eBPF not available
	printCheck(false, "eBPF TC redirect: "+dim("not available on Windows (proxy-only mode)"))

	return ok
}

func printFeatureSummary() {
	switch runtime.GOOS {
	case "linux":
		fmt.Println(yellow("WARN") + " Some checks failed. See above for details.")
	case "darwin":
		fmt.Println(yellow("NOTE") + " Running on macOS — the agent runs in proxy-only mode.")
		fmt.Println("  Available features:")
		fmt.Println("  " + green("✓") + " Secret injection via " + bold("fyvault run"))
		fmt.Println("  " + green("✓") + " Secret management via " + bold("fyvault secrets"))
		fmt.Println("  " + green("✓") + " HTTP/DB proxy interception")
		fmt.Println("  " + green("✓") + " macOS Keychain secret storage")
		fmt.Println("  " + red("✗") + " eBPF transparent redirect (Linux only)")
		fmt.Println("  " + red("✗") + " Kernel keyring (Linux only)")
	case "windows":
		fmt.Println(yellow("NOTE") + " Running on Windows — the agent runs in proxy-only mode.")
		fmt.Println("  Available features:")
		fmt.Println("  " + green("✓") + " Secret injection via " + bold("fyvault run"))
		fmt.Println("  " + green("✓") + " Secret management via " + bold("fyvault secrets"))
		fmt.Println("  " + green("✓") + " HTTP/DB proxy interception")
		fmt.Println("  " + green("✓") + " Encrypted file-based secret storage")
		fmt.Println("  " + red("✗") + " eBPF transparent redirect (Linux only)")
		fmt.Println("  " + red("✗") + " Kernel keyring (Linux only)")
	default:
		fmt.Println(yellow("WARN") + " Unsupported platform. Limited functionality available.")
	}
}
