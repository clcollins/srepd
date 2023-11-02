package tui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TODO: Figure out how to dynamically resize the table and columns based on terminal size
// and resize tea.Msg events received from the terminal
const (
	// Standard terminal size is 80x24
	// so we have 70 columns to work with after subtracting for the table borders (2)
	// and 2 for each column's cellpadding (8)
	// and 2 more for the terminal scroll bars
	// For rows: 20 subtracting table borders, status messages, and help
	initialTableWidth  = dotWidth + idWidth + summaryWidth + clusterIDWidth - scrollbars
	initialTableHeight = 18
	dotWidth           = 2
	idWidth            = 16 // Looks like most PD alerts are 14 characters
	summaryWidth       = 36
	clusterIDWidth     = 16 // ClusterID and UUIDs are 32 characters; Display Names tend to be ~16

	// Basic stuff (subtract 2 for scroll bars)
	defaultTerminalWidth = 80 - scrollbars
	scrollbars           = 2
	initialViewPortWidth = defaultTerminalWidth
)

var (
	incidentListTableColumns = []table.Column{
		// Currently the dot column is not used
		// but may be useful for selecting multiple incidents
		{Title: dot, Width: dotWidth},
		{Title: "ID", Width: idWidth},
		{Title: "Summary", Width: summaryWidth},
		{Title: "ClusterID", Width: clusterIDWidth},
	}
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
