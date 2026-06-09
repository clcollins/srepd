package tui

import (
	"fmt"
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
	m.recomputeLayout()

	log.Debug("tui.windowSizeMsgHandler",
		"window_width", m.layout.WindowWidth,
		"window_height", m.layout.WindowHeight,
		"table_width", m.layout.TableWidth,
		"table_height", m.layout.TableHeight,
		"column_width", m.layout.ColumnWidth,
	)

	m.table.SetColumns([]table.Column{
		{Title: dot, Width: dotWidth},
		{Title: "ID", Width: idWidth - dotWidth},
		{Title: "Summary", Width: m.layout.ColumnWidth},
		{Title: "Service", Width: m.layout.ColumnWidth},
	})

	m.incidentViewer.Width = m.layout.IncidentViewerWidth
	m.incidentViewer.Height = m.layout.IncidentViewerHeight
	m.logViewer.Width = m.layout.IncidentViewerWidth
	m.logViewer.Height = m.layout.IncidentViewerHeight

	if m.watcherExpanded {
		m.watcherViewport.Width = m.layout.WatcherWidth
		m.watcherViewport.Height = m.layout.WatcherHeight
	}

	if m.configMode && m.configForm != nil {
		m.configForm.WithWidth(m.layout.FormWidth).WithHeight(m.layout.FormHeight)
		form, cmd := m.configForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.configForm = f
		}
		return m, cmd
	}

	if m.teamSelectMode && m.teamSelectForm != nil {
		m.teamSelectForm.Update(msg)
	}

	if m.mergeMode {
		m.mergeTable.SetHeight(m.layout.TableHeight)
	}

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

	if m.input.Focused() {
		if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Quit) {
			return m, tea.Quit
		}
		return switchInputFocusMode(m, msg)
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

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Watcher) {
		m.watcherExpanded = !m.watcherExpanded
		m.recomputeLayout()
		return m, nil
	}

	// Commands for any focus mode
	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Input) {
		if msg.(tea.KeyMsg).String() == "/" {
			m.input.SetValue("/")
			m.input.SetCursor(1)
		}
		return m, tea.Sequence(
			m.input.Focus(),
		)
	}

	if key.Matches(msg.(tea.KeyMsg), defaultKeyMap.Flag) {
		m.input.SetValue("/flag ")
		m.input.SetCursor(len("/flag "))
		return m, tea.Sequence(
			m.input.Focus(),
		)
	}

	// Default commands for the table view
	switch {
	case m.configMode:
		return switchConfigFocusMode(m, msg)

	case m.bulkSilenceMode:
		return switchBulkSilenceFocusMode(m, msg)

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

func switchBulkSilenceFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.bulkSilenceForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.bulkSilenceForm = f
	}
	if m.bulkSilenceForm.State == huh.StateCompleted {
		m.bulkSilenceMode = false
		m.table.Focus()

		if len(m.bulkSilenceIDs) == 0 {
			m.setStatus("no incidents selected for silence")
			return m, nil
		}

		var selected []pagerduty.Incident
		idSet := make(map[string]bool)
		for _, id := range m.bulkSilenceIDs {
			idSet[id] = true
		}
		for _, inc := range m.incidentList {
			if idSet[inc.ID] {
				selected = append(selected, inc)
			}
		}

		incidentIDs := strings.Join(m.bulkSilenceIDs, ", ")
		m.pendingConfirmation = &confirmActionState{
			prompt: fmt.Sprintf("Silence %d incident(s): %s? [y/n]", len(selected), incidentIDs),
			action: func() tea.Msg {
				return bulkSilenceConfirmedMsg{incidents: selected}
			},
		}
		return m, nil
	}
	if m.bulkSilenceForm.State == huh.StateAborted {
		m.bulkSilenceMode = false
		m.table.Focus()
		m.setStatus("bulk silence cancelled")
		return m, nil
	}
	return m, cmd
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
			prompt := m.input.Value()

			if prompt == "" {
				m.input.Blur()
				m.table.Focus()
				return m, nil
			}

			if isAgentCommand(prompt) {
				query := parseAgentQuery(prompt)
				if query == "" {
					m.setStatus("usage: /agent <query>")
					return m, nil
				}
				m.input.Reset()
				return m, func() tea.Msg {
					return claudePromptMsg{prompt: query}
				}
			}

			m.input.Reset()
			m.input.Blur()
			m.table.Focus()

			if isFlagCommand(prompt) {
				return m, m.dispatchFlagCommand(prompt)
			}

			m.setStatus("unknown command — try /agent <query> or /flag <type> <value>")
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
