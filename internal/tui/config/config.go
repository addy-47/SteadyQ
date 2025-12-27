package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/tui/styles"
)

type Field struct {
	Label string
	Input textinput.Model
}

type Model struct {
	Config runner.Config

	Fields []Field
	Focus  int

	Width  int
	Height int
}

func NewModel(cfg runner.Config) Model {
	m := Model{
		Config: cfg,
		Fields: make([]Field, 4),
	}

	// 0: URL
	t0 := textinput.New()
	t0.Placeholder = "http://localhost:8080"
	t0.SetValue(cfg.URL)
	t0.Focus()
	t0.Width = 50
	m.Fields[0] = Field{Label: "Target URL", Input: t0}

	// 1: RPS
	t1 := textinput.New()
	t1.Placeholder = "10"
	t1.SetValue(fmt.Sprintf("%d", cfg.TargetRPS))
	t1.Width = 10
	m.Fields[1] = Field{Label: "Target RPS", Input: t1}

	// 2: Duration
	t2 := textinput.New()
	t2.Placeholder = "60"
	t2.SetValue(fmt.Sprintf("%d", cfg.SteadyDur))
	t2.Width = 10
	m.Fields[2] = Field{Label: "Duration (s)", Input: t2}

	// 3: Mode
	t3 := textinput.New()
	t3.Placeholder = "rps"
	t3.SetValue(cfg.Mode)
	t3.Width = 10
	m.Fields[3] = Field{Label: "Mode (users/rps)", Input: t3}

	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			if s == "enter" && m.Focus == len(m.Fields)-1 {
				// handled by parent
			}

			if s == "up" || s == "shift+tab" {
				m.Focus--
			} else {
				m.Focus++
			}

			if m.Focus > len(m.Fields)-1 {
				m.Focus = 0
			} else if m.Focus < 0 {
				m.Focus = len(m.Fields) - 1
			}

			for i := 0; i <= len(m.Fields)-1; i++ {
				if i == m.Focus {
					m.Fields[i].Input.Focus()
					m.Fields[i].Input.PromptStyle = styles.Active
					m.Fields[i].Input.TextStyle = styles.Active
				} else {
					m.Fields[i].Input.Blur()
					m.Fields[i].Input.PromptStyle = lipgloss.NewStyle()
					m.Fields[i].Input.TextStyle = lipgloss.NewStyle()
				}
			}
			return m, nil
		}
	}

	// Update inputs
	for i := range m.Fields {
		m.Fields[i].Input, cmd = m.Fields[i].Input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) GetConfig() runner.Config {
	c := m.Config
	c.URL = m.Fields[0].Input.Value()

	rps, _ := strconv.Atoi(m.Fields[1].Input.Value())
	c.TargetRPS = rps

	dur, _ := strconv.Atoi(m.Fields[2].Input.Value())
	c.SteadyDur = dur

	c.Mode = m.Fields[3].Input.Value()

	return c
}

func (m Model) View() string {
	s := strings.Builder{}

	s.WriteString(styles.Title.Render("🛠️  Configuration"))
	s.WriteString("\n\n")

	for i := range m.Fields {
		s.WriteString(styles.Subtle.Render(m.Fields[i].Label))
		s.WriteString("\n")
		s.WriteString(m.Fields[i].Input.View())
		s.WriteString("\n\n")
	}

	s.WriteString("\n")
	s.WriteString(styles.Active.Render("[Enter] Start Test"))

	return styles.Box.Render(s.String())
}
