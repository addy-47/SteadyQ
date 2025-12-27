package tui

import (
	"fmt"
	"strings"
	"time"

	"steadyq/internal/runner"
	"steadyq/internal/stats"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tickInterval = 200 * time.Millisecond
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575")).MarginBottom(1)
	statStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	subtle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type tickMsg time.Time

type Model struct {
	Runner    *runner.Runner
	Stats     *stats.Stats
	Progress  progress.Model
	StartTime time.Time
	Duration  time.Duration
	Quitting  bool
	Width     int
	Height    int
}

func NewModel(r *runner.Runner, totalDur time.Duration) Model {
	return Model{
		Runner:    r,
		Stats:     r.Stats,
		Progress:  progress.New(progress.WithDefaultGradient()),
		StartTime: time.Now(),
		Duration:  totalDur,
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Progress.Width = msg.Width - 4
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.Quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		// Calculate Progress
		elapsed := time.Since(m.StartTime)
		pct := float64(elapsed) / float64(m.Duration)
		if pct > 1.0 {
			pct = 1.0
		}

		cmd := m.Progress.SetPercent(pct)

		if pct >= 1.0 && m.Runner.GetInflight() == 0 {
			m.Quitting = true
			return m, tea.Quit
		}

		return m, tea.Batch(cmd, tickCmd())

	case progress.FrameMsg:
		progressModel, cmd := m.Progress.Update(msg)
		m.Progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.Quitting {
		return "Safe Exit.\n"
	}

	s := strings.Builder{}

	// Header
	s.WriteString(titleStyle.Render("🚀 SteadyQ Performance Test"))
	s.WriteString("\n")

	// Config Summary
	cfg := m.Runner.Cfg
	if cfg.Mode == "users" {
		s.WriteString(fmt.Sprintf("Mode: %s | Users: %d | ThinkTime: %s\n", cfg.Mode, cfg.NumUsers, cfg.ThinkTime))
	} else {
		s.WriteString(fmt.Sprintf("Mode: %s | Target RPS: %d\n", cfg.Mode, cfg.TargetRPS))
	}
	s.WriteString(fmt.Sprintf("URL: %s\n", cfg.URL))
	s.WriteString(subtle.Render(fmt.Sprintf("Duration: %s (Elapsed: %s)", m.Duration, time.Since(m.StartTime).Round(time.Second))))
	s.WriteString("\n\n")

	// Stats Grid
	reqs := m.Stats.Requests
	errRate := m.Stats.ErrorRate()

	inflight := m.Runner.GetInflight()

	// Queue Lag Warning
	qWaitMs := m.Stats.QueueWaitAvgMs()
	lagStatus := "OK"
	if qWaitMs > 1.0 {
		lagStatus = warnStyle.Render(fmt.Sprintf("WARNING (%.2fms)", qWaitMs))
	} else if qWaitMs > 10.0 {
		lagStatus = errStyle.Render(fmt.Sprintf("CRITICAL (%.2fms)", qWaitMs))
	}

	// Layout with columns
	leftCol := fmt.Sprintf(
		"Requests: %d\nInflight: %d\nErrors:   %.2f%%\nLag:      %s",
		reqs, inflight, errRate, lagStatus,
	)

	// Histograms
	p50 := m.Stats.ServiceTime.ValueAtQuantile(50) / 1000
	p90 := m.Stats.ServiceTime.ValueAtQuantile(90) / 1000
	p99 := m.Stats.ServiceTime.ValueAtQuantile(99) / 1000
	max := m.Stats.ServiceTime.Max() / 1000

	rightCol := fmt.Sprintf(
		"Latency (Service)\n  P50: %d ms\n  P90: %d ms\n  P99: %d ms\n  Max: %d ms",
		p50, p90, p99, max,
	)

	// Join columns
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(30).Render(leftCol),
		lipgloss.NewStyle().Width(30).Render(rightCol),
	))

	s.WriteString("\n\n")
	s.WriteString(m.Progress.View())
	s.WriteString("\n")
	s.WriteString(subtle.Render("Press q to quit"))

	return s.String()
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
