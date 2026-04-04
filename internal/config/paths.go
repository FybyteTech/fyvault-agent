package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultConfigPath returns the OS-appropriate default config file path.
func DefaultConfigPath() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(userConfigDir(), "fyvault", "fyvault.conf")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "fyvault", "fyvault.conf")
	default:
		return "/etc/fyvault/fyvault.conf"
	}
}

// DefaultHealthAddr returns the OS-appropriate default health endpoint address.
// On Unix systems this is a socket path; on Windows it is a TCP address.
func DefaultHealthAddr() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(userConfigDir(), "fyvault", "health.sock")
	case "windows":
		return "127.0.0.1:19476"
	default:
		return "/var/run/fyvault/health.sock"
	}
}

// DefaultDataDir returns the OS-appropriate data directory.
func DefaultDataDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(userConfigDir(), "fyvault")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "fyvault")
	default:
		return "/var/lib/fyvault"
	}
}

func userConfigDir() string {
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support")
	}
	dir, _ := os.UserConfigDir()
	return dir
}
