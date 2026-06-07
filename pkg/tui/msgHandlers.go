package tui

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
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

	verticalMargins := m.styles.Main.GetVerticalMargins()
	horizontalMargins := m.styles.Main.GetHorizontalMargins()
	verticalPadding := m.styles.Main.GetVerticalPadding()
	horizontalPadding := m.styles.Main.GetHorizontalPadding()
	verticalBorders := m.styles.Main.GetVerticalBorderSize()
	horizontalBorders := m.styles.Main.GetHorizontalBorderSize()

	tableVerticalMargins := m.styles.TableContainer.GetVerticalMargins()
	tableHorizontalMargins := m.styles.TableContainer.GetHorizontalMargins()
	tableVerticalPadding := m.styles.TableContainer.GetVerticalPadding()
	tableHorizontalPadding := m.styles.TableContainer.GetHorizontalPadding()
	tableVerticalBorders := m.styles.TableContainer.GetVerticalBorderSize()
	tableHorizontalBorders := m.styles.TableContainer.GetHorizontalBorderSize()

	cellStyle := m.styles.Table.Cell
	cellVerticalPadding := cellStyle.GetVerticalPadding() * rowCount
	cellHorizontalPadding := cellStyle.GetHorizontalPadding() * 4
	cellVerticalMargins := cellStyle.GetVerticalMargins() * rowCount
	cellHorizontalMargins := cellStyle.GetHorizontalMargins() * 4
	cellVerticalBorders := cellStyle.GetVerticalBorderSize() * rowCount
	cellHorizontalBorders := cellStyle.GetHorizontalBorderSize() * 4

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
	// Reserve lines for input field (1 line)
	// Additional spacing for help (needs ~12 lines when expanded with 10-item columns)
	inputReservedLines := 1
	additionalSpacing := 15
	tableHeight := windowSize.Height - verticalScratchWidth - tableVerticalScratchWidth - rowCount - estimatedExtraLinesFromComponents - inputReservedLines - additionalSpacing
	// table.SetHeight subtracts the rendered header height (2 lines: text + bottom border)
	// from the value we pass, so the minimum must exceed the header height to keep the
	// internal viewport height positive and avoid a panic in viewport.visibleLines
	if tableHeight < 4 {
		tableHeight = 4
	}

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

	tabWindowBorders := m.styles.TabWindow.GetHorizontalBorderSize()
	m.incidentViewer.Width = windowSize.Width - horizontalScratchWidth - incidentHorizontalScratchWidth - tabWindowBorders
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

	if m.viewingIncident {
		return m, func() tea.Msg { return renderIncidentMsg("window resized") }
	}

	if m.configWizardPending != nil {
		pending := *m.configWizardPending
		m.configWizardPending = nil
		return m.Update(pending)
	}

	return m, nil
}

func (m model) keyMsgHandler(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Cluster selection mode is handled as a view in the switch below

	// If a confirmation prompt is active, only accept y/n/Escape/quit
	if m.pendingConfirmation != nil {
		return m.handleConfirmationInput(msg.(tea.KeyMsg))
	}

	// Clear chord help overlay on any keypress so the regular help returns
	if m.chordHelpActive {
		m.chordHelpActive = false
	}

	keyStr := msg.(tea.KeyMsg).String()

	// Chord state machine: runs before focus-mode dispatch so chords work
	// in both table and incident-view modes. Disabled during input and error modes
	// (those are handled below in the focus-mode switch).
	if !m.input.Focused() && m.err == nil && !m.clusterSelectMode {
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
	case m.configMode:
		return switchConfigFocusMode(m, msg)

	case m.teamSelectMode:
		return switchTeamSelectFocusMode(m, msg)

	case m.err != nil:
		return switchErrorFocusMode(m, msg)

	case m.clusterSelectMode:
		return switchClusterSelectFocusMode(m, msg)

	case m.mergeMode:
		return switchMergeFocusMode(m, msg)

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

func switchTeamSelectFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.teamSelectForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.teamSelectForm = f
	}
	if m.teamSelectForm.State == huh.StateCompleted {
		m.teamSelectMode = false
		m.table.Focus()
		selected := make([]string, len(m.teamSelectIDs))
		copy(selected, m.teamSelectIDs)
		names := make(map[string]string)
		for k, v := range m.teamSelectNames {
			names[k] = v
		}
		return m, func() tea.Msg { return teamsSelectedMsg{ids: selected, names: names} }
	}
	if m.teamSelectForm.State == huh.StateAborted {
		m.teamSelectMode = false
		m.table.Focus()
		m.setStatus("team selection skipped")
		return m, nil
	}
	return m, cmd
}

func switchConfigFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.configForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.configForm = f
	}
	if m.configForm.State == huh.StateCompleted {
		m.configMode = false
		m.configModeRequested = false
		m.table.Focus()

		if !m.configState.Confirm {
			m.setStatus("config changes discarded")
			return m, func() tea.Msg { return updateIncidentListMsg("config discarded") }
		}

		final, err := pkgconfig.ResolveFinalValues(m.configExisting, pkgconfig.WizardInputs{
			TokenInput:          m.configState.TokenInput,
			SelectedTeams:       m.configState.SelectedTeams,
			SilentPolicyID:      m.configState.SilentPolicy,
			CustomMappingsInput: m.configState.CustomInput,
			KeepTeams:           m.configState.KeepTeams,
			KeepSilent:          m.configState.KeepSilent,
			KeepCustom:          m.configState.KeepCustom,
		})
		if err != nil {
			m.setStatus("config error: " + err.Error())
			return m, nil
		}

		var changes pkgconfig.ConfigChanges
		if m.configIsNewFile {
			changes = pkgconfig.DetectChangesForNewFile(final)
		} else {
			changes = pkgconfig.DetectChanges(m.configExisting, final, strings.TrimSpace(m.configState.TokenInput))
		}

		if m.configExisting.OldFormatDetected {
			if final.SilentPolicy != "" {
				changes.SilentChanged = true
			}
			if final.CustomMappingsInput != "" {
				changes.CustomChanged = true
			}
		}

		if !changes.AnyChanged() {
			m.setStatus("config is valid, no changes needed")
			return m, func() tea.Msg { return updateIncidentListMsg("config unchanged") }
		}

		teamNames := make(map[string]string)
		for k, v := range m.configTeamNames {
			teamNames[k] = v
		}

		// Parse custom policies
		var customPolicies map[string]string
		if changes.CustomChanged && final.CustomMappingsInput != "" {
			parsed, parseErr := pkgconfig.ParseCustomMappingsStrict(final.CustomMappingsInput)
			if parseErr != nil {
				m.setStatus("config error: " + parseErr.Error())
				return m, nil
			}
			customPolicies = parsed
		}

		return m, func() tea.Msg {
			return configCompletedMsg{
				final:          final,
				changes:        changes,
				teamNames:      teamNames,
				customPolicies: customPolicies,
				isNewFile:      m.configIsNewFile,
			}
		}
	}
	if m.configForm.State == huh.StateAborted {
		m.configMode = false
		m.configModeRequested = false
		m.table.Focus()
		m.setStatus("config cancelled")
		return m, nil
	}
	return m, cmd
}

func switchClusterSelectFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, defaultKeyMap.Back):
			m.clusterSelectMode = false
			m.clusterSelectOptions = nil
			m.setStatus("cluster selection cancelled")
			m.table.Focus()
			return m, nil

		case key.Matches(msg, defaultKeyMap.Enter):
			selectedRow := m.clusterSelectTable.SelectedRow()
			if len(selectedRow) < 1 {
				m.setStatus("no cluster selected")
				return m, nil
			}
			selected := selectedRow[0]
			m.clusterSelectMode = false
			m.clusterSelectOptions = nil
			m.table.Focus()
			return m, func() tea.Msg { return clusterSelectedMsg(selected) }

		default:
			var cmd tea.Cmd
			m.clusterSelectTable, cmd = m.clusterSelectTable.Update(msg)
			return m, cmd
		}
	}
	return m, nil
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
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
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

		case key.Matches(msg, defaultKeyMap.Merge):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			m.syncSelectedIncidentToHighlightedRow()
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			m.mergeMode = true
			m.mergeSourceIncident = m.selectedIncident
			m.mergeTeamMode = m.teamMode
			m.mergeTable = newTableWithStyles()
			m.rebuildMergeTable()
			return m, nil

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

			c := []string{defaultBrowserOpenCommand}
			return m, tea.Batch(m.flashNotification(fmt.Sprintf("Opened %s in browser", m.selectedIncident.ID)), openBrowserCmd(c, m.selectedIncident.HTMLURL))

		case key.Matches(msg, defaultKeyMap.SOP):
			if m.table.SelectedRow() == nil {
				m.setStatus("no incident highlighted")
				return m, nil
			}
			m.syncSelectedIncidentToHighlightedRow()
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
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

			c := []string{defaultBrowserOpenCommand}
			return m, tea.Batch(m.flashNotification(fmt.Sprintf("Opened SOP for %s", m.selectedIncident.ID)), openBrowserCmd(c, link))

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

		// Up/Down: scroll viewport within the active tab
		case key.Matches(msg, defaultKeyMap.Up):
			m.incidentViewer, _ = m.incidentViewer.Update(msg)
			return m, nil

		case key.Matches(msg, defaultKeyMap.Down):
			m.incidentViewer, _ = m.incidentViewer.Update(msg)
			return m, nil

		// Tab/Shift+Tab: switch between tabs
		case key.Matches(msg, defaultKeyMap.TabNext):
			m.activeTab = (m.activeTab + 1) % tabCount
			m.incidentViewer.GotoTop()
			return m, func() tea.Msg { return renderIncidentMsg("tab switch") }

		case key.Matches(msg, defaultKeyMap.TabPrev):
			m.activeTab = (m.activeTab + tabCount - 1) % tabCount
			m.incidentViewer.GotoTop()
			return m, func() tea.Msg { return renderIncidentMsg("tab switch") }

		case key.Matches(msg, defaultKeyMap.Refresh):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			return m, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) }

		case key.Matches(msg, defaultKeyMap.Ack):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{} }

		case key.Matches(msg, defaultKeyMap.UnAck):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			m.pendingConfirmation = &confirmActionState{
				prompt: fmt.Sprintf("Re-escalate %s? [y/n]", m.selectedIncident.ID),
				action: func() tea.Msg { return unAcknowledgeIncidentsMsg{} },
			}
			return m, nil

		case key.Matches(msg, defaultKeyMap.Silence):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			m.pendingConfirmation = &confirmActionState{
				prompt: fmt.Sprintf("Silence %s? [y/n]", m.selectedIncident.ID),
				action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
			}
			return m, nil

		case key.Matches(msg, defaultKeyMap.Note):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			// Note template requires full incident data (HTMLURL, Title, Service.Summary)
			if !m.incidentDataLoaded {
				m.setStatus("Loading incident details, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return parseTemplateForNoteMsg("add note") }

		case key.Matches(msg, defaultKeyMap.Login):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			// Login requires alerts to extract cluster_id
			if !m.incidentAlertsLoaded {
				m.setStatus("Loading incident alerts, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return loginMsg("login") }

		case key.Matches(msg, defaultKeyMap.Open):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			// Browser open requires HTMLURL from full incident data
			if !m.incidentDataLoaded {
				m.setStatus("Loading incident details, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return openBrowserMsg("incident") }

		case key.Matches(msg, defaultKeyMap.SOP):
			if m.selectedIncident == nil {
				m.setStatus("no incident selected")
				return m, nil
			}
			// SOP link requires alerts to extract the link field
			if !m.incidentAlertsLoaded {
				m.setStatus("Loading incident alerts, please wait...")
				return m, nil
			}
			return m, func() tea.Msg { return openSOPMsg("sop") }

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
