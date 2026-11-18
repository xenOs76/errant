package pulumi

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

var (
	colorGreen = lipgloss.Color("#04B575")
	colorCoral = lipgloss.Color("#FF5F87")
	colorAmber = lipgloss.Color("#FFAF00")
	colorSlate = lipgloss.Color("#767676")
	colorCyan  = lipgloss.Color("#00D7D7")
)

// ProcessEngineEvents reads events from eventCh and writes formatted status lines to out.
func ProcessEngineEvents(eventCh <-chan events.EngineEvent, out io.Writer) {
	for event := range eventCh {
		switch {
		case event.ResourcePreEvent != nil:
			renderPreEvent(event.ResourcePreEvent, out)
		case event.DiagnosticEvent != nil:
			renderDiagnosticEvent(event.DiagnosticEvent, out)
		default:
		}
	}
}

func renderPreEvent(pre *apitype.ResourcePreEvent, out io.Writer) {
	meta := pre.Metadata
	urnParts := strings.Split(meta.URN, "::")
	resourceLabel := meta.URN

	if len(urnParts) >= 4 {
		resourceLabel = urnParts[len(urnParts)-1]
	}

	op := strings.ToUpper(string(meta.Op))
	if op == "SAME" {
		return
	}

	icon := "•"
	iconColor := colorSlate

	switch meta.Op {
	case apitype.OpCreate:
		icon = "+"
		iconColor = colorGreen
	case apitype.OpDelete:
		icon = "-"
		iconColor = colorCoral
	case apitype.OpUpdate:
		icon = "~"
		iconColor = colorAmber
	default:
	}

	arrow := lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("==>")
	styledIcon := lipgloss.NewStyle().Foreground(iconColor).Bold(true).Render(icon)
	styledLabel := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(resourceLabel)
	styledType := lipgloss.NewStyle().Foreground(colorSlate).Render("(" + meta.Type + ")")

	fmt.Fprintf(out, " %s [%s] %s %s\n", arrow, styledIcon, styledLabel, styledType)
}

func renderDiagnosticEvent(diag *apitype.DiagnosticEvent, out io.Writer) {
	if diag.Severity == "error" {
		badge := lipgloss.NewStyle().Foreground(colorCoral).Bold(true).Render("[!]")
		fmt.Fprintf(out, " %s %s\n", badge, strings.TrimSpace(diag.Message))
	}
}
