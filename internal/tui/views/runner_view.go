package views

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/tui/styles"
)

type RunnerView struct {
	Inputs  []textinput.Model
	Focus   int
	Editing bool // Always true

	Width  int
	Height int
}

// Add GetHelp method
func (m RunnerView) GetHelp() string {
	switch m.Focus {
	case FieldReqType:
		return "Request Type determines how load is generated.\n• [HTTP]: Standard HTTP/1.1 requests.\n• [Script]: Execute a local shell command for every request.\n\nPress [Space] to toggle."
	case FieldURL:
		return "The absolute URL where requests will be sent.\nExample: http://localhost:8080/api/v1/health"
	case FieldMethod:
		return "The HTTP Method to use.\nSupported: GET, POST, PUT, DELETE, PATCH, HEAD."
	case FieldCommand:
		return "The Shell Command to execute for each 'request'.\n\nTemplate Variables:\n• {{userID}}: Unique UUID for the simulated user.\n• {{chatID}}: Unique UUID for the request context.\n\nExample: curl -X POST http://api.com/chat -d 'user={{userID}}'"
	case FieldLoadMode:
		return "Load Generation Mode.\n• [RPS] (Open Loop): Generates requests at a fixed rate, regardless of server response time.\n• [Users] (Closed Loop): Simulates fixed concurrent users. A new request starts only after previous one finishes (+ think time).\n\nPress [Space] to toggle."
	case FieldQPS:
		mode := m.Inputs[FieldLoadMode].Value()
		if mode == "users" {
			return "Number of concurrent users (virtual users) to simulate.\nEach user runs sequentially."
		}
		return "Target Requests Per Second (RPS).\nThe engine will attempt to hit this throughput strictly."
	case FieldDuration:
		return "The duration of the 'Steady State' phase.\nTotal Run Time = RampUp + Duration + RampDown."
	case FieldRampUp:
		mode := m.Inputs[FieldLoadMode].Value()
		if mode == "users" {
			return "Time period (in seconds) to gradually spawn all users.\nPrevents hitting the server with all users at once."
		}
		return "Time period (in seconds) to linearly increase RPS from 0 to Target."
	case FieldRampDown:
		return "Time period (in seconds) to linearly decrease RPS from Target to 0.\nUseful for graceful shutdown testing."
	case FieldThinkTime:
		return "Artificial delay (in milliseconds) between requests for a single user.\nOnly applies in [Users] mode."
	}
	return ""
}

// ... (Constants and NewRunnerView unchanged) ...

func (m RunnerView) View() string {
	s := strings.Builder{}
	s.WriteString(styles.Title.Render("🚀 Configure Load Test"))
	s.WriteString("\n\n")

	reqType := m.Inputs[FieldReqType].Value()
	loadMode := m.Inputs[FieldLoadMode].Value()

	// Row 0: Request Type
	s.WriteString(m.renderRow(FieldReqType, -1))
	s.WriteString("\n")

	// Row 1: Details
	if reqType == "http" {
		s.WriteString(m.renderRow(FieldURL, FieldMethod)) // URL, Method
	} else {
		s.WriteString(m.renderRow(FieldCommand, -1)) // Command
	}
	s.WriteString("\n")

	// Row 2: Load Mode & QPS/Users
	s.WriteString(m.renderRow(FieldLoadMode, FieldQPS))
	s.WriteString("\n")

	// Row 3: Duration & RampUp
	s.WriteString(m.renderRow(FieldDuration, FieldRampUp))
	s.WriteString("\n")

	// Row 4: RampDown or ThinkTime
	if loadMode == "rps" {
		s.WriteString(m.renderRow(FieldRampDown, -1))
	} else {
		s.WriteString(m.renderRow(FieldThinkTime, -1))
	}
	s.WriteString("\n\n")

	// Dynamic Help Box
	helpContent := m.GetHelp()
	if helpContent != "" {
		s.WriteString(styles.Subtle.Render("──────── Information ────────"))
		s.WriteString("\n")
		s.WriteString(styles.Text.Foreground(styles.ColorSecondary).Width(70).Render(helpContent))
	} else {
		s.WriteString("\n")
	}

	return s.String()
}

// Field Indices
const (
	FieldReqType = iota // HTTP vs Script
	FieldURL
	FieldMethod
	FieldCommand
	FieldLoadMode // RPS vs Users
	FieldQPS      // or NumUsers
	FieldDuration
	FieldRampUp
	FieldRampDown
	FieldThinkTime
	// Helper
	FieldNumUsers = FieldQPS // Alias
)

func NewRunnerView(initialCfg runner.Config) RunnerView {
	inputs := make([]textinput.Model, 10)

	// 0. ReqType
	inputs[FieldReqType] = textinput.New()
	if initialCfg.Command != "" {
		inputs[FieldReqType].SetValue("script")
	} else {
		inputs[FieldReqType].SetValue("http")
	}
	inputs[FieldReqType].Prompt = "Type (Space): "
	inputs[FieldReqType].Width = 10
	inputs[FieldReqType].Focus()

	// 1. URL
	inputs[FieldURL] = textinput.New()
	inputs[FieldURL].Placeholder = "http://localhost:8080"
	inputs[FieldURL].SetValue(initialCfg.URL)
	inputs[FieldURL].Prompt = "URL: "
	inputs[FieldURL].Width = 50

	// 2. Method
	inputs[FieldMethod] = textinput.New()
	inputs[FieldMethod].Placeholder = "GET"
	inputs[FieldMethod].SetValue(initialCfg.Method)
	inputs[FieldMethod].Prompt = "Method: "
	inputs[FieldMethod].Width = 10

	// 3. Command
	inputs[FieldCommand] = textinput.New()
	inputs[FieldCommand].Placeholder = "bash test.sh"
	inputs[FieldCommand].SetValue(initialCfg.Command)
	inputs[FieldCommand].Prompt = "Shell Command: "
	inputs[FieldCommand].Width = 60

	// 4. LoadMode
	inputs[FieldLoadMode] = textinput.New()
	inputs[FieldLoadMode].SetValue(initialCfg.Mode)
	if initialCfg.Mode == "" {
		inputs[FieldLoadMode].SetValue("rps")
	}
	inputs[FieldLoadMode].Prompt = "Mode (Space): "
	inputs[FieldLoadMode].Width = 10

	// 5. QPS / Users
	inputs[FieldQPS] = textinput.New()
	if initialCfg.Mode == "users" {
		inputs[FieldQPS].SetValue(strconv.Itoa(initialCfg.NumUsers))
		inputs[FieldQPS].Prompt = "Users: "
	} else {
		inputs[FieldQPS].SetValue(strconv.Itoa(initialCfg.TargetRPS))
		inputs[FieldQPS].Prompt = "Target QPS: "
	}
	inputs[FieldQPS].Width = 10

	// 6. Duration
	inputs[FieldDuration] = textinput.New()
	inputs[FieldDuration].Placeholder = "30"
	inputs[FieldDuration].SetValue(strconv.Itoa(initialCfg.SteadyDur))
	inputs[FieldDuration].Prompt = "Duration (s): "
	inputs[FieldDuration].Width = 10

	// 7. RampUp
	inputs[FieldRampUp] = textinput.New()
	inputs[FieldRampUp].Placeholder = "0"
	inputs[FieldRampUp].SetValue(strconv.Itoa(initialCfg.RampUp))
	inputs[FieldRampUp].Prompt = "Ramp Up (s): "
	inputs[FieldRampUp].Width = 10

	// 8. RampDown
	inputs[FieldRampDown] = textinput.New()
	inputs[FieldRampDown].Placeholder = "0"
	inputs[FieldRampDown].SetValue(strconv.Itoa(initialCfg.RampDown))
	inputs[FieldRampDown].Prompt = "Ramp Down (s): "
	inputs[FieldRampDown].Width = 10

	// 9. ThinkTime
	inputs[FieldThinkTime] = textinput.New()
	inputs[FieldThinkTime].Placeholder = "0"
	inputs[FieldThinkTime].SetValue(strconv.Itoa(int(initialCfg.ThinkTime.Milliseconds())))
	inputs[FieldThinkTime].Prompt = "Think (ms): "
	inputs[FieldThinkTime].Width = 10

	return RunnerView{
		Inputs:  inputs,
		Focus:   0,
		Editing: true,
	}
}

func (m RunnerView) Init() tea.Cmd {
	return textinput.Blink
}

func (m RunnerView) Update(msg tea.Msg) (RunnerView, tea.Cmd) {
	reqType := m.Inputs[FieldReqType].Value()
	loadMode := m.Inputs[FieldLoadMode].Value()

	// Handle Navigation & Toggles
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "down", "enter":
			m.Focus = m.nextFocus(m.Focus, 1, reqType, loadMode)
			return m.focusCmd()
		case "shift+tab", "up":
			m.Focus = m.nextFocus(m.Focus, -1, reqType, loadMode)
			return m.focusCmd()
		case " ":
			if m.Focus == FieldReqType {
				if reqType == "http" {
					m.Inputs[FieldReqType].SetValue("script")
				} else {
					m.Inputs[FieldReqType].SetValue("http")
				}
				return m, nil
			}
			if m.Focus == FieldLoadMode {
				if loadMode == "rps" {
					m.Inputs[FieldLoadMode].SetValue("users")
					m.Inputs[FieldQPS].Prompt = "Users: "
				} else {
					m.Inputs[FieldLoadMode].SetValue("rps")
					m.Inputs[FieldQPS].Prompt = "Target QPS: "
				}
				return m, nil
			}
		}
	}

	// Update inputs
	cmds := make([]tea.Cmd, len(m.Inputs))
	for i := range m.Inputs {
		m.Inputs[i], cmds[i] = m.Inputs[i].Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m RunnerView) nextFocus(current, direction int, reqType, loadMode string) int {
	// Build visible list
	visible := []int{FieldReqType}

	if reqType == "http" {
		visible = append(visible, FieldURL, FieldMethod)
	} else {
		visible = append(visible, FieldCommand)
	}

	visible = append(visible, FieldLoadMode, FieldQPS, FieldDuration, FieldRampUp)

	if loadMode == "rps" {
		visible = append(visible, FieldRampDown)
	} else {
		visible = append(visible, FieldThinkTime)
	}

	// Find current index
	idx := -1
	for i, v := range visible {
		if v == current {
			idx = i
			break
		}
	}

	if idx == -1 {
		return FieldReqType // Default
	}

	nextIdx := (idx + direction) % len(visible)
	if nextIdx < 0 {
		nextIdx = len(visible) - 1
	}

	return visible[nextIdx]
}

func (m RunnerView) focusCmd() (RunnerView, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.Inputs))
	for i := 0; i < len(m.Inputs); i++ {
		if i == m.Focus {
			cmds[i] = m.Inputs[i].Focus()
			m.Inputs[i].PromptStyle = styles.Active
			m.Inputs[i].TextStyle = styles.Text
		} else {
			m.Inputs[i].Blur()
			m.Inputs[i].PromptStyle = styles.Subtle
			m.Inputs[i].TextStyle = styles.Subtle
		}
	}
	return m, tea.Batch(cmds...)
}


func (m RunnerView) renderRow(idx1, idx2 int) string {
	v1 := m.renderInput(idx1)
	v2 := ""
	if idx2 >= 0 {
		v2 = m.renderInput(idx2)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, v1, "    ", v2)
}

func (m RunnerView) renderInput(idx int) string {
	style := styles.InputNormal
	if idx == m.Focus {
		style = styles.InputActive
	}
	return style.Render(m.Inputs[idx].View())
}

func (m RunnerView) GetConfig() runner.Config {
	reqType := m.Inputs[FieldReqType].Value()

	url := m.Inputs[FieldURL].Value()
	method := m.Inputs[FieldMethod].Value()
	cmd := m.Inputs[FieldCommand].Value()

	if reqType == "http" {
		cmd = ""
	} else {
		// Script mode
		// url/method ignored by runner logic if cmd present
	}

	mode := m.Inputs[FieldLoadMode].Value()
	qps, _ := strconv.Atoi(m.Inputs[FieldQPS].Value()) // Reused for users
	dur, _ := strconv.Atoi(m.Inputs[FieldDuration].Value())
	rup, _ := strconv.Atoi(m.Inputs[FieldRampUp].Value())
	rdown, _ := strconv.Atoi(m.Inputs[FieldRampDown].Value())
	think, _ := strconv.Atoi(m.Inputs[FieldThinkTime].Value())

	// QPS input is Users count if mode is users
	targetRPS := 0
	numUsers := 1
	if mode == "users" {
		numUsers = qps
	} else {
		targetRPS = qps
	}

	return runner.Config{
		URL:        url,
		Method:     method,
		Command:    cmd,
		TargetRPS:  targetRPS,
		SteadyDur:  dur,
		RampUp:     rup,
		RampDown:   rdown,
		NumUsers:   numUsers,
		ThinkTime:  time.Duration(think) * time.Millisecond,
		Mode:       mode,
		TimeoutSec: 30,
	}
}
