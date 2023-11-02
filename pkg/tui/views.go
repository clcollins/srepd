package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func (m model) renderIncidentView() string {
	var s strings.Builder

	s.WriteString(fmt.Sprintf("%s\n", m.selectedIncident.Title))
	s.WriteString(renderHelpArea(m.help.View(defaultKeyMap)))

	return s.String()
}

func (m model) renderIncidentTable() string {
	var s strings.Builder
	var assignedTo string

	assignedTo = "Team"

	if m.toggleCurrentUserOnly {
		assignedTo = m.currentUser.Name
	}

	s.WriteString(renderAssigneeArea(assignedTo))
	s.WriteString(renderStatusArea(m.statusMessage))
	s.WriteString(renderTableArea(m.table))
	s.WriteString(renderHelpArea(m.help.View(defaultKeyMap)))

	return s.String()
}

func renderAssigneeArea(s string) string {
	// Gotta figure out how to accurately update the width on screen resize
	var style = lipgloss.NewStyle().
		Width(initialTableWidth).
		Height(1).
		Align(lipgloss.Right, lipgloss.Bottom).
		BorderStyle(lipgloss.HiddenBorder())

	var fstring = "Assigned to %s\n"
	return style.Render(fmt.Sprint(fstring, s))
}

func renderTableArea(t table.Model) string {
	var style = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	var fstring = "%s\n"
	return style.Render(fmt.Sprint(fstring, t.View()))
}

func renderStatusArea(s string) string {
	var style = lipgloss.NewStyle().
		Width(initialTableWidth).
		Height(1).
		Align(lipgloss.Left).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Bold(false)

	var fstring = "msg > %s\n"
	return style.Render(fmt.Sprintf(fstring, s))
}

// Gotta figure out how to accurately update the width on screen resize
var logArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(1).
	Align(lipgloss.Left).
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240")).
	Bold(false)

func renderHelpArea(s string) string {
	var style = lipgloss.NewStyle().
		Width(initialTableWidth).
		Height(1).
		Align(lipgloss.Right, lipgloss.Bottom).
		BorderStyle(lipgloss.HiddenBorder())

	return style.Render(s)
}

var incidentScreenArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(initialTableHeight+2).
	Align(lipgloss.Center, lipgloss.Center).
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240")).
	Bold(false)

var logScreenArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(initialTableHeight).
	Align(lipgloss.Left).
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240")).
	Bold(false)
