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
		{Title: "URL", Width: 30},
		{Title: "RPS", Width: 10},
		{Title: "Reqs", Width: 10},
		{Title: "Success", Width: 10},
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
		Bold(false)
	s.Selected = s.Selected.
		Foreground(styles.ColorBg).
		Background(styles.ColorPrimary).
		Bold(false)
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
	rows := make([]table.Row, len(items))

	for i, item := range items {
		rows[i] = table.Row{
			item.Timestamp.Format("02 Jan 15:04"),
			item.Config.URL,
			fmt.Sprintf("%d", item.Config.TargetRPS),
			fmt.Sprintf("%d", item.Summary.TotalRequests),
			fmt.Sprintf("%d", item.Summary.Success),
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
		m.Table.SetWidth(msg.Width)
		m.Table.SetHeight(msg.Height - 5) // Reserve space for header

	case tea.KeyMsg:
		if msg.String() == "enter" {
			// Select item
			idx := m.Table.Cursor()
			items := m.Store.List()
			if idx >= 0 && idx < len(items) {
				cfg := items[idx].Config
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
	s.WriteString(styles.Box.Render(m.Table.View()))
	s.WriteString("\n\n")
	s.WriteString(styles.Subtle.Render("[Enter] Replay Selected"))
	return s.String()
}
