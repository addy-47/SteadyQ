package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// --- Color Palette (High Contrast Adaptive) ---
var (
	// Primary: Deep Purple in light mode, Neon Purple in dark mode
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#5A189A", Dark: "#9D4EDD"}

	// Secondary: Dark Cyan in light mode, Bright Cyan in dark mode
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#023E8A", Dark: "#48CAE4"}

	// Accent/Error: Dark Red in light mode, Bright Coral in dark mode
	ColorAccent    = lipgloss.AdaptiveColor{Light: "#C9184A", Dark: "#FF4D6D"}
	ColorError     = lipgloss.AdaptiveColor{Light: "#C9184A", Dark: "#FF4D6D"}
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#B36700", Dark: "#FFAF00"} // Dark Gold / Gold
	ColorText      = lipgloss.AdaptiveColor{Light: "#111111", Dark: "#FFFFFF"} // Almost Black / White
	ColorSubtle    = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#888888"} // Dark Gray / Light Gray
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#444444"} // Mid Gray / Dark Gray
	ColorBg        = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#121212"} // White / Almost Black
	ColorHighlight = lipgloss.AdaptiveColor{Light: "#EEEEEE", Dark: "#333333"} // Very Light Gray / Dark Gray
	ColorBanner    = lipgloss.AdaptiveColor{Light: "#444444", Dark: "#AAAAAA"} // Variation of Subtle
)

// --- Base Styles ---

var (
	// Main Container Panel
	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2)

	// Titles
	Title = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorSubtle)

	// Text Styles
	Text   = lipgloss.NewStyle().Foreground(ColorText)
	Subtle = lipgloss.NewStyle().Foreground(ColorSubtle)

	// Value metrics
	Value  = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	Active = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	// Alerts
	Error   = lipgloss.NewStyle().Foreground(ColorError)
	Warn    = lipgloss.NewStyle().Foreground(ColorWarning)
	Success = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)

	// Keys
	KeyKey  = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	KeyDesc = lipgloss.NewStyle().Foreground(ColorSubtle)

	// Inputs
	InputActive = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(ColorPrimary).Padding(0, 1)
	InputNormal = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)

	// Box/Card container
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Margin(0, 1)

	// Footer
	TabBase = lipgloss.NewStyle().
		Foreground(ColorSubtle).
		Padding(0, 2)

	TabActive = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorPrimary).
			Padding(0, 2)

	FooterBase = lipgloss.NewStyle().
			Height(1).
			Padding(0, 1)
)

func RenderKey(key, desc string) string {
	return lipgloss.JoinHorizontal(lipgloss.Center,
		KeyKey.Render("<"+key+">"), // Add brackets for style
		" ",
		KeyDesc.Render(desc),
	)
}
