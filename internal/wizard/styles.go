package wizard

import "github.com/charmbracelet/lipgloss"

// Palette holds the consistent visual color definitions.
var (
	ColorPrimary   = lipgloss.Color("#7D56F4") // Vibrant Purple
	ColorSecondary = lipgloss.Color("#00D7D7") // Cyan
	ColorAccent    = lipgloss.Color("#FF5F87") // Coral Pink
	ColorMuted     = lipgloss.Color("#626262") // Slate Grey
	ColorSuccess   = lipgloss.Color("#04B575") // Emerald Green
	ColorLight     = lipgloss.Color("#EEEEEE") // Soft White
	ColorCardBg    = lipgloss.Color("#1A1A24") // Dark Container
)

// Styles bundle all reusable Lipgloss formatting functions.
type Styles struct {
	Header         lipgloss.Style
	Subtitle       lipgloss.Style
	StepBadge      lipgloss.Style
	Title          lipgloss.Style
	Card           lipgloss.Style
	Item           lipgloss.Style
	SelectedItem   lipgloss.Style
	ItemDesc       lipgloss.Style
	Help           lipgloss.Style
	InputPrompt    lipgloss.Style
	SuccessTitle   lipgloss.Style
	SuccessMessage lipgloss.Style
	PreviewBox     lipgloss.Style
}

// DefaultStyles returns the themed Lipgloss style definitions for Errant.
func DefaultStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorLight).
			Background(ColorPrimary).
			Padding(0, 2).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Italic(true).
			MarginBottom(1),

		StepBadge: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1).
			MarginRight(1),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorLight),

		Card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			MarginBottom(1),

		Item: lipgloss.NewStyle().
			Foreground(ColorLight).
			PaddingLeft(2),

		SelectedItem: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary).
			PaddingLeft(0),

		ItemDesc: lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingLeft(4),

		Help: lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1),

		InputPrompt: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true),

		SuccessTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess).
			MarginBottom(1),

		SuccessMessage: lipgloss.NewStyle().
			Foreground(ColorLight).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess),

		PreviewBox: lipgloss.NewStyle().
			Foreground(ColorLight).
			Background(ColorCardBg).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			MarginTop(1).
			MarginBottom(1),
	}
}
