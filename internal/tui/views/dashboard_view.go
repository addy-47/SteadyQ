package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/tui/components"
	"steadyq/internal/tui/styles"
)

type DashboardView struct {
	Stats    runner.StatsSnapshot
	Progress progress.Model

	RpsLine     components.Sparkline
	LatencyLine components.Sparkline

	StartTime  time.Time
	Duration   time.Duration
	LastUpdate time.Time
	LastReqs   uint64

	Width  int
	Height int
}

func NewDashboardView(totalDur time.Duration) DashboardView {
	slRps := components.NewSparkline(
		40, 1,
		"RPS (Active)",
		styles.Active,
	)

	slLat := components.NewSparkline(
		40, 1,
		"Latency P90 (ms)",
		styles.Warn, // Gold for Latency
	)

	return DashboardView{
		Progress:    progress.New(progress.WithDefaultGradient()),
		RpsLine:     slRps,
		LatencyLine: slLat,
		StartTime:   time.Now(),
		Duration:    totalDur,
		LastUpdate:  time.Now(),
	}
}

func (m DashboardView) Init() tea.Cmd {
	return nil
}

func (m DashboardView) Update(msg tea.Msg) (DashboardView, tea.Cmd) {
	switch msg := msg.(type) {
	case runner.StatsSnapshot:
		now := time.Now()
		dt := now.Sub(m.LastUpdate).Seconds()
		if dt < 0.01 {
			dt = 0.01
		}

		// 1. Calculate RPS
		deltaReqs := msg.Requests - m.LastReqs
		rps := float64(deltaReqs) / dt

		// 2. Update Sparklines
		m.RpsLine.Add(uint64(rps))
		m.LatencyLine.Add(uint64(msg.P90ServiceMs))

		// 3. Update State
		m.Stats = msg
		m.LastReqs = msg.Requests
		m.LastUpdate = now

		// 4. Update Progress
		// Calculate elapsed based on actual start time, or approximate?
		// We'll reset StartTime when dashboard is "Started" properly?
		// For now assume it flows.
		elapsed := time.Since(m.StartTime)
		pct := float64(elapsed) / float64(m.Duration)
		if pct > 1.0 {
			pct = 1.0
		}
		cmd := m.Progress.SetPercent(pct)
		return m, cmd

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Progress.Width = msg.Width - 4

		half := (msg.Width / 2) - 4
		if half < 10 {
			half = 10
		}
		m.RpsLine.Width = half
		m.LatencyLine.Width = half
		return m, nil

	case progress.FrameMsg:
		prog, cmd := m.Progress.Update(msg)
		m.Progress = prog.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m DashboardView) View() string {
	s := strings.Builder{}

	s.WriteString(styles.Title.Render("📊 Live Dashboard"))
	s.WriteString("\n\n")

	// Top Grid: Metrics
	// We want big boxes.

	reqs := m.Stats.Requests
	inflight := m.Stats.Inflight
	errRate := 0.0
	if reqs > 0 {
		errRate = (float64(m.Stats.Fail) / float64(reqs)) * 100
	}

	// Styles
	styleErr := styles.Active
	if errRate > 1.0 {
		styleErr = styles.Warn
	}
	if errRate > 5.0 {
		styleErr = styles.Error
	}

	renderMetric := func(label, value string, style lipgloss.Style) string {
		return styles.Box.Render(
			fmt.Sprintf("%s\n%s", styles.Subtle.Render(label), style.Render(value)),
		)
	}

	// Row 1
	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		renderMetric("Requests", fmt.Sprintf("%d", reqs), styles.Active),
		renderMetric("Inflight", fmt.Sprintf("%d", inflight), styles.Active),
		renderMetric("Errors", fmt.Sprintf("%.2f%% (%d)", errRate, m.Stats.Fail), styleErr),
	)
	s.WriteString(row1)
	s.WriteString("\n")

	// Row 2: Latency
	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		renderMetric("P50 Latency", fmt.Sprintf("%.2f ms", m.Stats.P50ServiceMs), styles.Active),
		renderMetric("P90 Latency", fmt.Sprintf("%.2f ms", m.Stats.P90ServiceMs), styles.Warn),
		renderMetric("P99 Latency", fmt.Sprintf("%.2f ms", m.Stats.P99ServiceMs), styles.Warn),
	)
	s.WriteString(row2)
	s.WriteString("\n\n")

	// Sparklines
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Box.Render(m.RpsLine.View()),
		styles.Box.Render(m.LatencyLine.View()),
	))
	s.WriteString("\n\n")

	// Progress
	s.WriteString(m.Progress.View())

	return s.String()
}
