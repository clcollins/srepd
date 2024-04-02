package style

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

const (
	Gray       = lipgloss.Color("240")
	PaleYellow = lipgloss.Color("229")
	NeonPurple = lipgloss.Color("57")
	Lilac      = lipgloss.Color("105")
)

var (
	HorizontalPadding = 1

	borderWidth = 1

	Main = lipgloss.NewStyle().Margin(1, 0).Padding(0, HorizontalPadding)

	Assignee = Main.Copy()

	Status = Main.Copy()

	AssignedStringWidth = len("Assigned to User") + (HorizontalPadding * 2 * 2) + (borderWidth * 2 * 2) + 10

	TableContainer = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(Gray)

	Table = table.Styles{
		Selected: lipgloss.NewStyle().Bold(true).Foreground(PaleYellow).Background(NeonPurple),
		Header:   lipgloss.NewStyle().Bold(false).Padding(0, 1).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(Gray)).BorderBottom(true),
		Cell:     lipgloss.NewStyle().Padding(0, 1),
	}

	Help = lipgloss.NewStyle().Foreground(Lilac)

	IncidentViewer = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(Gray)).Padding(2)

	Error = lipgloss.NewStyle().
		Bold(true).
		Width(64).
		Foreground(lipgloss.AdaptiveColor{Light: "#E11C9C", Dark: "#FF62DA"}).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#E11C9C", Dark: "#FF62DA"}).
		Padding(1, 3, 1, 3)
)
