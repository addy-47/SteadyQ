package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/tui/styles"
)

type DashboardView struct {
	Stats    runner.StatsSnapshot
	Viewport viewport.Model
	Progress progress.Model
	Config   runner.Config

	StartTime  time.Time
	Duration   time.Duration
	LastUpdate time.Time

	Width  int
	Height int
}

func NewDashboardView(cfg runner.Config, width, height int) DashboardView {
	totalDur := time.Duration(cfg.RampUp+cfg.SteadyDur+cfg.RampDown) * time.Second

	// Gradient Progress Bar
	prog := progress.New(
		progress.WithGradient("#7D56F4", "#04B575"),
		progress.WithWidth(width-10),
		progress.WithoutPercentage(),
	)

	vp := viewport.New(width-6, height-8)

	return DashboardView{
		Viewport:   vp,
		Progress:   prog,
		Config:     cfg,
		StartTime:  time.Now(),
		Duration:   totalDur,
		LastUpdate: time.Now(),
		Width:      width,
		Height:     height,
	}
}

func (m DashboardView) Init() tea.Cmd {
	return nil // Progress might need tick? Usually handled by Update frame
}

func (m DashboardView) Update(msg tea.Msg) (DashboardView, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case runner.StatsSnapshot:
		m.LastUpdate = time.Now()
		m.Stats = msg

		var elapsed time.Duration
		if !m.StartTime.IsZero() {
			elapsed = time.Since(m.StartTime)
		}

		pct := 0.0
		if m.Duration > 0 {
			pct = float64(elapsed) / float64(m.Duration)
		}

		if pct > 1.0 {
			pct = 1.0
		}
		cmds = append(cmds, m.Progress.SetPercent(pct))

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Progress.Width = msg.Width - 10
		m.Viewport.Width = msg.Width - 6
		m.Viewport.Height = msg.Height - 8

	case progress.FrameMsg:
		newModel, cmd := m.Progress.Update(msg)
		if newModel, ok := newModel.(progress.Model); ok {
			m.Progress = newModel
		}
		cmds = append(cmds, cmd)
	}

	m.Viewport, cmd = m.Viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m DashboardView) View() string {
	s := strings.Builder{}

	// --- Header ---
	var elapsed time.Duration
	if !m.StartTime.IsZero() {
		elapsed = time.Since(m.StartTime)
	}
	remaining := m.Duration - elapsed
	if remaining < 0 {
		remaining = 0
	}

	// Determine Phase
	phase := "Steady State"
	if m.Duration > 0 { // Only calculate phase if duration is set
		rupEnd := time.Duration(m.Config.RampUp) * time.Second
		steadyEnd := rupEnd + time.Duration(m.Config.SteadyDur)*time.Second

		if elapsed < rupEnd {
			phase = "Ramp Up"
		} else if elapsed > steadyEnd {
			phase = "Ramp Down"
		}
	}

	timer := fmt.Sprintf("%s / %s", elapsed.Round(time.Second), remaining.Round(time.Second))
	header := lipgloss.JoinHorizontal(lipgloss.Center,
		styles.Title.Render("⚡ Testing in Progress"),
		lipgloss.NewStyle().MarginLeft(2).Foreground(styles.ColorSubtle).Render(timer),
		lipgloss.NewStyle().MarginLeft(4).Foreground(styles.ColorPrimary).Bold(true).Render("["+phase+"]"),
	)
	s.WriteString(header)
	s.WriteString("\n\n")

	// --- Progress ---
	s.WriteString(m.Progress.View())
	s.WriteString("\n\n")

	// --- Metrics Grid ---
	// Row 1: Volume
	reqsVal := styles.Value.Render(fmt.Sprintf("%d", m.Stats.Requests))
	rps := 0.0
	if elapsed.Seconds() > 0 {
		rps = float64(m.Stats.Requests) / elapsed.Seconds()
	}
	rpsVal := styles.Value.Render(fmt.Sprintf("%.1f", rps))
	inflightVal := styles.Active.Render(fmt.Sprintf("%d", m.Stats.Inflight))

	// Target display
	targetStr := fmt.Sprintf("%d RPS", m.Config.TargetRPS)
	if m.Config.Mode == "users" {
		targetStr = fmt.Sprintf("%d Users", m.Config.NumUsers)
	}
	targetVal := styles.Subtle.Render(targetStr)

	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		MakeCard("Requests", reqsVal),
		MakeCard("Avg RPS", rpsVal),
		MakeCard("Inflight", inflightVal),
		MakeCard("Target", targetVal),
	)
	s.WriteString(row1)
	s.WriteString("\n")

	// Row 2: Latency Percentiles
	p50Val := styles.Text.Render(fmt.Sprintf("%.1f ms", m.Stats.P50ServiceMs))
	p90Val := styles.Text.Render(fmt.Sprintf("%.1f ms", m.Stats.P90ServiceMs))
	p95Val := styles.Warn.Render(fmt.Sprintf("%.1f ms", m.Stats.P95ServiceMs))
	p99Val := styles.Error.Render(fmt.Sprintf("%.1f ms", m.Stats.P99ServiceMs))

	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		MakeCard("P50 Latency", p50Val),
		MakeCard("P90 Latency", p90Val),
		MakeCard("P95 Latency", p95Val),
		MakeCard("P99 Latency", p99Val),
	)
	s.WriteString(row2)
	s.WriteString("\n")

	// Row 3: Others
	meanVal := styles.Text.Render(fmt.Sprintf("%.1f ms", m.Stats.MeanServiceMs))
	maxVal := styles.Text.Render(fmt.Sprintf("%d ms", m.Stats.MaxServiceMs))

	errColor := styles.Text
	if m.Stats.Fail > 0 {
		errColor = styles.Error
	}
	failVal := errColor.Render(fmt.Sprintf("%d", m.Stats.Fail))

	row3 := lipgloss.JoinHorizontal(lipgloss.Top,
		MakeCard("Mean Latency", meanVal),
		MakeCard("Max Latency", maxVal),
		MakeCard("Errors", failVal),
	)
	s.WriteString(row3)
	s.WriteString("\n\n")

	// --- Response Codes ---
	if len(m.Stats.StatusCodes) > 0 {
		s.WriteString(styles.Subtle.Render("Response Breakdown"))
		s.WriteString("\n")

		var codes []int
		for k := range m.Stats.StatusCodes {
			codes = append(codes, k)
		}
		sort.Ints(codes)

		barWidth := 30
		maxCount := 0
		for _, c := range m.Stats.StatusCodes {
			if c > maxCount {
				maxCount = c
			}
		}

		for _, c := range codes {
			count := m.Stats.StatusCodes[c]
			// Simple bar
			w := 0
			if maxCount > 0 {
				w = int((float64(count) / float64(maxCount)) * float64(barWidth))
			}
			bar := strings.Repeat("█", w)

			// Formatting
			codeStr := fmt.Sprintf("%d", c)
			if c == 0 {
				codeStr = "ERR"
			}

			color := styles.Value
			if c == 0 || c >= 500 {
				color = styles.Error
			} else if c >= 400 {
				color = styles.Warn
			}

			line := fmt.Sprintf("%3s : %s %d", codeStr, color.Render(bar), count)
			s.WriteString(line + "\n")
		}
	}

	// --- Error Detail Breakdown ---
	if len(m.Stats.ErrorCounts) > 0 {
		s.WriteString("\n")
		s.WriteString(styles.Subtle.Render("Error Details"))
		s.WriteString("\n")

		// Sort keys
		var errs []string
		for k := range m.Stats.ErrorCounts {
			errs = append(errs, k)
		}
		sort.Strings(errs)

		for _, e := range errs {
			count := m.Stats.ErrorCounts[e]
			// Truncate error if too long
			dispErr := e
			if len(dispErr) > 60 {
				dispErr = dispErr[:57] + "..."
			}
			s.WriteString(fmt.Sprintf("%s %s\n", styles.Error.Render(fmt.Sprintf("%d x", count)), dispErr))
		}
	}

	// --- Response Samples ---
	if len(m.Stats.ResponseSamples) > 0 {
		s.WriteString("\n")
		s.WriteString(styles.Subtle.Render("Sample Response Body (>=400)"))
		s.WriteString("\n")

		var codes []int
		for k := range m.Stats.ResponseSamples {
			codes = append(codes, k)
		}
		sort.Ints(codes)

		for _, c := range codes {
			sample := m.Stats.ResponseSamples[c]
			// Clean up sample (newlines, etc)
			sample = strings.ReplaceAll(sample, "\n", " ")
			sample = strings.ReplaceAll(sample, "\r", "")
			if len(sample) > 80 {
				sample = sample[:77] + "..."
			}
			s.WriteString(fmt.Sprintf("%s: %s\n", styles.Warn.Render(fmt.Sprintf("[%d]", c)), sample))
		}
	}

	content := styles.Panel.Width(m.Width - 6).Render(s.String())
	m.Viewport.SetContent(content)

	return m.Viewport.View()
}

func MakeCard(title, value string) string {
	return styles.Box.Width(18).Align(lipgloss.Center).Render(
		fmt.Sprintf("%s\n%s", styles.Subtle.Render(title), value),
	)
}
