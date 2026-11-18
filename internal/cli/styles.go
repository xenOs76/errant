package cli

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorGreen  = lipgloss.Color("#04B575") // Emerald Green
	colorPurple = lipgloss.Color("#7D56F4") // Purple
	colorCyan   = lipgloss.Color("#00D7D7") // Cyan
	colorAmber  = lipgloss.Color("#FFAF00") // Amber / Warning
	colorSlate  = lipgloss.Color("#767676") // Muted Slate
)

// formatStep creates a standardized Errant CLI step header: "==> [tag] message".
func formatStep(tag, message string) string {
	arrow := lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("==>")
	badge := lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render("[" + tag + "]")

	return arrow + " " + badge + " " + message
}

// formatWarningHeader formats a prominent warning banner for destructive operations.
func formatWarningHeader(message string) string {
	badge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#000000")).
		Background(colorAmber).
		Padding(0, 1).
		Render("WARNING")

	text := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorAmber).
		Render(message)

	return badge + " " + text
}

// formatDomainStatus returns an appropriately colored domain status badge.
func formatDomainStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render(status)
	case "shut off", "paused", "halted":
		return lipgloss.NewStyle().Foreground(colorAmber).Render(status)
	case "not created":
		return lipgloss.NewStyle().Foreground(colorSlate).Render(status)
	default:
		return status
	}
}

// formatPrompt creates a styled interactive confirmation prompt.
func formatPrompt(message, options string) string {
	styledMsg := lipgloss.NewStyle().Bold(true).Render(message)
	styledOpts := lipgloss.NewStyle().Foreground(colorAmber).Bold(true).Render(options)

	return styledMsg + " " + styledOpts + ": "
}

// padStyled pads a styled string to target visual width using lipgloss.Width.
func padStyled(styled string, targetWidth int) string {
	w := lipgloss.Width(styled)
	if w >= targetWidth {
		return styled
	}

	return styled + strings.Repeat(" ", targetWidth-w)
}

// formatCheckmark returns a green checkmark symbol with message.
func formatCheckmark(message string) string {
	check := lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("✔")

	return check + " " + message
}
