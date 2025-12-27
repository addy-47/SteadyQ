package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"steadyq/internal/runner"
	"steadyq/internal/stats"
	"steadyq/internal/storage"
	"steadyq/internal/tui/config"
	"steadyq/internal/tui/history"
	"steadyq/internal/tui/live"
	"steadyq/internal/tui/result"
	"steadyq/internal/tui/styles"
)

type State int

const (
	StateRunning State = iota
	StateResult
	StateHistory
	StateConfig
)

type StatsMsg runner.StatsSnapshot

type Model struct {
	Runner  *runner.Runner
	Stats   *stats.Stats
	Updates runner.StatsUpdateChan
	Store   *storage.Store

	State State

	ConfigView  config.Model
	LiveView    live.Model
	ResultView  result.Model
	HistoryView history.Model

	Quitting bool
	Width    int
	Height   int
}

func NewModel(r *runner.Runner, updates runner.StatsUpdateChan, store *storage.Store, totalDur time.Duration, interactive bool) Model {
	initialState := StateRunning
	if interactive {
		initialState = StateConfig
	}

	return Model{
		Runner:      r,
		Stats:       r.Stats,
		Updates:     updates,
		Store:       store,
		State:       initialState,
		ConfigView:  config.NewModel(r.Cfg),
		LiveView:    live.NewModel(totalDur),
		HistoryView: history.NewModel(store),
	}
}

func (m Model) Init() tea.Cmd {
	if m.State == StateConfig {
		return m.ConfigView.Init()
	}
	return tea.Batch(
		waitForUpdate(m.Updates),
		m.LiveView.Init(),
	)
}

func waitForUpdate(sub runner.StatsUpdateChan) tea.Cmd {
	return func() tea.Msg {
		return StatsMsg(<-sub)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.LiveView.Width = msg.Width
		m.LiveView.Height = msg.Height
		m.ResultView.Width = msg.Width
		m.ResultView.Height = msg.Height
		m.ConfigView.Width = msg.Width
		m.HistoryView.Update(msg)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" { // Quit anywhere on Ctrl+C // Removed q for Config/Running sometimes?
			m.Quitting = true
			return m, tea.Quit
		}

		if m.State != StateConfig && msg.String() == "q" {
			m.Quitting = true
			return m, tea.Quit
		}

		// Global Navigation (if not in config/running?)
		if m.State == StateResult || m.State == StateHistory {
			if msg.String() == "h" {
				m.State = StateHistory
				m.HistoryView.Refresh()
				return m, nil
			}
			if msg.String() == "esc" {
				// Go back
				if m.State == StateHistory {
					m.State = StateResult
				}
			}
		}

		// Config Logic
		if m.State == StateConfig {
			if msg.String() == "enter" {
				// START TEST
				newCfg := m.ConfigView.GetConfig()

				// Update Runner
				// We assume runner hasn't started yet.
				// We need to trigger the runner start.
				// For now, we update config and assume main.go isn't running it?
				// But Wait, main.go started `go run.Run(ctx)`.
				// If we want to support this, we need `runner` to wait for a start signal or update config safely.
				// Simplest way: The Runner logic in `runRPS` reads config ONCE at start.
				// So if it's already running, it's too late.
				// Hack: We can restart it? No.
				// Correct way: `main.go` shouldn't start it.
				// TUI should start it.

				m.Runner.Cfg = newCfg
				m.State = StateRunning

				// Re-init LiveView with new duration
				totalDur := time.Duration(newCfg.RampUp+newCfg.SteadyDur+newCfg.RampDown) * time.Second
				m.LiveView = live.NewModel(totalDur)
				m.LiveView.Width = m.Width
				m.LiveView.Height = m.Height

				// Start Runner via Cmd?
				// m.Runner.Start(context.Background()) // If we added Start method.

				// Since we didn't add Start method yet, and main.go already running...
				// This is a problem.
				// I'll update main.go to NOT start runner if interactive.
				// And I'll add a Start() method to Runner or expose Run in a goroutine here.

				go m.Runner.Run(context.TODO()) // Context management is tricky here.

				return m, tea.Batch(
					m.ConfigView.Init(), // Blink?
					waitForUpdate(m.Updates),
					m.LiveView.Init(),
				)
			}

			// Update Config View
			var cmd tea.Cmd
			m.ConfigView, cmd = m.ConfigView.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

	case StatsMsg:
		snap := runner.StatsSnapshot(msg)

		if m.State == StateRunning {
			updatedLive, c := m.LiveView.Update(snap)
			m.LiveView = updatedLive
			cmds = append(cmds, c)

			if m.LiveView.Progress.Percent() >= 1.0 && snap.Inflight == 0 {
				m.State = StateResult
				m.ResultView = result.NewModel(m.Stats)
				m.saveHistory()
				return m, nil
			}

			cmds = append(cmds, waitForUpdate(m.Updates))
		}

	default:
		// Propagate
		switch m.State {
		case StateRunning:
			updatedLive, c := m.LiveView.Update(msg)
			m.LiveView = updatedLive
			cmds = append(cmds, c)
		case StateResult:
			updatedRes, c := m.ResultView.Update(msg)
			m.ResultView = updatedRes
			cmds = append(cmds, c)
		case StateHistory:
			updatedHist, c := m.HistoryView.Update(msg)
			m.HistoryView = updatedHist
			cmds = append(cmds, c)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) saveHistory() {
	if m.Store == nil {
		return
	}

	item := storage.HistoryItem{
		ID:        fmt.Sprintf("%d", time.Now().Unix()),
		Timestamp: time.Now(),
		Config:    m.Runner.Cfg,
		Summary: storage.RunSummary{
			TotalRequests: m.Stats.Requests,
			Success:       m.Stats.Success,
			Fail:          m.Stats.Fail,
			AvgLatencyMs:  m.Stats.ServiceTime.Mean() / 1000.0,
			P99LatencyMs:  m.Stats.GetP99Service(),
		},
	}
	m.Store.Save(item)
}

func (m Model) View() string {
	if m.Quitting {
		return "Safe Exit.\n"
	}

	s := strings.Builder{}

	// Header
	s.WriteString(styles.Title.Render("🚀 SteadyQ Performance Test"))
	s.WriteString("\n")

	switch m.State {
	case StateConfig:
		s.WriteString(m.ConfigView.View())
	case StateRunning:
		s.WriteString(m.LiveView.View())
	case StateResult:
		s.WriteString(m.ResultView.View())
	case StateHistory:
		s.WriteString(m.HistoryView.View())
	}

	return s.String()
}
