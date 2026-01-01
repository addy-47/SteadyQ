package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"steadyq/internal/dummy"
	"steadyq/internal/runner"
	"steadyq/internal/tui/app"
)

func main() {
	// 0. Check for Subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "dummy":
			// Parse flags manually for subcommand since we are not using a CLI lib
			port := 8080
			args := os.Args[2:]
			for i := 0; i < len(args); i++ {
				if args[i] == "-port" || args[i] == "--port" {
					if i+1 < len(args) {
						fmt.Sscanf(args[i+1], "%d", &port)
						i++ // Skip value
					}
				}
			}
			dummy.Start(dummy.ServerConfig{Port: port})
			// Block forever
			select {}
		case "help", "--help", "-h":
			fmt.Println("Usage: steadyq [dummy] [-port 8080]")
			return
		}
	}
	// 1. Initialize dependencies
	// History removed as per request

	// 2. Setup Default Runner (Idle)
	defaultCfg := runner.Config{
		TargetRPS: 10,
		SteadyDur: 10, // Default 10s
		Mode:      "rps",
		URL:       "http://localhost:8080/fast",
	}
	updates := make(runner.StatsUpdateChan, 100)
	run := runner.NewRunner(defaultCfg, updates)

	// 3. Launch TUI Application
	m := app.NewModel(run, updates)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running SteadyQ: %v\n", err)
		os.Exit(1)
	}
}
