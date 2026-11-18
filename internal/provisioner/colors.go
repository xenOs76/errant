package provisioner

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorArrow   = lipgloss.Color("#04B575") // Emerald Green
	colorStderr  = lipgloss.Color("#FF5F87") // Coral Pink
	colorDim     = lipgloss.Color("#767676") // Muted Slate
	colorStepTag = lipgloss.Color("#E5C07B") // Warm Gold

	nodePalette = []lipgloss.Color{
		lipgloss.Color("#00D7D7"), // Cyan
		lipgloss.Color("#7D56F4"), // Purple
		lipgloss.Color("#00AF87"), // Teal
		lipgloss.Color("#FFAF00"), // Amber
		lipgloss.Color("#FF79C6"), // Pink
		lipgloss.Color("#8BE9FD"), // Sky Blue
		lipgloss.Color("#50FA7B"), // Mint
	}
)

// NodeColor returns a consistent lipgloss color for a given node name.
func NodeColor(nodeName string) lipgloss.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(nodeName))

	idx := int(h.Sum32()) % len(nodePalette)
	if idx < 0 {
		idx = -idx
	}

	return nodePalette[idx]
}

// FormatStep formats an Errant step header with colors: "==> [nodeName] message".
func FormatStep(nodeName, message string) string {
	arrow := lipgloss.NewStyle().Foreground(colorArrow).Bold(true).Render("==>")
	badge := lipgloss.NewStyle().Foreground(NodeColor(nodeName)).Bold(true).Render("[" + nodeName + "]")

	return arrow + " " + badge + " " + message
}

// FormatNodePrefix formats the indentation prefix for regular stdout log lines: "    nodeName: ".
func FormatNodePrefix(nodeName string) string {
	styledNode := lipgloss.NewStyle().Foreground(NodeColor(nodeName)).Bold(true).Render(nodeName)
	colon := lipgloss.NewStyle().Foreground(colorDim).Render(":")

	return "    " + styledNode + colon + " "
}

// FormatNodeStderrPrefix formats the indentation prefix for stderr log lines: "    nodeName: ".
func FormatNodeStderrPrefix(nodeName string) string {
	styledNode := lipgloss.NewStyle().Foreground(colorStderr).Bold(true).Render(nodeName)
	colon := lipgloss.NewStyle().Foreground(colorDim).Render(":")

	return "    " + styledNode + colon + " "
}

// StyleStepTag styles step identifiers like [7/7] with a warm accent.
func StyleStepTag(tag string) string {
	return lipgloss.NewStyle().Foreground(colorStepTag).Bold(true).Render(tag)
}
