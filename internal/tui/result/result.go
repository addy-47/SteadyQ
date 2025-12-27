package result

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"steadyq/internal/stats"
	"steadyq/internal/tui/styles"
)

type Model struct {
	Stats *stats.Stats

	Width  int
	Height int
}

func NewModel(s *stats.Stats) Model {
	return Model{Stats: s}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	s := strings.Builder{}

	// Create a summary table style

	reqs := m.Stats.Requests
	success := m.Stats.Success
	fail := m.Stats.Fail

	avg := m.Stats.ServiceTime.Mean() / 1000.0
	p50 := float64(m.Stats.ServiceTime.ValueAtQuantile(50)) / 1000.0
	p90 := float64(m.Stats.ServiceTime.ValueAtQuantile(90)) / 1000.0
	p99 := float64(m.Stats.ServiceTime.ValueAtQuantile(99)) / 1000.0
	max := float64(m.Stats.ServiceTime.Max()) / 1000.0

	s.WriteString(styles.Title.Render("📊 Test Complete"))
	s.WriteString("\n\n")

	// 1. Overview
	s.WriteString(styles.Active.Render("Overview"))
	s.WriteString("\n")

	overview := fmt.Sprintf(
		"Total Requests: %d\nSuccess:        %d\nFailed:         %d\nTotal Bytes:    %d",
		reqs, success, fail, m.Stats.Bytes,
	)
	s.WriteString(styles.Box.Render(overview))
	s.WriteString("\n\n")

	// 2. Latency
	s.WriteString(styles.Active.Render("Latency (Service Time)"))
	s.WriteString("\n")

	latency := fmt.Sprintf(
		"Avg: %.2f ms\nP50: %.2f ms\nP90: %.2f ms\nP99: %.2f ms\nMax: %.2f ms",
		avg, p50, p90, p99, max,
	)
	s.WriteString(styles.Box.Render(latency))

	s.WriteString("\n\n")
	s.WriteString(styles.Subtle.Render("Press q to quit"))

	return s.String()
}
