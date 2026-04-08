package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const preCommitHookContent = `#!/bin/sh
# FyVault pre-commit hook — scans staged files for leaked secrets
fyvault scan --staged
`

// ── hook install ──────────────────────────────────────

var hookInstallCmd = &cobra.Command{
	Use:   "hook:install",
	Short: "Install a git pre-commit hook that scans for leaked secrets",
	Long: `Writes a pre-commit hook to .git/hooks/pre-commit that calls
'fyvault scan --staged' before each commit.

If secrets with HIGH confidence are found, the commit is blocked.
MEDIUM confidence findings are warnings only.

Usage:
  fyvault hook:install`,
	RunE: runHookInstall,
}

func runHookInstall(cmd *cobra.Command, args []string) error {
	gitDir, err := findGitDir()
	if err != nil {
		return err
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	hookPath := filepath.Join(hooksDir, "pre-commit")

	// Check if hook already exists
	if _, statErr := os.Stat(hookPath); statErr == nil {
		// Read existing hook to check if it's ours
		existing, readErr := os.ReadFile(hookPath)
		if readErr == nil && contains(string(existing), "fyvault scan") {
			fmt.Printf("%s FyVault pre-commit hook is already installed\n", green("OK"))
			return nil
		}
		if !promptConfirm("A pre-commit hook already exists. Overwrite it?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Ensure hooks directory exists
	if mkErr := os.MkdirAll(hooksDir, 0755); mkErr != nil {
		return fmt.Errorf("failed to create hooks directory: %w", mkErr)
	}

	// Write the hook
	if writeErr := os.WriteFile(hookPath, []byte(preCommitHookContent), 0755); writeErr != nil {
		return fmt.Errorf("failed to write pre-commit hook: %w", writeErr)
	}

	fmt.Printf("%s Installed pre-commit hook at %s\n", green("OK"), dim(hookPath))
	fmt.Printf("  Staged files will be scanned for leaked secrets before each commit.\n")
	return nil
}

// ── hook uninstall ────────────────────────────────────

var hookUninstallCmd = &cobra.Command{
	Use:   "hook:uninstall",
	Short: "Remove the FyVault pre-commit hook",
	Long: `Removes the pre-commit hook installed by 'fyvault hook:install'.

Usage:
  fyvault hook:uninstall`,
	RunE: runHookUninstall,
}

func runHookUninstall(cmd *cobra.Command, args []string) error {
	gitDir, err := findGitDir()
	if err != nil {
		return err
	}

	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")

	data, readErr := os.ReadFile(hookPath)
	if readErr != nil {
		fmt.Printf("%s No pre-commit hook found\n", yellow("WARN"))
		return nil
	}

	if !contains(string(data), "fyvault scan") {
		fmt.Printf("%s The pre-commit hook was not installed by FyVault. Skipping.\n", yellow("WARN"))
		return nil
	}

	if removeErr := os.Remove(hookPath); removeErr != nil {
		return fmt.Errorf("failed to remove hook: %w", removeErr)
	}

	fmt.Printf("%s Removed FyVault pre-commit hook\n", green("OK"))
	return nil
}

// ── helpers ───────────────────────────────────────────

// findGitDir walks up from cwd to find the .git directory.
func findGitDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		if info, statErr := os.Stat(gitPath); statErr == nil && info.IsDir() {
			return gitPath, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("not a git repository (no .git directory found)")
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
