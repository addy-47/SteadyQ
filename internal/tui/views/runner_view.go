package views

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/tui/styles"
)

type RunnerView struct {
	Inputs  []textinput.Model
	Headers textarea.Model
	Body    textarea.Model
	Focus   int
	Editing bool

	Viewport viewport.Model

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
	case FieldHeaders:
		return "Custom HTTP Headers.\nFormat: Key: Value (one per line).\nExample:\nAuthorization: Bearer abc\n\nNavigation:\n• [Tab] Next Field\n• [Arrows] Line navigation\n• [Down] (at end) Next field\n• [Ctrl+N/P] Force Nav"
	case FieldBody:
		return "The Request Body.\nUsually JSON or raw text.\n\nNavigation:\n• [Tab] Next Field\n• [Arrows] Line navigation\n• [Down] (at end) Next field"
	case FieldCommand:
		return "The Shell Command to execute for each 'request'.\n\nTemplate Variables:\n• {{userID}}: Stable ID for the Virtual User (persists across requests).\n• {{uuid}}: A fresh, random UUID v4 (36-character string) generated for every request."
	case FieldLoadMode:
		return "Load Generation Mode.\n• [RPS] (Open Loop): Generates requests at a fixed rate.\n• [Users] (Closed Loop): Simulates fixed concurrent users.\n\nPress [Space] to toggle."
	case FieldRPS:
		if m.Inputs[FieldLoadMode].Value() == "users" {
			return "Number of concurrent users (virtual users) to simulate."
		}
		return "Target Requests Per Second (RPS)."
	case FieldDuration:
		return "The duration of the 'Steady State' phase."
	case FieldRampUp:
		return "Time period (s) to reach Target (RPS or Users)."
	case FieldRampDown:
		return "Time period (s) to gracefully decrease RPS to 0."
	case FieldThinkTime:
		return "Delay (ms) between requests per user (Users mode only)."
	}
	return ""
}

// ... (Constants and NewRunnerView unchanged) ...

func (m RunnerView) View() string {
	reqType := m.Inputs[FieldReqType].Value()
	loadMode := m.Inputs[FieldLoadMode].Value()

	// 1. Left Side: Inputs
	inputCol := strings.Builder{}
	inputCol.WriteString("\n") // Top margin

	inputCol.WriteString(m.renderInput(FieldReqType))
	inputCol.WriteString("\n")

	if reqType == "http" {
		inputCol.WriteString(m.renderInput(FieldURL))
		inputCol.WriteString("\n")
		inputCol.WriteString(m.renderInput(FieldMethod))
		inputCol.WriteString("\n")
		inputCol.WriteString(m.renderInput(FieldHeaders))
		inputCol.WriteString("\n")
		inputCol.WriteString(m.renderInput(FieldBody))
		inputCol.WriteString("\n")
	} else {
		inputCol.WriteString(m.renderInput(FieldCommand))
		inputCol.WriteString("\n")
	}

	inputCol.WriteString(m.renderInput(FieldLoadMode))
	inputCol.WriteString("\n")
	inputCol.WriteString(m.renderInput(FieldRPS))
	inputCol.WriteString("\n")
	inputCol.WriteString(m.renderInput(FieldDuration))
	inputCol.WriteString("\n")
	inputCol.WriteString(m.renderInput(FieldRampUp))
	inputCol.WriteString("\n")

	if loadMode == "rps" {
		inputCol.WriteString(m.renderInput(FieldRampDown))
	} else {
		inputCol.WriteString(m.renderInput(FieldThinkTime))
	}

	// 2. Right Side: Help
	helpCol := strings.Builder{}
	helpBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(1, 2).
		Width(45).
		Height(15) // Fixed height for help or dynamic?

	helpTitle := styles.Subtle.Bold(true).Render("Information")
	helpContent := m.GetHelp()

	helpCol.WriteString(helpTitle)
	helpCol.WriteString("\n\n")
	helpCol.WriteString(styles.Text.Foreground(styles.ColorSecondary).Render(helpContent))

	mainRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(55).Render(inputCol.String()),
		helpBox.Render(helpCol.String()),
	)

	// Set viewport content
	m.Viewport.SetContent(mainRow)
	return m.Viewport.View()
}

// Field Indices
const (
	FieldReqType = iota
	FieldURL
	FieldMethod
	FieldHeaders
	FieldBody
	FieldCommand
	FieldLoadMode
	FieldRPS
	FieldDuration
	FieldRampUp
	FieldRampDown
	FieldThinkTime
)

func NewRunnerView(initialCfg runner.Config) RunnerView {
	inputs := make([]textinput.Model, 12)

	// Base settings for all inputs
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].PromptStyle = styles.Subtle
		inputs[i].TextStyle = styles.Text
	}

	inputs[FieldReqType].SetValue(ternary(initialCfg.Command != "", "script", "http"))
	inputs[FieldReqType].Prompt = "Type (Space): "
	inputs[FieldReqType].Width = 10
	inputs[FieldReqType].Focus()

	inputs[FieldURL].Placeholder = "http://localhost:8080"
	inputs[FieldURL].SetValue(initialCfg.URL)
	inputs[FieldURL].Prompt = "URL: "
	inputs[FieldURL].Width = 40

	inputs[FieldMethod].Placeholder = "GET"
	inputs[FieldMethod].SetValue(ternary(initialCfg.Method != "", initialCfg.Method, "GET"))
	inputs[FieldMethod].Prompt = "Method: "
	inputs[FieldMethod].Width = 10

	// TextAreas for Headers and Body
	hArea := textarea.New()
	hArea.Placeholder = "Key: Value\nAuthorization: Bearer ..."
	var hLines []string
	for k, v := range initialCfg.Headers {
		hLines = append(hLines, k+": "+v)
	}
	hArea.SetValue(strings.Join(hLines, "\n"))
	hArea.SetWidth(40)
	hArea.SetHeight(5)
	hArea.Prompt = ""

	bArea := textarea.New()
	bArea.Placeholder = "{\n  \"key\": \"value\"\n}"
	bArea.SetValue(initialCfg.Body)
	bArea.SetWidth(40)
	bArea.SetHeight(5)
	bArea.Prompt = ""

	inputs[FieldCommand].Placeholder = "bash test.sh"
	inputs[FieldCommand].SetValue(initialCfg.Command)
	inputs[FieldCommand].Prompt = "Shell Command: "
	inputs[FieldCommand].Width = 40

	inputs[FieldLoadMode].SetValue(ternary(initialCfg.Mode != "", initialCfg.Mode, "rps"))
	inputs[FieldLoadMode].Prompt = "Mode (Space): "
	inputs[FieldLoadMode].Width = 10

	if initialCfg.Mode == "users" {
		inputs[FieldRPS].SetValue(strconv.Itoa(initialCfg.NumUsers))
		inputs[FieldRPS].Prompt = "Users: "
	} else {
		inputs[FieldRPS].SetValue(strconv.Itoa(initialCfg.TargetRPS))
		inputs[FieldRPS].Prompt = "Target RPS: "
	}
	inputs[FieldRPS].Width = 10

	inputs[FieldDuration].SetValue(strconv.Itoa(initialCfg.SteadyDur))
	inputs[FieldDuration].Prompt = "Duration (s): "
	inputs[FieldDuration].Width = 10

	inputs[FieldRampUp].SetValue(strconv.Itoa(initialCfg.RampUp))
	inputs[FieldRampUp].Prompt = "Ramp Up (s): "
	inputs[FieldRampUp].Width = 10

	inputs[FieldRampDown].SetValue(strconv.Itoa(initialCfg.RampDown))
	inputs[FieldRampDown].Prompt = "Ramp Down (s): "
	inputs[FieldRampDown].Width = 10

	inputs[FieldThinkTime].SetValue(strconv.Itoa(int(initialCfg.ThinkTime.Milliseconds())))
	inputs[FieldThinkTime].Prompt = "Think (ms): "
	inputs[FieldThinkTime].Width = 10

	return RunnerView{
		Inputs:   inputs,
		Headers:  hArea,
		Body:     bArea,
		Focus:    0,
		Editing:  true,
		Viewport: viewport.New(0, 0),
	}
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func (m RunnerView) Init() tea.Cmd {
	return textinput.Blink
}

func (m RunnerView) Update(msg tea.Msg) (RunnerView, tea.Cmd) {
	reqType := m.Inputs[FieldReqType].Value()
	loadMode := m.Inputs[FieldLoadMode].Value()
	var cmds []tea.Cmd

	// Handle Navigation & Toggles
	isNav := false
	dir := 0

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "ctrl+n":
			isNav = true
			dir = 1
		case "shift+tab", "ctrl+p":
			isNav = true
			dir = -1
		case "down":
			if m.Focus == FieldHeaders || m.Focus == FieldBody {
				break // Handle internally for multi-line
			}
			isNav = true
			dir = 1
		case "up":
			if m.Focus == FieldHeaders || m.Focus == FieldBody {
				break // Handle internally for multi-line
			}
			isNav = true
			dir = -1
		case "enter":
			if m.Focus == FieldHeaders || m.Focus == FieldBody {
				break // Allow newline
			}
			isNav = true
			dir = 1
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
					m.Inputs[FieldRPS].Prompt = "Users: "
				} else {
					m.Inputs[FieldLoadMode].SetValue("rps")
					m.Inputs[FieldRPS].Prompt = "Target RPS: "
				}
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Viewport.Width = msg.Width - 4
		m.Viewport.Height = msg.Height - 8
	}

	if isNav {
		m.Focus = m.nextFocus(m.Focus, dir, reqType, loadMode)
		newM, cmd := m.focusCmd()
		m = newM
		cmds = append(cmds, cmd)
	} else {
		// Update active component
		if m.Focus == FieldHeaders {
			var cmd tea.Cmd
			m.Headers, cmd = m.Headers.Update(msg)
			cmds = append(cmds, cmd)
		} else if m.Focus == FieldBody {
			var cmd tea.Cmd
			m.Body, cmd = m.Body.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			for i := range m.Inputs {
				var cmd tea.Cmd
				m.Inputs[i], cmd = m.Inputs[i].Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	var vpCmd tea.Cmd
	m.Viewport, vpCmd = m.Viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m RunnerView) nextFocus(current, direction int, reqType, loadMode string) int {
	// Build visible list
	visible := []int{FieldReqType}

	if reqType == "http" {
		visible = append(visible, FieldURL, FieldMethod, FieldHeaders, FieldBody)
	} else {
		visible = append(visible, FieldCommand)
	}

	visible = append(visible, FieldLoadMode, FieldRPS, FieldDuration, FieldRampUp)

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
	cmds := make([]tea.Cmd, 0)
	for i := 0; i < len(m.Inputs); i++ {
		if i == m.Focus {
			cmds = append(cmds, m.Inputs[i].Focus())
			m.Inputs[i].PromptStyle = styles.Active
			m.Inputs[i].TextStyle = styles.Text
		} else {
			m.Inputs[i].Blur()
			m.Inputs[i].PromptStyle = styles.Subtle
			m.Inputs[i].TextStyle = styles.Subtle
		}
	}

	if m.Focus == FieldHeaders {
		cmds = append(cmds, m.Headers.Focus())
	} else {
		m.Headers.Blur()
	}

	if m.Focus == FieldBody {
		cmds = append(cmds, m.Body.Focus())
	} else {
		m.Body.Blur()
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

	if idx == FieldHeaders {
		return style.Render("Headers:\n" + m.Headers.View())
	}
	if idx == FieldBody {
		return style.Render("Body:\n" + m.Body.View())
	}

	return style.Render(m.Inputs[idx].View())
}

func (m RunnerView) GetConfig() runner.Config {
	reqType := m.Inputs[FieldReqType].Value()
	url := m.Inputs[FieldURL].Value()
	method := m.Inputs[FieldMethod].Value()
	cmd := m.Inputs[FieldCommand].Value()
	body := m.Body.Value()

	// Parse Headers
	headers := make(map[string]string)
	hRaw := m.Headers.Value()
	if hRaw != "" {
		lines := strings.Split(hRaw, "\n")
		for _, l := range lines {
			kv := strings.SplitN(strings.TrimSpace(l), ":", 2)
			if len(kv) == 2 {
				headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	if reqType == "http" {
		cmd = ""
	}

	mode := m.Inputs[FieldLoadMode].Value()
	rps, _ := strconv.Atoi(m.Inputs[FieldRPS].Value())
	dur, _ := strconv.Atoi(m.Inputs[FieldDuration].Value())
	rup, _ := strconv.Atoi(m.Inputs[FieldRampUp].Value())
	rdown, _ := strconv.Atoi(m.Inputs[FieldRampDown].Value())
	think, _ := strconv.Atoi(m.Inputs[FieldThinkTime].Value())

	targetRPS := 0
	numUsers := 1
	if mode == "users" {
		numUsers = rps
	} else {
		targetRPS = rps
	}

	return runner.Config{
		URL:        url,
		Method:     method,
		Headers:    headers,
		Body:       body,
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
