package tui

import (
	"fmt"
	"math"
	"reflect"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

// setStatusMsgHandler is the message handler for the setStatusMsg message
func (m model) setStatusMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.setStatus(msg.(setStatusMsg).Status())
	return m, nil
}

// errMsgHandler is the message handler for the errMsg message
func (m model) errMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Error("errMsgHandler", "tea.errMsg", msg)
	m.setStatus(msg.(errMsg).Error())
	m.err = msg.(errMsg)
	return m, nil
}

// windowSizeMsgHandler is the message handler for the windowSizeMsg message
// and resizes the tui according to the new terminal window size
func (m model) windowSizeMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	windowSize = msg.(tea.WindowSizeMsg)
	rowCount := func(m model) int {
		// This func sets the table height to a reasonable size
		// in the case there are no incidents
		if len(m.table.Rows()) > 0 {
			return len(m.table.Rows())
		}
		return 10
	}(m)

	verticalMargins := mainStyle.GetVerticalMargins()
	horizontalMargins := mainStyle.GetHorizontalMargins()
	verticalPadding := mainStyle.GetVerticalPadding()
	horizontalPadding := mainStyle.GetHorizontalPadding()
	verticalBorders := mainStyle.GetVerticalBorderSize()
	horizontalBorders := mainStyle.GetHorizontalBorderSize()

	tableVerticalMargins := tableContainerStyle.GetVerticalMargins()
	tableHorizontalMargins := tableContainerStyle.GetHorizontalMargins()
	tableVerticalPadding := tableContainerStyle.GetVerticalPadding()
	tableHorizontalPadding := tableContainerStyle.GetHorizontalPadding()
	tableVerticalBorders := tableContainerStyle.GetVerticalBorderSize()
	tableHorizontalBorders := tableContainerStyle.GetHorizontalBorderSize()

	cellVerticalPadding := tableCellStyle.GetVerticalPadding() * rowCount    // Number of rows
	cellHorizontalPadding := tableCellStyle.GetHorizontalPadding() * 4       // Four columns
	cellVerticalMargins := tableCellStyle.GetVerticalMargins() * rowCount    // Number of rows
	cellHorizontalMargins := tableCellStyle.GetHorizontalMargins() * 4       // Four columns
	cellVerticalBorders := tableCellStyle.GetVerticalBorderSize() * rowCount // Number of rows
	cellHorizontalBorders := tableCellStyle.GetHorizontalBorderSize() * 4    // Four columns

	estimatedExtraLinesFromComponents := 7 // TODO: figure out how to calculate this

	horizontalScratchWidth := horizontalMargins + horizontalPadding + horizontalBorders
	verticalScratchWidth := verticalMargins + verticalPadding + verticalBorders

	incidentHorizontalScratchWidth := incidentViewerStyle.GetHorizontalMargins() + incidentViewerStyle.GetHorizontalPadding() + incidentViewerStyle.GetHorizontalBorderSize()
	incidentVerticalScratchWidth := incidentViewerStyle.GetVerticalMargins() + incidentViewerStyle.GetVerticalPadding() + incidentViewerStyle.GetVerticalBorderSize()

	tableHorizontalScratchWidth := tableHorizontalMargins + tableHorizontalPadding + tableHorizontalBorders + cellHorizontalPadding + cellHorizontalMargins + cellHorizontalBorders
	tableVerticalScratchWidth := tableVerticalMargins + tableVerticalPadding + tableVerticalBorders + cellVerticalPadding + cellVerticalMargins + cellVerticalBorders

	tableWidth := windowSize.Width - horizontalScratchWidth - tableHorizontalScratchWidth
	tableHeight := windowSize.Height - verticalScratchWidth - tableVerticalScratchWidth - rowCount - estimatedExtraLinesFromComponents

	m.table.SetHeight(tableHeight)

	// converting to floats, rounding up and converting back to int handles layout issues arising from odd numbers
	columnWidth := int(math.Ceil(float64(tableWidth-idWidth-dotWidth) / float64(2)))

	log.Debug("tui.windowSizeMsgHandler",
		"window_width", windowSize.Width,
		"window_height", windowSize.Height,
		"table_horizontal_scratch_width", tableHorizontalScratchWidth,
		"table_vertical_scratch_width", tableVerticalScratchWidth,
		"incident_horizontal_scratch_width", incidentHorizontalScratchWidth,
		"incident_vertical_scratch_width", incidentVerticalScratchWidth,
		"horizontal_scratch_width", horizontalScratchWidth,
		"vertical_scratch_width", verticalScratchWidth,
		"table_width", tableWidth,
		"table_height", tableHeight,
		"column_width", columnWidth,
	)

	m.table.SetColumns([]table.Column{
		{Title: dot, Width: dotWidth},
		{Title: "ID", Width: idWidth - dotWidth},
		{Title: "Summary", Width: columnWidth},
		{Title: "Service", Width: columnWidth},
	})

	m.incidentViewer.Width = windowSize.Width - horizontalScratchWidth - incidentHorizontalScratchWidth
	// Account for header (2 lines), footer (1 line), help (~2 lines), bottom status (1 line), and spacing
	reservedLines := 7 // header + footer + help + bottom status + borders/padding
	m.incidentViewer.Height = windowSize.Height - verticalScratchWidth - incidentVerticalScratchWidth - reservedLines
	if m.incidentViewer.Height < 10 {
		m.incidentViewer.Height = 10 // Minimum height
	}

	m.help.Width = windowSize.Width - horizontalScratchWidth

	return m, nil
}

func (m model) keyMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Quit) {
		return m, tea.Quit
	}

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.AutoRefresh) {
		m.autoRefresh = !m.autoRefresh
		return m, updateIncidentList(m.config)
	}

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.AutoAck) {
		m.autoAcknowledge = !m.autoAcknowledge
		return m, nil
	}

	// Commands for any focus mode
	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Input) {
		return m, tea.Sequence(
			m.input.Focus(),
		)
	}

	// Default commands for the table view
	switch {
	case m.err != nil:
		return switchErrorFocusMode(m, msg)

	case m.viewingIncident:
		return switchIncidentFocusMode(m, msg)

	case m.input.Focused():
		return switchInputFocusMode(m, msg)

	case m.table.Focused():
		return switchTableFocusMode(m, msg)
	}

	return m, nil
}

// tableFocusMode is the main mode for the application
func switchTableFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// [1] is column two of the row: the incident ID
	var row table.Row
	var incidentID string
	row = m.table.SelectedRow()
	if row == nil {
		incidentID = ""
	} else {
		incidentID = m.table.SelectedRow()[1]
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.toggleHelp()

		case key.Matches(msg, defaultKeyMap.Up):
			m.table.MoveUp(1)

		case key.Matches(msg, defaultKeyMap.Down):
			m.table.MoveDown(1)

		case key.Matches(msg, defaultKeyMap.Top):
			m.table.GotoTop()

		case key.Matches(msg, defaultKeyMap.Bottom):
			m.table.GotoBottom()

		case key.Matches(msg, defaultKeyMap.Team):
			m.teamMode = !m.teamMode
			log.Debug("switchTableFocusMode", "teamMode", m.teamMode)
			cmds = append(cmds, func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} })

		case key.Matches(msg, defaultKeyMap.Refresh):
			m.clearSelectedIncident(msg.String() + " (refresh)")
			m.setStatus(loadingIncidentsStatus)
			cmds = append(cmds, updateIncidentList(m.config))

		// In table mode, highlighted incidents are not selected yet, so they need to be retrieved
		// and then can be acted upon.  Since tea.Sequence does not wait for completion, the
		// "waitForSelectedIncidentsThen..." functions are used to wait for the selected incident
		// to be retrieved from PagerDuty
		case key.Matches(msg, defaultKeyMap.Enter):
			// Check if we have cached data for this incident
			if cached, exists := m.incidentCache[incidentID]; exists {
				// Use cached data immediately
				if cached.incident != nil {
					m.selectedIncident = cached.incident
					m.incidentDataLoaded = cached.dataLoaded
				} else {
					m.selectedIncident = &pagerduty.Incident{
						APIObject: pagerduty.APIObject{
							ID:      incidentID,
							Summary: "Loading incident details...",
						},
					}
					m.incidentDataLoaded = false
				}

				if cached.notes != nil {
					m.selectedIncidentNotes = cached.notes
					m.incidentNotesLoaded = cached.notesLoaded
				} else {
					m.incidentNotesLoaded = false
				}

				if cached.alerts != nil {
					m.selectedIncidentAlerts = cached.alerts
					m.incidentAlertsLoaded = cached.alertsLoaded
				} else {
					m.incidentAlertsLoaded = false
				}
			} else {
				// No cache - show loading placeholder
				m.selectedIncident = &pagerduty.Incident{
					APIObject: pagerduty.APIObject{
						ID:      incidentID,
						Summary: "Loading incident details...",
					},
				}
				m.incidentDataLoaded = false
				m.incidentNotesLoaded = false
				m.incidentAlertsLoaded = false
			}

			m.viewingIncident = true
			// Reset viewport to top when opening incident
			m.incidentViewer.GotoTop()
			// Render with whatever data we have (cached or loading placeholder)
			// Then refresh from PagerDuty in the background
			return m, tea.Sequence(
				func() tea.Msg { return renderIncidentMsg("Render incident: " + incidentID) },
				func() tea.Msg { return getIncidentMsg(incidentID) },
			)

		case key.Matches(msg, defaultKeyMap.Silence):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			return m, func() tea.Msg { return silenceSelectedIncidentMsg{} }

		case key.Matches(msg, defaultKeyMap.Ack):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{} }

		case key.Matches(msg, defaultKeyMap.UnAck):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			return m, func() tea.Msg { return unAcknowledgeIncidentsMsg{} }

		case key.Matches(msg, defaultKeyMap.Note):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			incident := m.getHighlightedIncident()
			if incident == nil {
				m.setStatus("failed to find incident")
				return m, nil
			}
			return m, parseTemplateForNote(incident)

		case key.Matches(msg, defaultKeyMap.Login):
			return m, doIfIncidentSelected(&m, func() tea.Msg {
				return waitForSelectedIncidentThenDoMsg{action: func() tea.Msg { return loginMsg("login") }, msg: "wait"}
			})

		case key.Matches(msg, defaultKeyMap.Open):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			incident := m.getHighlightedIncident()
			if incident == nil {
				m.setStatus("failed to find incident")
				return m, nil
			}
			if defaultBrowserOpenCommand == "" {
				return m, func() tea.Msg { return errMsg{fmt.Errorf("unsupported OS: no browser open command available")} }
			}
			c := []string{defaultBrowserOpenCommand}
			return m, openBrowserCmd(c, incident.HTMLURL)

		}
	}
	return m, tea.Batch(cmds...)
}

func switchInputFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Quit):
			// Ctrl+q/Ctrl+c quits the application
			return m, tea.Quit

		case key.Matches(msg, defaultKeyMap.Back):
			// Esc exits input mode
			m.input.Blur()
			m.table.Focus()
			m.input.Reset() // Clear the input text
			return m, nil

		case key.Matches(msg, defaultKeyMap.Enter):
			// For now, do nothing on Enter
			// Future: process the input command
			return m, nil

		default:
			// Pass ALL other keypresses (including 'h') to the input component
			// This allows text entry and disables all other key bindings
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func switchIncidentFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Track if we handled the key ourselves
	handledKey := false

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.toggleHelp()
			handledKey = true

		// This un-sets the selected incident and returns to the table view
		case key.Matches(msg, defaultKeyMap.Back):
			m.clearSelectedIncident(msg.String() + " (back)")
			m.table.Focus()  // Ensure table regains focus immediately
			// Return immediately - no need to process anything else or update viewport
			return m, nil

		case key.Matches(msg, defaultKeyMap.Refresh):
			return m, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) }

		case key.Matches(msg, defaultKeyMap.Ack):
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{} }

		case key.Matches(msg, defaultKeyMap.UnAck):
			return m, func() tea.Msg { return unAcknowledgeIncidentsMsg{} }

		case key.Matches(msg, defaultKeyMap.Silence):
			return m, func() tea.Msg { return silenceSelectedIncidentMsg{} }

		case key.Matches(msg, defaultKeyMap.Note):
			// Note template requires full incident data (HTMLURL, Title, Service.Summary)
			if !m.incidentDataLoaded {
				m.setStatus("Loading incident details, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return parseTemplateForNoteMsg("add note") }

		case key.Matches(msg, defaultKeyMap.Login):
			// Login requires alerts to extract cluster_id
			if !m.incidentAlertsLoaded {
				m.setStatus("Loading incident alerts, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return loginMsg("login") }

		case key.Matches(msg, defaultKeyMap.Open):
			// Browser open requires HTMLURL from full incident data
			if !m.incidentDataLoaded {
				m.setStatus("Loading incident details, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return openBrowserMsg("incident") }

		}
	}

	// Only pass the message to the viewport if we didn't handle it as a key command
	// This prevents the viewport from consuming ESC and other navigation keys
	if !handledKey {
		m.incidentViewer, cmd = m.incidentViewer.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func switchErrorFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Debug("switchErrorFocusMode", reflect.TypeOf(msg), msg)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Back):
			m.err = nil
		}
	}
	return m, nil
}
