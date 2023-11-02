package tui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	initialTableHeight = 20
	initialTableWidth  = 106
)

var (
	incidentListTableColumns = []table.Column{
		{Title: dot, Width: 2},
		{Title: "ID", Width: 16},
		{Title: "Summary", Width: 64},
		{Title: "ClusterID", Width: 16},
	}

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
)

type createTableWithStylesMsg string
type createdTableWithStylesMsg struct {
	// Not sure this is the right way to do this
	// Why can I not just pass the table.Model?
	// Because it's a struct?
	table table.Model
}

func createTableWithStyles() tea.Cmd {
	return func() tea.Msg {
		t := table.New(
			table.WithColumns(incidentListTableColumns),
			table.WithRows([]table.Row{}),
			table.WithFocused(true),
			table.WithHeight(initialTableHeight),
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
		return createdTableWithStylesMsg(createdTableWithStylesMsg{table: t})
	}
}
