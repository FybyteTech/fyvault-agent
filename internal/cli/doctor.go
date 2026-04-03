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

	// 1. OS check
	isLinux := runtime.GOOS == "linux"
	printCheck(isLinux, fmt.Sprintf("Operating system: %s/%s %s",
		runtime.GOOS, runtime.GOARCH,
		func() string {
			if isLinux {
				return ""
			}
			return dim("(Linux required for full agent)")
		}()))
	if !isLinux {
		allOK = false
	}

	// 2. Architecture
	validArch := runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"
	printCheck(validArch, fmt.Sprintf("Architecture: %s", runtime.GOARCH))
	if !validArch {
		allOK = false
	}

	// 3. Kernel version (Linux only)
	if isLinux {
		kernelOK := false
		out, err := exec.Command("uname", "-r").Output()
		if err == nil {
			kernel := strings.TrimSpace(string(out))
			// Check >= 4.15 for eBPF TC support
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
			allOK = false
		}
	} else {
		printCheck(false, "Kernel version: "+dim("skipped (not Linux)"))
	}

	// 4. CAP_NET_ADMIN (Linux only)
	if isLinux {
		// Check if running as root or has capability
		hasCapNetAdmin := os.Geteuid() == 0
		if !hasCapNetAdmin {
			// Try checking capabilities via getpcaps or /proc
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
	} else {
		printCheck(false, "CAP_NET_ADMIN: "+dim("skipped (not Linux)"))
	}

	// 5. systemd
	if isLinux {
		_, err := exec.LookPath("systemctl")
		hasSystemd := err == nil
		printCheck(hasSystemd, "systemd present")
		if !hasSystemd {
			allOK = false
		}
	} else {
		printCheck(false, "systemd: "+dim("skipped (not Linux)"))
	}

	// 6. Kernel keyring (/proc/keys)
	if isLinux {
		_, err := os.Stat("/proc/keys")
		hasKeyring := err == nil
		printCheck(hasKeyring, "Kernel keyring (/proc/keys)")
		if !hasKeyring {
			allOK = false
		}
	} else {
		printCheck(false, "Kernel keyring: "+dim("skipped (not Linux)"))
	}

	// 7. Credentials check
	_, err := loadCredentials()
	hasCredentials := err == nil
	printCheck(hasCredentials, "FyVault credentials")

	// 8. API connectivity
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

	fmt.Println()
	if allOK {
		printSuccess("All checks passed. System is ready for the FyVault agent.")
	} else if !isLinux {
		fmt.Println(yellow("NOTE") + " The full FyVault agent requires Linux. On " + runtime.GOOS + ", you can use:")
		fmt.Println("  - " + bold("fyvault run") + " to inject secrets into local processes")
		fmt.Println("  - " + bold("fyvault secrets") + " to manage secrets via the API")
	} else {
		fmt.Println(yellow("WARN") + " Some checks failed. See above for details.")
	}

	return nil
}
