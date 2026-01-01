package banner

import (
	"steadyq/internal/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

func GetString() string {
	renderer := lipgloss.DefaultRenderer()

	style := renderer.NewStyle().
		Foreground(styles.ColorBanner).
		Bold(true)

	ascii := `
   _____ __                 __      ____ 
  / ___// /____  ____ _____/ /_  __/ __ \
  \__ \/ __/ _ \/ __ '/ __  / / / / / / /
 ___/ / /_/  __/ /_/ / /_/ / /_/ / /_/ / 
/____/\__/\___/\__,_/\__,_/\__, /\___\_\ 
                          /____/         `

	return "\n" + style.Render(ascii) + "\n"
}
