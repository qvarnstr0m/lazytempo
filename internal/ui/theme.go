// Package ui contains the Bubble Tea models and styling for lazytempo.
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Palette defines the colours used throughout the app. Colours are
// AdaptiveColor so the app looks right in both dark and light terminals.
//
// The look follows Charm's own tools: pink/purple accents on a dark background.
var (
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#7D56F4"} // purple
	ColorAccent    = lipgloss.AdaptiveColor{Light: "#F25D94", Dark: "#F25D94"} // pink
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"} // magenta
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"} // green
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#B58B00", Dark: "#ECFD65"} // yellow
	ColorError     = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"} // red
	ColorText      = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#FAFAFA"}
	ColorMuted     = lipgloss.AdaptiveColor{Light: "#777777", Dark: "#777777"}
)

// Theme holds the reusable lipgloss styles. Building it once avoids recreating
// styles on every render.
type Theme struct {
	// Panel borders.
	PanelFocused lipgloss.Style
	PanelBlurred lipgloss.Style

	// Panel titles.
	TitleFocused lipgloss.Style
	TitleBlurred lipgloss.Style

	// Text styles.
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Selected lipgloss.Style

	// Semantic styles.
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Accent  lipgloss.Style

	// Status bar.
	StatusBar lipgloss.Style
	StatusKey lipgloss.Style
}

// lipglossAccent is a convenience style used for the spinner.
func lipglossAccent() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorAccent)
}

// NewTheme builds the default theme.
func NewTheme() Theme {
	base := lipgloss.NewStyle().Foreground(ColorText)
	return Theme{
		PanelFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Foreground(ColorText).
			Padding(0, 1),
		PanelBlurred: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Foreground(ColorText).
			Padding(0, 1),

		TitleFocused: lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true),
		TitleBlurred: lipgloss.NewStyle().Foreground(ColorMuted).Bold(true),

		Text:     base,
		Muted:    lipgloss.NewStyle().Foreground(ColorMuted),
		Selected: lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true),

		Success: lipgloss.NewStyle().Foreground(ColorSuccess),
		Warning: lipgloss.NewStyle().Foreground(ColorWarning),
		Error:   lipgloss.NewStyle().Foreground(ColorError).Bold(true),
		Accent:  lipgloss.NewStyle().Foreground(ColorAccent),

		StatusBar: lipgloss.NewStyle().Foreground(ColorText),
		StatusKey: lipgloss.NewStyle().Foreground(ColorAccent).Bold(true),
	}
}

// panelStyle returns the focused or blurred panel style sized to the given
// content width and height.
func (t Theme) panelStyle(focused bool, width, height int) lipgloss.Style {
	s := t.PanelBlurred
	if focused {
		s = t.PanelFocused
	}
	return s.Width(width).Height(height)
}

// title renders a panel title using the focused/blurred variant.
func (t Theme) title(focused bool, s string) string {
	if focused {
		return t.TitleFocused.Render(s)
	}
	return t.TitleBlurred.Render(s)
}
