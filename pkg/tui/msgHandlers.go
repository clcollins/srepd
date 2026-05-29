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
	log.Error("tui.errMsgHandler()", "error", msg)
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

	// The incident viewer viewport has no border/padding/margin of its own;
	// all visual framing comes from tableContainerStyle wrapping the viewport.
	incidentHorizontalScratchWidth := 0
	incidentVerticalScratchWidth := 0

	tableHorizontalScratchWidth := tableHorizontalMargins + tableHorizontalPadding + tableHorizontalBorders + cellHorizontalPadding + cellHorizontalMargins + cellHorizontalBorders
	tableVerticalScratchWidth := tableVerticalMargins + tableVerticalPadding + tableVerticalBorders + cellVerticalPadding + cellVerticalMargins + cellVerticalBorders

	tableWidth := windowSize.Width - horizontalScratchWidth - tableHorizontalScratchWidth
	// Reserve lines for input field (1 line) and action log when visible (11 lines for 5-row table + spacing)
	// Additional spacing for help (needs ~12 lines when expanded with 10-item columns)
	inputReservedLines := 1
	additionalSpacing := 15
	actionLogReservedLines := 0
	if m.showActionLog {
		actionLogReservedLines = 11
	}
	tableHeight := windowSize.Height - verticalScratchWidth - tableVerticalScratchWidth - rowCount - estimatedExtraLinesFromComponents - actionLogReservedLines - inputReservedLines - additionalSpacing
	// table.SetHeight subtracts the rendered header height (2 lines: text + bottom border)
	// from the value we pass, so the minimum must exceed the header height to keep the
	// internal viewport height positive and avoid a panic in viewport.visibleLines
	if tableHeight < 4 {
		tableHeight = 4
	}

	m.table.SetHeight(tableHeight)

	// Action log table setup - fixed 6 rows
	actionLogHeight := 6
	m.actionLogTable.SetHeight(actionLogHeight)

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

	// Action log table columns: total width must match main table
	// First column needs width 2 to accommodate 2-char sequences like "^e", "%R"
	// We compensate by taking 1 from the last column
	// Total: 2 + 15 + columnWidth + (columnWidth-1) = 16 + 2*columnWidth = same as main table
	m.actionLogTable.SetColumns([]table.Column{
		{Title: " " + dot, Width: 2},               // Keypress column - space + dot like main table
		{Title: "ID", Width: idWidth - dotWidth},   // ID column (same as main table)
		{Title: "Summary", Width: columnWidth},     // Summary column (same as main table)
		{Title: "Service", Width: columnWidth - 1}, // Action column (1 less to compensate)
	})
	// Set explicit width to force table to respect column widths and not auto-size
	m.actionLogTable.SetWidth(tableWidth)

	m.incidentViewer.Width = windowSize.Width - horizontalScratchWidth - incidentHorizontalScratchWidth
	// Account for header (2 lines), footer (1 line), help (~2 lines), bottom status (1 line), and spacing
	reservedLines := 7 // header + footer + help + bottom status + borders/padding
	m.incidentViewer.Height = windowSize.Height - verticalScratchWidth - incidentVerticalScratchWidth - reservedLines
	if m.incidentViewer.Height < 10 {
		m.incidentViewer.Height = 10 // Minimum height
	}

	// Log viewer uses the same dimensions as the incident viewer
	m.logViewer.Width = m.incidentViewer.Width
	m.logViewer.Height = m.incidentViewer.Height

	m.help.Width = windowSize.Width - horizontalScratchWidth

	return m, nil
}

func (m model) keyMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If cluster selection mode is active, only accept digit keys and Escape/quit
	if m.clusterSelectMode {
		return m.handleClusterSelectInput(msg.(tea.KeyMsg))
	}

	// If a confirmation prompt is active, only accept y/n/Escape/quit
	if m.pendingConfirmation != nil {
		return m.handleConfirmationInput(msg.(tea.KeyMsg))
	}

	keyStr := msg.(tea.KeyMsg).String()

	// Chord state machine: runs before focus-mode dispatch so chords work
	// in both table and incident-view modes. Disabled during input and error modes
	// (those are handled below in the focus-mode switch).
	if !m.input.Focused() && m.err == nil {
		// 1. If chord pending and Escape: cancel chord
		if m.chordPending && keyStr == "esc" {
			m.chordPending = false
			m.setStatus("")
			return m, nil
		}

		// 2. If chord pending: resolve second key
		if m.chordPending {
			m.chordPending = false
			action := resolveChord(keyStr)
			if action != nil {
				return action.Handler(m)
			}
			m.setStatus(fmt.Sprintf("unknown chord: %s %s", m.chordPrefix, keyStr))
			return m, nil
		}

		// 3. If key matches prefix: enter chord mode
		if keyStr == m.chordPrefix {
			m.chordPending = true
			m.setStatus(fmt.Sprintf("%s ...", m.chordPrefix))
			return m, nil
		}
	}

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

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.ToggleActionLog) {
		m.showActionLog = !m.showActionLog
		return m, nil
	}

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Urgency) {
		m.showLowUrgency = !m.showLowUrgency
		return m, func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} }
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

	case m.viewingLog:
		return switchLogFocusMode(m, msg)

	case m.viewingIncident:
		return switchIncidentFocusMode(m, msg)

	case m.input.Focused():
		return switchInputFocusMode(m, msg)

	case m.table.Focused():
		return switchTableFocusMode(m, msg)
	}

	return m, nil
}

// handleClusterSelectInput processes keypresses while cluster selection mode is active.
// Accepts digit keys 1-9 to select a cluster, Escape to cancel, and quit keys to exit.
func (m model) handleClusterSelectInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, defaultKeyMap.Quit) {
		return m, tea.Quit
	}

	keyStr := msg.String()
	switch keyStr {
	case "esc":
		m.clusterSelectMode = false
		m.clusterSelectOptions = nil
		m.setStatus("cluster selection cancelled")
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(keyStr[0]-'0') - 1
		if idx < len(m.clusterSelectOptions) {
			selected := m.clusterSelectOptions[idx]
			m.clusterSelectMode = false
			m.clusterSelectOptions = nil
			return m, func() tea.Msg { return clusterSelectedMsg(selected) }
		}
		// Index out of range - ignore
		return m, nil
	default:
		// Ignore all other keys while cluster selection is active
		return m, nil
	}
}

// handleConfirmationInput processes keypresses while a confirmation prompt is active.
// Only 'y' (execute), 'n' (cancel), Escape (cancel), and quit keys are accepted.
func (m model) handleConfirmationInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, defaultKeyMap.Quit) {
		return m, tea.Quit
	}

	keyStr := msg.String()
	switch keyStr {
	case "y":
		// Execute the pending action
		action := m.pendingConfirmation.action
		m.pendingConfirmation = nil
		return m, action
	case "n":
		// Cancel the pending action
		m.pendingConfirmation = nil
		m.setStatus("action cancelled")
		return m, nil
	case "esc":
		// Cancel the pending action
		m.pendingConfirmation = nil
		m.setStatus("action cancelled")
		return m, nil
	default:
		// Ignore all other keys while confirmation is active
		return m, nil
	}
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
			if cmd := m.syncSelectedIncidentToHighlightedRow(); cmd != nil {
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, defaultKeyMap.Down):
			m.table.MoveDown(1)
			if cmd := m.syncSelectedIncidentToHighlightedRow(); cmd != nil {
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, defaultKeyMap.Top):
			m.table.GotoTop()
			if cmd := m.syncSelectedIncidentToHighlightedRow(); cmd != nil {
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, defaultKeyMap.Bottom):
			m.table.GotoBottom()
			if cmd := m.syncSelectedIncidentToHighlightedRow(); cmd != nil {
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, defaultKeyMap.Team):
			m.teamMode = !m.teamMode
			log.Debug("switchTableFocusMode", "teamMode", m.teamMode)
			cmds = append(cmds, func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} })

		case key.Matches(msg, defaultKeyMap.Refresh):
			m.clearSelectedIncident(msg.String() + " (refresh)")
			m.setStatus(loadingIncidentsStatus)
			cmds = append(cmds, updateIncidentList(m.config))

		// --- Incident action key handler pattern ---
		// Most actions that operate on an incident follow this normalized pattern:
		//   1. Check SelectedRow() == nil -> "no incident highlighted" (handles empty table)
		//   2. Call syncSelectedIncidentToHighlightedRow() to ensure selectedIncident
		//      matches the currently highlighted row (handles startup, list refresh, etc.)
		//   3. Check selectedIncident == nil -> "no incident selected" (edge case: ID not in list)
		//   4. Proceed with the action
		//
		// Login uses a different pattern (doIfIncidentSelected) because it needs to
		// fetch full incident data from the API before proceeding.

		case key.Matches(msg, defaultKeyMap.Enter):
			// Clear any pending confirmation on view transition
			m.pendingConfirmation = nil
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
			m.syncSelectedIncidentToHighlightedRow()
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			m.pendingConfirmation = &confirmActionState{
				prompt: fmt.Sprintf("Silence %s? [y/n]", incidentID),
				action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
			}
			return m, nil

		case key.Matches(msg, defaultKeyMap.Ack):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			m.syncSelectedIncidentToHighlightedRow()
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{} }

		case key.Matches(msg, defaultKeyMap.UnAck):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			m.syncSelectedIncidentToHighlightedRow()
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			m.pendingConfirmation = &confirmActionState{
				prompt: fmt.Sprintf("Re-escalate %s? [y/n]", incidentID),
				action: func() tea.Msg { return unAcknowledgeIncidentsMsg{} },
			}
			return m, nil

		case key.Matches(msg, defaultKeyMap.Note):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			m.syncSelectedIncidentToHighlightedRow()
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			return m, parseTemplateForNote(m.selectedIncident)

		case key.Matches(msg, defaultKeyMap.Login):
			// Login uses doIfIncidentSelected() instead of the standard sync pattern
			// because it needs to fetch full incident details + alerts from the API
			// before proceeding (via getIncidentMsg + waitForSelectedIncidentThenDoMsg).
			return m, doIfIncidentSelected(&m, func() tea.Msg {
				return waitForSelectedIncidentThenDoMsg{action: func() tea.Msg { return loginMsg("login") }, msg: "wait"}
			})

		case key.Matches(msg, defaultKeyMap.Open):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			m.syncSelectedIncidentToHighlightedRow()
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			// HTMLURL should be in stub data from incident list, but check to be safe
			if m.selectedIncident.HTMLURL == "" {
				m.setStatus("incident URL not available")
				return m, nil
			}
			if defaultBrowserOpenCommand == "" {
				return m, func() tea.Msg { return errMsg{fmt.Errorf("unsupported OS: no browser open command available")} }
			}

			// TEMPORARY: Log open action to action log for testing
			m.addActionLogEntry("o", m.selectedIncident.ID, m.selectedIncident.Title, m.selectedIncident.Service.Summary)

			c := []string{defaultBrowserOpenCommand}
			return m, openBrowserCmd(c, m.selectedIncident.HTMLURL)

		case key.Matches(msg, defaultKeyMap.SOP):
			if !m.incidentAlertsLoaded {
				m.setStatus("Loading incident alerts, please wait...")
				return m, nil
			}
			link, ok := getSOPLink(m.selectedIncidentAlerts)
			if !ok {
				m.setStatus("no SOP link found")
				return m, nil
			}
			if defaultBrowserOpenCommand == "" {
				return m, func() tea.Msg { return errMsg{fmt.Errorf("unsupported OS: no browser open command available")} }
			}

			m.addActionLogEntry("s", m.selectedIncident.ID, m.selectedIncident.Title, m.selectedIncident.Service.Summary)

			c := []string{defaultBrowserOpenCommand}
			return m, openBrowserCmd(c, link)

		case key.Matches(msg, defaultKeyMap.ViewLog):
			return m, readLogFile(m.logFilePath)

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
			// Extract the input text, reset input, and dispatch to Claude
			prompt := m.input.Value()
			m.input.Reset()
			m.input.Blur()
			m.table.Focus()

			// Don't dispatch on empty input
			if prompt == "" {
				return m, nil
			}

			return m, func() tea.Msg {
				return claudePromptMsg{prompt: prompt}
			}

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
			m.table.Focus()                                         // Ensure table regains focus immediately
			prefetchCmd := m.syncSelectedIncidentToHighlightedRow() // Re-establish selection to current cursor position
			// Return immediately - no need to process anything else or update viewport
			return m, prefetchCmd

		// Tab navigation: switch between Details/Alerts/Notes tabs
		case key.Matches(msg, defaultKeyMap.TabNext):
			m.activeTab = (m.activeTab + 1) % tabCount
			m.incidentViewer.GotoTop()
			return m, func() tea.Msg { return renderIncidentMsg("tab switch") }

		case key.Matches(msg, defaultKeyMap.TabPrev):
			m.activeTab = (m.activeTab + tabCount - 1) % tabCount
			m.incidentViewer.GotoTop()
			return m, func() tea.Msg { return renderIncidentMsg("tab switch") }

		// Item navigation: left/right within Alerts/Notes tabs
		case key.Matches(msg, defaultKeyMap.ItemNext):
			switch m.activeTab {
			case tabAlerts:
				if len(m.selectedIncidentAlerts) > 0 {
					m.activeAlertIdx = (m.activeAlertIdx + 1) % len(m.selectedIncidentAlerts)
					m.incidentViewer.GotoTop()
					return m, func() tea.Msg { return renderIncidentMsg("alert next") }
				}
			case tabNotes:
				if len(m.selectedIncidentNotes) > 0 {
					m.activeNoteIdx = (m.activeNoteIdx + 1) % len(m.selectedIncidentNotes)
					m.incidentViewer.GotoTop()
					return m, func() tea.Msg { return renderIncidentMsg("note next") }
				}
			}
			return m, nil

		case key.Matches(msg, defaultKeyMap.ItemPrev):
			handledKey = true
			switch m.activeTab {
			case tabAlerts:
				if len(m.selectedIncidentAlerts) > 0 {
					m.activeAlertIdx = (m.activeAlertIdx + len(m.selectedIncidentAlerts) - 1) % len(m.selectedIncidentAlerts)
					m.incidentViewer.GotoTop()
					return m, func() tea.Msg { return renderIncidentMsg("alert prev") }
				}
			case tabNotes:
				if len(m.selectedIncidentNotes) > 0 {
					m.activeNoteIdx = (m.activeNoteIdx + len(m.selectedIncidentNotes) - 1) % len(m.selectedIncidentNotes)
					m.incidentViewer.GotoTop()
					return m, func() tea.Msg { return renderIncidentMsg("note prev") }
				}
			}
			return m, nil

		case key.Matches(msg, defaultKeyMap.Refresh):
			return m, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) }

		case key.Matches(msg, defaultKeyMap.Ack):
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{} }

		case key.Matches(msg, defaultKeyMap.UnAck):
			incidentID := ""
			if m.selectedIncident != nil {
				incidentID = m.selectedIncident.ID
			}
			m.pendingConfirmation = &confirmActionState{
				prompt: fmt.Sprintf("Re-escalate %s? [y/n]", incidentID),
				action: func() tea.Msg { return unAcknowledgeIncidentsMsg{} },
			}
			return m, nil

		case key.Matches(msg, defaultKeyMap.Silence):
			incidentID := ""
			if m.selectedIncident != nil {
				incidentID = m.selectedIncident.ID
			}
			m.pendingConfirmation = &confirmActionState{
				prompt: fmt.Sprintf("Silence %s? [y/n]", incidentID),
				action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
			}
			return m, nil

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

		case key.Matches(msg, defaultKeyMap.SOP):
			// SOP link requires alerts to extract the link field
			if !m.incidentAlertsLoaded {
				m.setStatus("Loading incident alerts, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return openSOPMsg("sop") }

		default:
			// Number keys 1-9 jump to specific alert/note index
			if m.activeTab == tabAlerts || m.activeTab == tabNotes {
				keyStr := msg.String()
				if len(keyStr) == 1 && keyStr[0] >= '1' && keyStr[0] <= '9' {
					idx := int(keyStr[0]-'0') - 1
					switch m.activeTab {
					case tabAlerts:
						if idx < len(m.selectedIncidentAlerts) {
							m.activeAlertIdx = idx
							m.incidentViewer.GotoTop()
							handledKey = true
							return m, func() tea.Msg { return renderIncidentMsg("alert jump") }
						}
					case tabNotes:
						if idx < len(m.selectedIncidentNotes) {
							m.activeNoteIdx = idx
							m.incidentViewer.GotoTop()
							handledKey = true
							return m, func() tea.Msg { return renderIncidentMsg("note jump") }
						}
					}
					// Out of range -- ignore
					handledKey = true
					return m, nil
				}
			}
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

func switchLogFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	handledKey := false

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Back):
			m.viewingLog = false
			m.table.Focus()
			return m, nil

		case key.Matches(msg, defaultKeyMap.Help):
			m.toggleHelp()
			handledKey = true
		}
	}

	if !handledKey {
		m.logViewer, cmd = m.logViewer.Update(msg)
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
