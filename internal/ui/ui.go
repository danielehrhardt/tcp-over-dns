package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
)

// Info prints an informational message.
func Info(format string, args ...interface{}) {
	fmt.Printf(Blue+"[*]"+Reset+" "+format+"\n", args...)
}

// Success prints a success message.
func Success(format string, args ...interface{}) {
	fmt.Printf(Green+"[+]"+Reset+" "+format+"\n", args...)
}

// Warn prints a warning message.
func Warn(format string, args ...interface{}) {
	fmt.Printf(Yellow+"[!]"+Reset+" "+format+"\n", args...)
}

// Error prints an error message.
func Error(format string, args ...interface{}) {
	fmt.Printf(Red+"[-]"+Reset+" "+format+"\n", args...)
}

// Step prints a step indicator.
func Step(step, total int, format string, args ...interface{}) {
	prefix := fmt.Sprintf(Cyan+"[%d/%d]"+Reset+" ", step, total)
	fmt.Printf(prefix+format+"\n", args...)
}

// Header prints a section header.
func Header(title string) {
	line := strings.Repeat("─", 50)
	fmt.Printf("\n%s%s%s\n", Bold, title, Reset)
	fmt.Printf("%s%s%s\n\n", Dim, line, Reset)
}

// Banner prints the application banner.
func Banner(version string) {
	fmt.Printf(`
%s╔╦╗╔═╗╔═╗  ┌┬┐┌┐┌┌─┐
%s ║ ║  ╠═╝   │││││└─┐
%s ╩ ╚═╝╩    ─┴┘┘└┘└─┘%s
%s  TCP over DNS Tunnel%s  %sv%s%s

`, Cyan, Cyan, Cyan, Reset, Dim, Reset, Yellow, version, Reset)
}

// Prompt asks the user for input with a default value.
func Prompt(question string, defaultVal string) string {
	reader := bufio.NewReader(os.Stdin)
	if defaultVal != "" {
		fmt.Printf("%s%s%s [%s%s%s]: ", Bold, question, Reset, Dim, defaultVal, Reset)
	} else {
		fmt.Printf("%s%s%s: ", Bold, question, Reset)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

// PromptSecret asks for a password without showing default.
func PromptSecret(question string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s%s%s: ", Bold, question, Reset)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// Confirm asks a yes/no question.
func Confirm(question string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	fmt.Printf("%s%s%s [%s]: ", Bold, question, Reset, hint)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

// Table prints a simple key-value table.
func Table(rows [][]string) {
	maxKey := 0
	for _, row := range rows {
		if len(row[0]) > maxKey {
			maxKey = len(row[0])
		}
	}
	for _, row := range rows {
		fmt.Printf("  %s%-*s%s  %s\n", Dim, maxKey, row[0], Reset, row[1])
	}
}

// Box prints text in a box.
func Box(title string, content string) {
	lines := strings.Split(content, "\n")
	maxLen := len(title)
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}
	border := strings.Repeat("─", maxLen+2)
	fmt.Printf("  ┌%s┐\n", border)
	fmt.Printf("  │ %s%-*s%s │\n", Bold, maxLen, title, Reset)
	fmt.Printf("  ├%s┤\n", border)
	for _, line := range lines {
		fmt.Printf("  │ %-*s │\n", maxLen, line)
	}
	fmt.Printf("  └%s┘\n", border)
}

// Spinner characters for progress indication.
var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerFrame returns the spinner character for the given frame.
func SpinnerFrame(frame int) string {
	return Cyan + spinnerChars[frame%len(spinnerChars)] + Reset
}

// StatusLine prints a status line with color based on status.
func StatusLine(label, status string) {
	color := Dim
	switch strings.ToLower(status) {
	case "running", "connected", "active", "ok", "pass":
		color = Green
	case "stopped", "disconnected", "inactive", "error", "fail":
		color = Red
	case "unknown", "checking", "pending":
		color = Yellow
	}
	fmt.Printf("  %-20s %s%s%s\n", label, color, status, Reset)
}
