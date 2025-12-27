package views

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

type RunnerView struct {
	Config runner.Config
	Inputs []textinput.Model
	Focus  int

	// Layout
	Width  int
	Height int
}

func NewRunnerView(defaultCfg runner.Config) RunnerView {
	m := RunnerView{
		Config: defaultCfg,
		Inputs: make([]textinput.Model, 4),
	}

	// 0: URL
	t0 := textinput.New()
	t0.Placeholder = "http://localhost:8080"
	t0.SetValue(defaultCfg.URL)
	t0.Focus()
	t0.Width = 40
	m.Inputs[0] = t0

	// 1: Capacity (RPS or Users)
	t1 := textinput.New()
	t1.Placeholder = "10"
	t1.SetValue(fmt.Sprintf("%d", defaultCfg.TargetRPS))
	t1.Width = 10
	m.Inputs[1] = t1

	// 2: Duration
	t2 := textinput.New()
	t2.Placeholder = "60"
	t2.SetValue(fmt.Sprintf("%d", defaultCfg.SteadyDur))
	t2.Width = 10
	m.Inputs[2] = t2

	// 3: Mode
	t3 := textinput.New()
	t3.Placeholder = "rps"
	t3.SetValue(defaultCfg.Mode)
	t3.Width = 10
	m.Inputs[3] = t3

	return m
}

func (m RunnerView) Init() tea.Cmd {
	return textinput.Blink
}

func (m RunnerView) Update(msg tea.Msg) (RunnerView, tea.Cmd) {
	var cmds []tea.Cmd = make([]tea.Cmd, len(m.Inputs))

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Navigation
			if s == "up" || s == "shift+tab" {
				m.Focus--
			} else {
				m.Focus++
			}

			if m.Focus > len(m.Inputs)-1 {
				m.Focus = 0
			} else if m.Focus < 0 {
				m.Focus = len(m.Inputs) - 1
			}

			// Update Focus State
			for i := 0; i < len(m.Inputs); i++ {
				if i == m.Focus {
					cmds[i] = m.Inputs[i].Focus()
					m.Inputs[i].PromptStyle = styles.Active
					m.Inputs[i].TextStyle = styles.Active
				} else {
					m.Inputs[i].Blur()
					m.Inputs[i].PromptStyle = lipgloss.NewStyle()
					m.Inputs[i].TextStyle = lipgloss.NewStyle()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}

	// Update individual inputs
	for i := range m.Inputs {
		var cmd tea.Cmd
		m.Inputs[i], cmd = m.Inputs[i].Update(msg)
		cmds[i] = cmd
	}

	return m, tea.Batch(cmds...)
}

func (m RunnerView) GetConfig() runner.Config {
	c := m.Config
	c.URL = m.Inputs[0].Value()

	capVal, _ := strconv.Atoi(m.Inputs[1].Value())
	if c.Mode == "users" {
		c.NumUsers = capVal
	} else {
		c.TargetRPS = capVal
	}

	dur, _ := strconv.Atoi(m.Inputs[2].Value())
	c.SteadyDur = dur

	c.Mode = m.Inputs[3].Value() // Simple text for now
	return c
}

func (m RunnerView) View() string {
	s := strings.Builder{}

	// Form Container
	s.WriteString(styles.Title.Render("🚀 New Load Test"))
	s.WriteString("\n\n")

	// Field Helper
	renderField := func(label string, input textinput.Model) {
		s.WriteString(styles.Subtle.Render(label))
		s.WriteString("\n")
		s.WriteString(input.View())
		s.WriteString("\n\n")
	}

	renderField("Target URL", m.Inputs[0])
	renderField("Throughput (RPS) / Users", m.Inputs[1]) // Hybrid label for now
	renderField("Duration (s)", m.Inputs[2])
	renderField("Mode (rps/users)", m.Inputs[3])

	s.WriteString("\n")
	s.WriteString(styles.ButtonActive.Render("[ ENTER ] Start Test")) // Need to define ButtonActive

	return s.String()
}
