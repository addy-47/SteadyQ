package live

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

type Model struct {
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

func NewModel(totalDur time.Duration) Model {
	slRps := components.NewSparkline(
		40, 1,
		"RPS (Active)",
		styles.Active,
	)

	slLat := components.NewSparkline(
		40, 1,
		"Latency P90 (ms)",
		styles.Warn,
	)

	return Model{
		Progress:    progress.New(progress.WithDefaultGradient()),
		RpsLine:     slRps,
		LatencyLine: slLat,
		StartTime:   time.Now(),
		Duration:    totalDur,
		LastUpdate:  time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
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

func (m Model) View() string {
	s := strings.Builder{}

	// Top Grid: Metrics
	reqs := m.Stats.Requests
	inflight := m.Stats.Inflight
	errRate := 0.0
	if reqs > 0 {
		errRate = (float64(m.Stats.Fail) / float64(reqs)) * 100
	}

	var errColor lipgloss.Style
	if errRate > 5.0 {
		errColor = styles.Error
	} else if errRate > 1.0 {
		errColor = styles.Warn
	} else {
		errColor = styles.Active
	}

	col1 := fmt.Sprintf("REQ: %d\nINF: %d", reqs, inflight)
	col2 := fmt.Sprintf("ERR: %.2f%%\nFAIL: %d", errRate, m.Stats.Fail)

	qWait := m.Stats.AvgQueueWaitMs
	lagStyle := styles.Active
	if qWait > 2.0 {
		lagStyle = styles.Warn
	}
	if qWait > 10.0 {
		lagStyle = styles.Error
	}

	col3 := fmt.Sprintf(
		"LAG: %s\nBYTES: %d",
		lagStyle.Render(fmt.Sprintf("%.2f ms", qWait)),
		m.Stats.Bytes/1024,
	)

	grid := lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Box.Render(col1),
		styles.Box.Render(errColor.Render(col2)),
		styles.Box.Render(col3),
	)
	s.WriteString(grid)
	s.WriteString("\n\n")

	// Sparklines
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		styles.Box.Render(m.RpsLine.View()),
		styles.Box.Render(m.LatencyLine.View()),
	))
	s.WriteString("\n\n")

	// Detailed Latency
	latencies := fmt.Sprintf(
		"P50: %.2f ms  |  P90: %.2f ms  |  P99: %.2f ms  |  Max: %d ms",
		m.Stats.P50ServiceMs,
		m.Stats.P90ServiceMs,
		m.Stats.P99ServiceMs,
		m.Stats.MaxServiceMs,
	)
	s.WriteString(styles.Box.Width(m.Width - 4).Render(latencies))
	s.WriteString("\n\n")

	// Progress
	s.WriteString(m.Progress.View())

	return s.String()
}
