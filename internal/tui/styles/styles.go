package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// --- Color Palette (Premium / Dark Mode) ---
var (
	ColorPrimary   = lipgloss.Color("#7D56F4") // Indigo/Purple
	ColorSecondary = lipgloss.Color("#04B575") // Green
	ColorError     = lipgloss.Color("#FF5F87") // Pink/Red
	ColorWarning   = lipgloss.Color("#FFAF00") // Gold
	ColorText      = lipgloss.Color("#FAFAFA") // White-ish
	ColorSubtle    = lipgloss.Color("#767676") // Gray
	ColorBorder    = lipgloss.Color("#3C3C3C") // Dark Gray border
	ColorBg        = lipgloss.Color("#1A1A1A") // Dark BG (often terminal default)
	ColorHighlight = lipgloss.Color("#3E3E3E") // Slightly lighter BG
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
