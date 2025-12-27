package app

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/tui/styles"
	"steadyq/internal/tui/views"
)

type ClearStatusMsg struct{}

func clearStatusCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}

// View Enum
type ViewID int

const (
	ViewRunner ViewID = iota
	ViewDashboard
)

type StatsMsg runner.StatsSnapshot

type Model struct {
	Runner  *runner.Runner
	Updates runner.StatsUpdateChan

	// Core State
	RunActive bool
	RunCtx    context.Context // To cancel run
	RunCancel context.CancelFunc

	// Layout
	Width  int
	Height int

	CurrentView ViewID
	MenuItems   []string

	RunnerView views.RunnerView
	DashView   views.DashboardView

	// Feedback
	StatusMsg string
}

func NewModel(r *runner.Runner, updates runner.StatsUpdateChan) Model {
	return Model{
		Runner:      r,
		Updates:     updates,
		CurrentView: ViewRunner,
		MenuItems:   []string{"[1] New Run", "[2] Dashboard"},
		RunnerView:  views.NewRunnerView(r.Cfg),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.RunnerView.Init(),
		waitForUpdate(m.Updates),
	)
}

func waitForUpdate(sub runner.StatsUpdateChan) tea.Cmd {
	return func() tea.Msg {
		return StatsMsg(<-sub)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ClearStatusMsg:
		m.StatusMsg = ""
		return m, nil

	case tea.KeyMsg:
		// 1. GLOBAL NAVIGATION & CONTROL (Prioritized)
		switch msg.String() {
		case "ctrl+c", "ctrl+q": // Removed "q" to allow typing
			return m, tea.Quit

		case "ctrl+d": // Dashboard
			m.CurrentView = ViewDashboard
			return m, nil

		case "ctrl+right":
			m.CurrentView++
			if m.CurrentView > ViewDashboard {
				m.CurrentView = ViewRunner
			}
			return m, nil
		case "ctrl+left":
			m.CurrentView--
			if m.CurrentView < ViewRunner {
				m.CurrentView = ViewDashboard
			}
			return m, nil
		// Removed 1, 2, 3 to allow numeric input

		// 2. ACTIONS
		case "ctrl+r": // Run
			if m.CurrentView == ViewRunner {
				cfg := m.RunnerView.GetConfig()
				m.startRun(cfg)
			}
			return m, nil

		case "ctrl+s": // Stop
			if m.RunActive && m.RunCancel != nil {
				m.RunCancel()
				m.RunActive = false
			}
			return m, nil
		}

		// 3. FALLTHROUGH: VIEW SPECIFIC UPDATE
		// Key wasn't global, pass to active view
		// ... (Logic continues below in default case)

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		contentHeight := m.Height - 7 // Increased footer space (3 rows now)

		m.RunnerView.Width = m.Width
		m.RunnerView.Height = contentHeight

		m.DashView.Width = m.Width
		m.DashView.Height = contentHeight

		updatedDash, _ := m.DashView.Update(msg)
		m.DashView = updatedDash

	case StatsMsg:
		snap := runner.StatsSnapshot(msg)
		updatedDash, c := m.DashView.Update(snap)
		m.DashView = updatedDash
		cmds = append(cmds, c)

		// Check for Completion (Time based)
		elapsed := time.Since(m.DashView.StartTime)
		if m.RunActive && elapsed >= m.DashView.Duration {
			// Test finished naturally
			m.RunActive = false
			if m.RunCancel != nil {
				m.RunCancel()
			}
			m.StatusMsg = "Test Completed."
		}

		cmds = append(cmds, waitForUpdate(m.Updates))
	}

	// DEFAULT: Forward all other messages (KeyMsg that fell through, FrameMsg, BlinkMsg, etc.)
	// This is CRITICAL for Bubbles to work (Progress bar animation, Input blinking)
	var defaultCmd tea.Cmd
	switch m.CurrentView {
	case ViewRunner:
		m.RunnerView, defaultCmd = m.RunnerView.Update(msg)
	case ViewDashboard:
		m.DashView, defaultCmd = m.DashView.Update(msg)
	}
	cmds = append(cmds, defaultCmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) startRun(cfg runner.Config) {
	// Cancel existing if any
	if m.RunActive && m.RunCancel != nil {
		m.RunCancel()
	}

	m.Runner.Cfg = cfg
	m.Runner.Stats.Reset()

	ctx, cancel := context.WithCancel(context.Background())
	m.RunCtx = ctx
	m.RunCancel = cancel
	m.RunActive = true

	// totalDur calculated in NewDashboardView
	m.DashView = views.NewDashboardView(cfg)
	m.DashView.Width = m.Width
	m.DashView.Height = m.Height - 6 // Adjusted for footer

	m.CurrentView = ViewDashboard

	go m.Runner.Run(ctx)
}

func (m Model) View() string {
	if m.Width == 0 {
		return "Loading..."
	}

	nav := strings.Builder{}
	for i, item := range m.MenuItems {
		if ViewID(i) == m.CurrentView {
			nav.WriteString(styles.TabActive.Render(item))
		} else {
			nav.WriteString(styles.TabBase.Render(item))
		}
	}
	navBar := styles.FooterBase.Width(m.Width).Render(nav.String())

	contentStr := ""
	switch m.CurrentView {
	case ViewRunner:
		contentStr = m.RunnerView.View()
	case ViewDashboard:
		contentStr = m.DashView.View()
	}

	// Adjust height for larger footer
	content := styles.Panel.Width(m.Width - 2).Height(m.Height - 6).Render(contentStr)

	// Help Grid
	// Row 1: Navigation
	keys1 := []string{
		styles.RenderKey("Ctrl+<->", "View"),
		styles.RenderKey("Tab", "Field"),
		styles.RenderKey("Enter", "Edit"),
	}

	// Row 2: Actions
	keys2 := []string{
		styles.RenderKey("Ctrl+R", "Run"),
		styles.RenderKey("Ctrl+S", "Stop"),
		styles.RenderKey("Ctrl+Q", "Quit"),
	}

	// Row 3: Shortcuts
	keys3 := []string{
		styles.RenderKey("Ctrl+D", "Dash"),
	}

	helpRow1 := styles.FooterBase.Width(m.Width).Render(strings.Join(keys1, "   "))
	helpRow2 := styles.FooterBase.Width(m.Width).Render(strings.Join(keys2, "   "))
	helpRow3 := styles.FooterBase.Width(m.Width).Render(strings.Join(keys3, "   "))

	footer := lipgloss.JoinVertical(lipgloss.Left, helpRow1, helpRow2, helpRow3)

	// Status Overlay? Or just append to footer?
	// Let's replace footer keybindings with status if exists, or append above footer.
	if m.StatusMsg != "" {
		status := styles.Box.BorderForeground(styles.ColorHighlight).Render(m.StatusMsg)
		// Float it at the bottom above footer
		return lipgloss.JoinVertical(lipgloss.Left, navBar, content, status, footer)
	}

	return lipgloss.JoinVertical(lipgloss.Left, navBar, content, footer)
}
