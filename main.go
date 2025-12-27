package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"steadyq/internal/runner"
	"steadyq/internal/storage"
	"steadyq/internal/tui/app"
)

func main() {
	// 1. Initialize dependencies
	store, err := storage.NewStore()
	if err != nil {
		fmt.Printf("Fatal: Could not load persistence: %v\n", err)
		os.Exit(1)
	}

	// 2. Setup Default Runner (Idle)
	defaultCfg := runner.Config{
		TargetRPS: 10,
		SteadyDur: 60,
		Mode:      "rps",
	}
	updates := make(runner.StatsUpdateChan, 100)
	run := runner.NewRunner(defaultCfg, updates)

	// 3. Launch TUI Application
	m := app.NewModel(run, updates, store)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running SteadyQ: %v\n", err)
		os.Exit(1)
	}
}
