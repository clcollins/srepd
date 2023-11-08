package tui

import "github.com/charmbracelet/bubbles/table"

const (
	idColumnWidth      = 16
	defaultColumnWidth = 32
)

var (
	incidentViewColumns = []table.Column{
		// Currently the dot column is not used
		// but may be useful for selecting multiple incidents
		// TODO: Figure out some way to update these columns on resize
		{Title: dot, Width: 1},
		{Title: "ID", Width: idColumnWidth},
		{Title: "Summary"},
		{Title: "ClusterID"},
	}
)
