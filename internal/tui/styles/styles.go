package styles

import "github.com/charmbracelet/lipgloss"

// Color Palette (Cyberpunk / Neon)
var (
	ColorPrimary   = lipgloss.Color("#00F0FF") // Cyan
	ColorSecondary = lipgloss.Color("#7D3C98") // Purple
	ColorSuccess   = lipgloss.Color("#00FF41") // Matrix Green
	ColorError     = lipgloss.Color("#FF3131") // Neon Red
	ColorWarn      = lipgloss.Color("#FFD700") // Gold
	ColorText      = lipgloss.Color("#E0E0E0") // Off-white
	ColorSub       = lipgloss.Color("#6E6E6E") // Grey
	ColorBorder    = lipgloss.Color("#333333") // Dark Grey
	ColorBg        = lipgloss.Color("#0D0D0D") // Black
)

var (
	// Standard Text
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1).
		MarginBottom(1)

	Subtle = lipgloss.NewStyle().
		Foreground(ColorSub)

	Active = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	// Navigation Sidebar
	MenuItem = lipgloss.NewStyle().
			Foreground(ColorSub).
			Padding(0, 1)

	MenuItemActive = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(ColorPrimary).
			Foreground(ColorText).
			Bold(true).
			Padding(0, 1)

	// Panels
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1)

	PanelActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1)

	// Buttons
	Button = lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorSub).
		Padding(0, 3).
		MarginTop(1)

	ButtonActive = lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(ColorPrimary).
			Bold(true).
			Padding(0, 3).
			MarginTop(1)

	// Footer
	Footer = lipgloss.NewStyle().
		Foreground(ColorSub).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	// Alerts
	Error = lipgloss.NewStyle().Foreground(ColorError)
	Warn  = lipgloss.NewStyle().Foreground(ColorWarn)
)
