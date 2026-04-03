package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// ANSI color helpers

func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func bold(s string) string   { return "\033[1m" + s + "\033[0m" }
func cyan(s string) string   { return "\033[36m" + s + "\033[0m" }
func dim(s string) string    { return "\033[2m" + s + "\033[0m" }

// printTable prints a formatted ASCII table with colored headers.
func printTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	// Calculate column widths.
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header.
	headerLine := ""
	separatorLine := ""
	for i, h := range headers {
		if i > 0 {
			headerLine += "  "
			separatorLine += "  "
		}
		headerLine += bold(padRight(h, widths[i]))
		separatorLine += strings.Repeat("-", widths[i])
	}
	fmt.Println(headerLine)
	fmt.Println(separatorLine)

	// Print rows.
	for _, row := range rows {
		line := ""
		for i := 0; i < len(headers); i++ {
			if i > 0 {
				line += "  "
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			line += padRight(cell, widths[i])
		}
		fmt.Println(line)
	}

	if len(rows) == 0 {
		fmt.Println(dim("  (none)"))
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// promptInput reads a line of input from stdin.
func promptInput(label string) string {
	fmt.Print(label)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

// promptPassword reads hidden input from stdin.
func promptPassword(label string) string {
	fmt.Print(label)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // newline after hidden input
	if err != nil {
		return ""
	}
	return string(password)
}

// promptConfirm asks a yes/no question, default no.
func promptConfirm(message string) bool {
	answer := promptInput(message + " (y/N): ")
	return strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes"
}

// promptSelect shows numbered options and returns the selected index and value.
func promptSelect(label string, options []string) (int, string) {
	fmt.Println(label)
	for i, opt := range options {
		fmt.Printf("  %s %s\n", cyan(fmt.Sprintf("[%d]", i+1)), opt)
	}
	for {
		answer := promptInput("Select: ")
		var idx int
		if _, err := fmt.Sscanf(answer, "%d", &idx); err == nil && idx >= 1 && idx <= len(options) {
			return idx - 1, options[idx-1]
		}
		fmt.Println(red("Invalid selection. Try again."))
	}
}

// printSuccess prints a green success message.
func printSuccess(msg string) {
	fmt.Println(green("OK") + " " + msg)
}

// printError prints a red error message.
func printError(msg string) {
	fmt.Println(red("ERROR") + " " + msg)
}

// printCheck prints a checklist item.
func printCheck(ok bool, label string) {
	if ok {
		fmt.Printf("  %s %s\n", green("[PASS]"), label)
	} else {
		fmt.Printf("  %s %s\n", red("[FAIL]"), label)
	}
}
