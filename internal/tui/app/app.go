package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/storage"
	"steadyq/internal/tui/styles"
	"steadyq/internal/tui/views"
)

// View Enum
type ViewID int

const (
	ViewRunner ViewID = iota
	ViewDashboard
	ViewHistory
)

// StatsMsg wrapper
type StatsMsg runner.StatsSnapshot

type Model struct {
	// Global State
	Runner  *runner.Runner
	Store   *storage.Store
	Updates runner.StatsUpdateChan

	// Layout
	Width  int
	Height int

	// Navigation
	CurrentView ViewID
	MenuItems   []string
	RunActive   bool

	// Views
	RunnerView  views.RunnerView
	DashView    views.DashboardView
	HistoryView views.HistoryView
}

func (m Model) saveHistory() {
	if m.Store == nil {
		return
	}

	item := storage.HistoryItem{
		ID:        fmt.Sprintf("%d", time.Now().Unix()),
		Timestamp: time.Now(),
		Config:    m.Runner.Cfg,
		Summary: storage.RunSummary{
			TotalRequests: m.Runner.Stats.Requests,
			Success:       m.Runner.Stats.Success,
			Fail:          m.Runner.Stats.Fail,
			AvgLatencyMs:  m.Runner.Stats.ServiceTime.Mean() / 1000.0,
			P99LatencyMs:  m.Runner.Stats.GetP99Service(),
		},
	}
	m.Store.Save(item)
	// Refresh history view if it exists
	m.HistoryView.Refresh()
}

func NewModel(r *runner.Runner, updates runner.StatsUpdateChan, store *storage.Store) Model {
	return Model{
		Runner:      r,
		Updates:     updates,
		Store:       store,
		CurrentView: ViewRunner,
		MenuItems:   []string{"New Test", "Dashboard", "History"},
		RunnerView:  views.NewRunnerView(r.Cfg),
		HistoryView: views.NewHistoryView(store),
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
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global Keys
		switch msg.String() {
		case "ctrl+c":
			m.Runner.Stats.Reset() // Hacky: stop runner? No, just quit.
			// Ideally cancel context.
			return m, tea.Quit
		case "f1":
			m.CurrentView = ViewRunner
			return m, nil
		case "f2":
			m.CurrentView = ViewDashboard
			return m, nil
		case "f3":
			m.CurrentView = ViewHistory
			m.HistoryView.Refresh() // Refresh on enter
			return m, nil
		}

		// View Specific Handling
		if m.CurrentView == ViewRunner {
			if msg.String() == "enter" && m.RunnerView.Focus >= 3 {
				// START TEST
				cfg := m.RunnerView.GetConfig()
				m.Runner.Cfg = cfg
				m.CurrentView = ViewDashboard

				// Reset & Init Stats/Runner
				m.Runner.Stats.Reset()

				// Init Dashboard
				totalDur := time.Duration(cfg.RampUp+cfg.SteadyDur+cfg.RampDown) * time.Second
				m.DashView = views.NewDashboardView(totalDur)
				m.DashView.Width = m.Width - 25
				m.DashView.Height = m.Height

				// Launch
				m.RunActive = true
				go m.Runner.Run(context.TODO())

				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.RunnerView.Width = msg.Width - 25
		m.DashView.Width = msg.Width - 25
		m.DashView.Height = msg.Height
		m.HistoryView.Width = msg.Width - 25
		m.HistoryView.Height = msg.Height // Pass full height, view handles reservation

		updatedDash, _ := m.DashView.Update(msg)
		m.DashView = updatedDash
		updatedHist, _ := m.HistoryView.Update(msg)
		m.HistoryView = updatedHist

	case StatsMsg:
		snap := runner.StatsSnapshot(msg)

		updatedDash, c := m.DashView.Update(snap)
		m.DashView = updatedDash
		cmds = append(cmds, c)

		// If test just finished?
		if snap.Inflight == 0 && m.DashView.Progress.Percent() >= 1.0 {
			// Check if we already saved this run?
			// We can use a simple flag in Model, or just check if last saved timestamp is close?
			// Better: Add `RunActive bool` to Model.
			if m.RunActive {
				m.saveHistory()
				m.RunActive = false
			}
		}

		cmds = append(cmds, waitForUpdate(m.Updates))
	}

	// Propagate to active view
	switch m.CurrentView {
	case ViewRunner:
		m.RunnerView, cmd = m.RunnerView.Update(msg)
		cmds = append(cmds, cmd)
	case ViewDashboard:
		// m.DashView, cmd = m.DashView.Update(msg)
	case ViewHistory:
		m.HistoryView, cmd = m.HistoryView.Update(msg)
		cmds = append(cmds, cmd)

		// Check Replay
		if m.HistoryView.SelectedConfig != nil {
			// Replay!
			cfg := *m.HistoryView.SelectedConfig
			m.RunnerView = views.NewRunnerView(cfg) // Reset Runner Form with new config
			m.RunnerView.Width = m.Width - 25
			m.HistoryView.SelectedConfig = nil // Clear flag
			m.CurrentView = ViewRunner
			return m, nil
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.Width == 0 {
		return "Initializing..."
	}

	sidebarWidth := 20
	contentWidth := m.Width - sidebarWidth - 4

	// 1. Sidebar
	sidebar := strings.Builder{}
	sidebar.WriteString(styles.Title.Render("⚡ SteadyQ"))
	sidebar.WriteString("\n\n")

	for i, item := range m.MenuItems {
		if ViewID(i) == m.CurrentView {
			sidebar.WriteString(styles.MenuItemActive.Render(item))
		} else {
			sidebar.WriteString(styles.MenuItem.Render(item))
		}
		sidebar.WriteString("\n")
	}

	// 2. Content
	content := ""
	switch m.CurrentView {
	case ViewRunner:
		content = m.RunnerView.View()
	case ViewDashboard:
		content = m.DashView.View()
	case ViewHistory:
		content = m.HistoryView.View()
	}

	// 3. Compose
	leftPane := styles.Panel.
		Width(sidebarWidth).
		Height(m.Height - 2).
		Render(sidebar.String())

	rightPane := styles.PanelActive.
		Width(contentWidth).
		Height(m.Height - 2).
		Render(content)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}
