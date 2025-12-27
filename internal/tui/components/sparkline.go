package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var levels = []string{" ", " ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

type Sparkline struct {
	Data   []uint64
	Width  int
	Height int
	Max    uint64
	Style  lipgloss.Style
	Label  string
}

func NewSparkline(width, height int, label string, style lipgloss.Style) Sparkline {
	return Sparkline{
		Width:  width,
		Height: height,
		Label:  label,
		Style:  style,
		Data:   make([]uint64, 0, width),
	}
}

func (s *Sparkline) Add(val uint64) {
	s.Data = append(s.Data, val)
	if len(s.Data) > s.Width {
		s.Data = s.Data[len(s.Data)-s.Width:]
	}

	// Update global max or window max?
	// For scrolling window, we usually want max of visible window.
	max := uint64(0)
	for _, v := range s.Data {
		if v > max {
			max = v
		}
	}
	s.Max = max
}

func (s Sparkline) View() string {
	if s.Width <= 0 {
		return ""
	}

	// Render label
	out := strings.Builder{}
	out.WriteString(s.Style.Render(s.Label))
	out.WriteString("\n")

	// Render graph
	// Simple 1-line implementation for now to save space
	// Map 0..Max to levels

	var graph strings.Builder
	for _, v := range s.Data {
		if s.Max == 0 {
			graph.WriteString(levels[0])
			continue
		}

		pct := float64(v) / float64(s.Max)
		idx := int(pct * float64(len(levels)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(levels) {
			idx = len(levels) - 1
		}

		graph.WriteString(levels[idx])
	}

	// Pad if not full
	pad := s.Width - len(s.Data)
	if pad > 0 {
		graph.WriteString(strings.Repeat(" ", pad))
	}

	return out.String() + s.Style.Render(graph.String())
}
