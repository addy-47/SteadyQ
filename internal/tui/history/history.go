package history

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"steadyq/internal/storage"
	"steadyq/internal/tui/styles"
)

type Model struct {
	Store *storage.Store
	Table table.Model

	Width  int
	Height int
}

func NewModel(store *storage.Store) Model {
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
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := Model{
		Store: store,
		Table: t,
	}
	m.Refresh()
	return m
}

func (m *Model) Refresh() {
	items := m.Store.List()
	rows := make([]table.Row, len(items))

	for i, item := range items {
		rows[i] = table.Row{
			item.Timestamp.Format(time.RFC822),
			item.Config.URL,
			fmt.Sprintf("%d", item.Config.TargetRPS),
			fmt.Sprintf("%d", item.Summary.TotalRequests),
			fmt.Sprintf("%d", item.Summary.Success),
		}
	}
	m.Table.SetRows(rows)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Table.SetWidth(msg.Width - 4)

	// Handle keys for table navigation...
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Replay? Emit event?
			// For now just placeholders
		}
	}

	m.Table, cmd = m.Table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	return styles.Box.Render(m.Table.View())
}
