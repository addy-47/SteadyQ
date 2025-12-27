package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/runner"
	"steadyq/internal/storage"
	"steadyq/internal/tui/styles"
)

// Add debug check
var historyLoadError error

type HistoryView struct {
	Store *storage.Store
	Table table.Model

	SelectedConfig *runner.Config // Output for parent to grab

	Width  int
	Height int
}

func NewHistoryView(store *storage.Store) HistoryView {
	columns := []table.Column{
		{Title: "Time", Width: 20},
		{Title: "URL", Width: 40},
		{Title: "QPS", Width: 10},
		{Title: "Reqs", Width: 10},
		{Title: "Success", Width: 10},
		{Title: "P99 (ms)", Width: 12}, // Added P99
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10), // Will resize
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.ColorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(styles.ColorPrimary)

	s.Selected = s.Selected.
		Foreground(styles.ColorBg).
		Background(styles.ColorPrimary).
		Bold(true)

	t.SetStyles(s)

	m := HistoryView{
		Store: store,
		Table: t,
	}
	m.Refresh()
	return m
}

func (m *HistoryView) Refresh() {
	if m.Store == nil {
		return
	}

	items := m.Store.List()
	// Reverse order (newest first)
	rows := make([]table.Row, len(items))
	for i := 0; i < len(items); i++ {
		item := items[len(items)-1-i]
		rows[i] = table.Row{
			item.Timestamp.Format("15:04:05"),
			item.Config.URL,
			fmt.Sprintf("%d", item.Config.TargetRPS),
			fmt.Sprintf("%d", item.Summary.TotalRequests),
			fmt.Sprintf("%d", item.Summary.Success),
			fmt.Sprintf("%.2f", item.Summary.P99LatencyMs),
		}
	}
	m.Table.SetRows(rows)
}

func (m HistoryView) Init() tea.Cmd {
	return nil
}

func (m HistoryView) Update(msg tea.Msg) (HistoryView, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Table.SetWidth(msg.Width - 4)
		m.Table.SetHeight(msg.Height - 6) // Reserve space for header
		m.Refresh()                       // Re-fetch on resize just in case

	case tea.KeyMsg:
		if msg.String() == "ctrl+h" {
			m.Refresh() // Explicit refresh on shortcut (though app handles view switch)
		}
		if msg.String() == "enter" {
			// ... (rest of enter logic)
			// Select item
			idx := m.Table.Cursor()
			items := m.Store.List()
			if idx >= 0 && idx < len(items) {
				realIdx := len(items) - 1 - idx
				cfg := items[realIdx].Config
				m.SelectedConfig = &cfg // Signal parent
				return m, nil
			}
		}
	}

	m.Table, cmd = m.Table.Update(msg)
	return m, cmd
}

func (m HistoryView) View() string {
	s := strings.Builder{}
	s.WriteString(styles.Title.Render("📜 Past Runs"))
	s.WriteString("\n\n")

	// Check if table empty
	if len(m.Table.Rows()) == 0 {
		s.WriteString(styles.Subtle.Render("No history found.\nRun a test to generate data."))
	} else {
		s.WriteString(styles.Box.Render(m.Table.View()))
	}
	s.WriteString("\n\n")
	s.WriteString(styles.Subtle.Render("[Enter] Replay  [p] Export Selected"))
	return s.String()
}

func (m HistoryView) GetSelectedItem() *storage.HistoryItem {
	if m.Store == nil {
		return nil
	}
	idx := m.Table.Cursor()
	items := m.Store.List()

	// Handle reversed list
	if idx >= 0 && idx < len(items) {
		realIdx := len(items) - 1 - idx
		return &items[realIdx]
	}
	return nil
}
