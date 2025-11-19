package tui

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

const (
	title              = "SREPD: It really whips the PDs' ACKs!"
	waitTime           = time.Millisecond * 1
	defaultInputPrompt = " $ "
	maxStaleAge        = time.Minute * 5
	nilNoteErr         = "incident note content is empty"
	nilIncidentMsg     = "no incident selected"
	staleLabelRegex    = "^(\\[STALE\\]\\s)?(.*)$"
	staleLabelStr      = "[STALE] $2"
)

const (
	reEscalateDefaultPolicyLevel = 2 // Skips Nobody
	silentDefaultPolicyKey       = "SILENT_DEFAULT"
	silentDefaultPolicyLevel     = 1 // Nobody
)

func (s setStatusMsg) Status() string {
	return s.string
}

func (m model) Init() tea.Cmd {
	if m.err != nil {
		return func() tea.Msg { return errMsg{m.err} }
	}
	return tea.Batch(
		tea.SetWindowTitle(title),
		func() tea.Msg { return updateIncidentListMsg("sender: Init") },
	)

}

type filteredMsg struct {
	msg       tea.Msg
	truncated bool
}

type scheduledJob struct {
	jobMsg    tea.Cmd
	lastRun   time.Time
	frequency time.Duration
}

func filterMsgContent(msg tea.Msg) tea.Msg {
	var truncatedMsg string
	switch msg := msg.(type) {
	default:
		return msg
	case renderedIncidentMsg:
		truncatedMsg = "template rendered"
	case updatedIncidentListMsg:
		var ids []string
		for _, i := range msg.incidents {
			ids = append(ids, i.ID)
		}
		truncatedMsg = fmt.Sprintf("%v", ids)
	case gotIncidentAlertsMsg:
		var ids []string
		for _, i := range msg.alerts {
			ids = append(ids, i.ID)
		}
		truncatedMsg = fmt.Sprintf("%v", ids)
	case gotIncidentNotesMsg:
		var ids []string
		for _, i := range msg.notes {
			ids = append(ids, i.ID)
		}
		truncatedMsg = fmt.Sprintf("%v", ids)
	}
	return filteredMsg{
		msg:       truncatedMsg,
		truncated: true,
	}
}

// The Update function is called for every message that is sent to the model,
// and it is responsible for updating the model based on the message and returning
// the new model and a command to execute.  These commands should be actual
// tea.Cmds, not functions that return tea.Msgs, though that signature is also a
// tea.Cmd, unless Update should handle the msg, or the msg is a tea.Batch or
// tea.Sequence.
//
// eg, good:
// return m, getIncident(m.config, msg.incident.ID)
//
// eg, ok:
// return m, func() tea.Msg { return errMsg{msg.err} }
//
// eg, bad:
// return m, func() tea.Msg { getIncident(m.config, msg.incident.ID) }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	msgType := reflect.TypeOf(msg)
	// TickMsg and arrow key messages are not helpful for logging
	shouldLog := msgType != reflect.TypeOf(TickMsg{})
	if keyMsg, ok := msg.(tea.KeyMsg); ok && shouldLog {
		// Skip logging for arrow keys used in scrolling
		if keyMsg.Type == tea.KeyUp || keyMsg.Type == tea.KeyDown {
			shouldLog = false
		}
	}
	if shouldLog {
		log.Debug("Update", msgType, filterMsgContent(msg))
	}

	// PRIORITY HANDLING: Process user input keys immediately, before any queued messages
	// This ensures navigation and interaction keys are always responsive
	// even when the message queue is backed up with async responses
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Filter out terminal escape sequences that aren't real keypresses
		// These come from terminal queries (colors, cursor position, etc.)
		keyStr := keyMsg.String()

		// Drop terminal response sequences (OSC, CSI, etc.)
		if strings.Contains(keyStr, "rgb:") ||        // Color queries: ]11;rgb:1d1d/1d1d/2020
			strings.Contains(keyStr, ":1d1d/") ||     // Partial color responses
			strings.Contains(keyStr, "gb:1d1d/") ||   // Truncated color responses
			strings.Contains(keyStr, "alt+]") ||      // OSC start sequence
			strings.Contains(keyStr, "alt+\\") ||     // OSC/DCS end sequence
			strings.Contains(keyStr, "CSI") ||        // Control Sequence Introducer
			keyStr == "OP" ||                         // SS3 sequence (function keys)
			keyStr == "[A" || keyStr == "[B" ||       // Broken arrow key sequences
			keyStr == "[C" || keyStr == "[D" ||       // (should be handled by bubbletea)
			(strings.HasPrefix(keyStr, "[") && strings.HasSuffix(keyStr, "R")) || // CPR: [row;colR
			(strings.HasPrefix(keyStr, "]11;") || strings.HasPrefix(keyStr, "11;")) { // OSC 11 fragments
			// Drop these fake key messages - they're terminal responses, not user input
			log.Debug("Update", "filtered terminal escape sequence", keyStr)
			return m, nil
		}

		// All real user keypresses get priority handling
		// This ensures the UI is always responsive even when async messages are queued
		log.Debug("Update", "priority key handling", keyMsg.String())
		return m.keyMsgHandler(keyMsg)
	}

	// Filter out unknown CSI sequences (cursor position reports, etc.)
	// These are private bubble tea types, so we check the string representation
	msgStr := fmt.Sprintf("%T", msg)
	if strings.Contains(msgStr, "unknownCSISequenceMsg") {
		log.Debug("Update", "filtered unknown CSI sequence", msg)
		return m, nil
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case errMsg:
		return m.errMsgHandler(msg)

	case TickMsg:
		return m, tea.Batch(runScheduledJobs(&m)...)

	case tea.WindowSizeMsg:
		return m.windowSizeMsgHandler(msg)

	case tea.KeyMsg:
		return m.keyMsgHandler(msg)

	case setStatusMsg:
		return m.setStatusMsgHandler(msg)

	// Command to trigger a regular poll for new incidents
	case PollIncidentsMsg:
		if !m.autoRefresh {
			return m, nil
		}
		return m, updateIncidentList(m.config)

	// Command to get an incident by ID
	case getIncidentMsg:
		if msg == "" {
			return m, func() tea.Msg {
				return setStatusMsg{"no incident selected"}
			}
		}

		m.setStatus(fmt.Sprintf("getting details for incident %v...", msg))
		id := string(msg)
		cmds = append(cmds, 
			getIncident(m.config, id),
			getIncidentAlerts(m.config, id),
			getIncidentNotes(m.config, id),
		)

	// Set the selected incident to the incident returned from the getIncident command
	case gotIncidentMsg:
		if msg.err != nil {
			m.selectedIncident = nil
			m.viewingIncident = false
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// Update cache with fetched incident data
		if cached, exists := m.incidentCache[msg.incident.ID]; exists {
			log.Debug("Update", "gotIncidentMsg", "refreshing cached incident data", "incident", msg.incident.ID)
			cached.incident = msg.incident
			cached.dataLoaded = true
			cached.lastFetched = time.Now()
		} else {
			log.Debug("Update", "gotIncidentMsg", "caching new incident data", "incident", msg.incident.ID)
			m.incidentCache[msg.incident.ID] = &cachedIncidentData{
				incident:    msg.incident,
				dataLoaded:  true,
				lastFetched: time.Now(),
			}
		}

		// Only update selected incident if no incident is selected or this matches the selected one
		// Skip if we're viewing a different incident (don't let background pre-fetch overwrite it)
		if m.selectedIncident == nil || msg.incident.ID == m.selectedIncident.ID {
			m.setStatus(fmt.Sprintf("got incident %s", msg.incident.ID))
			m.selectedIncident = msg.incident
			m.incidentDataLoaded = true

			if m.viewingIncident {
				return m, func() tea.Msg { return renderIncidentMsg("refresh") }
			}
		}

	case gotIncidentNotesMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// Update cache with fetched notes
		if cached, exists := m.incidentCache[msg.incidentID]; exists {
			log.Debug("Update", "gotIncidentNotesMsg", "refreshing cached notes", "incident", msg.incidentID, "count", len(msg.notes))
			cached.notes = msg.notes
			cached.notesLoaded = true
		} else {
			log.Debug("Update", "gotIncidentNotesMsg", "caching new notes", "incident", msg.incidentID, "count", len(msg.notes))
			m.incidentCache[msg.incidentID] = &cachedIncidentData{
				notes:       msg.notes,
				notesLoaded: true,
			}
		}

		// Only update selected incident notes if no incident is selected or this matches the selected one
		// Skip if we're viewing a different incident (don't let background pre-fetch overwrite it)
		if m.selectedIncident == nil || msg.incidentID == m.selectedIncident.ID {
			switch {
			case len(msg.notes) == 1:
				m.setStatus(fmt.Sprintf("got %d note for incident", len(msg.notes)))
			case len(msg.notes) > 1:
				m.setStatus(fmt.Sprintf("got %d notes for incident", len(msg.notes)))
			}

			m.selectedIncidentNotes = msg.notes
			m.incidentNotesLoaded = true

			// Don't auto-render here - wait for explicit render request
			// This prevents redundant template renders when alerts/notes arrive separately
		}

	case gotIncidentAlertsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// Update cache with fetched alerts
		if cached, exists := m.incidentCache[msg.incidentID]; exists {
			log.Debug("Update", "gotIncidentAlertsMsg", "refreshing cached alerts", "incident", msg.incidentID, "count", len(msg.alerts))
			cached.alerts = msg.alerts
			cached.alertsLoaded = true
		} else {
			log.Debug("Update", "gotIncidentAlertsMsg", "caching new alerts", "incident", msg.incidentID, "count", len(msg.alerts))
			m.incidentCache[msg.incidentID] = &cachedIncidentData{
				alerts:       msg.alerts,
				alertsLoaded: true,
			}
		}

		// Only update selected incident alerts if no incident is selected or this matches the selected one
		// Skip if we're viewing a different incident (don't let background pre-fetch overwrite it)
		if m.selectedIncident == nil || msg.incidentID == m.selectedIncident.ID {
			switch {
			case len(msg.alerts) == 1:
				m.setStatus(fmt.Sprintf("got %d alert for incident", len(msg.alerts)))
			case len(msg.alerts) > 1:
				m.setStatus(fmt.Sprintf("got %d alerts for incident", len(msg.alerts)))
			}

			m.selectedIncidentAlerts = msg.alerts
			m.incidentAlertsLoaded = true

			// Don't auto-render here - wait for explicit render request
			// This prevents redundant template renders when alerts/notes arrive separately
		}

	case updateIncidentListMsg:
		m.setStatus(loadingIncidentsStatus)
		cmds = append(cmds, updateIncidentList(m.config))

	case updatedIncidentListMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		var staleIncidentList []pagerduty.Incident
		var acknowledgeIncidentsList []pagerduty.Incident

		// If m.incidentList contains incidents that are not in msg.incidents, add them to the stale list
		for _, i := range m.incidentList {
			idx := slices.IndexFunc(msg.incidents, func(incident pagerduty.Incident) bool {
				return incident.ID == i.ID
			})

			updated, err := time.Parse(time.RFC3339, i.LastStatusChangeAt)

			if err != nil {
				log.Error("Update", "updatedIncidentListMsg", "failed to parse time", "incident", i.ID, "time", updated, "error", err)
				updated = time.Now().Add(-(maxStaleAge))
			}

			age := time.Since(updated)
			ttl := maxStaleAge - age

			if idx == -1 {
				if ttl <= 0 {
					log.Debug("Update", "updatedIncidentListMsg", "removing stale incident", "incident", i.ID, "lastUpdated", updated, "ttl", ttl)
				} else {
					log.Debug("Update", "updatedIncidentListMsg", "adding stale incident", "incident", i.ID, "lastUpdated", updated, "ttl", ttl)

					// Add stale label to incident title to make it clear to the user
					m := regexp.MustCompile(staleLabelRegex)
					i.Title = m.ReplaceAllString(i.Title, staleLabelStr)

					staleIncidentList = append(staleIncidentList, i)
				}
			}
		}

		// Overwrite m.incidentList with current incidents
		m.incidentList = msg.incidents

		// Pre-fetch incident data for all incidents in the list
		for _, i := range m.incidentList {
			// Check if incident is already cached
			if _, exists := m.incidentCache[i.ID]; !exists {
				// Not cached - pre-fetch all data in the background
				cmds = append(cmds,
					getIncident(m.config, i.ID),
					getIncidentAlerts(m.config, i.ID),
					getIncidentNotes(m.config, i.ID),
				)
			}
		}

		// Check if any incidents should be auto-acknowledged;
		// This must be done before adding the stale incidents
		if m.autoAcknowledge {
			// Cache the on-call check - it's the same for all incidents in this update
			userIsOnCall := UserIsOnCall(m.config, m.config.CurrentUser.ID)

			for _, i := range m.incidentList {
				if ShouldBeAcknowledgedCached(i, m.config.CurrentUser.ID, userIsOnCall) {
					acknowledgeIncidentsList = append(acknowledgeIncidentsList, i)
				}
			}

			if len(acknowledgeIncidentsList) > 0 {
				cmds = append(cmds, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: acknowledgeIncidentsList} })
			}
		}

		// Add stale incidents to the list
		m.incidentList = append(m.incidentList, staleIncidentList...)

		// Clean up cache - remove entries for incidents no longer in the list
		// (including STALE incidents, but excluding those that have aged out)
		incidentIDs := make(map[string]bool)
		for _, i := range m.incidentList {
			incidentIDs[i.ID] = true
		}
		for id := range m.incidentCache {
			if !incidentIDs[id] {
				delete(m.incidentCache, id)
				log.Debug("Update", "updatedIncidentListMsg", "removing cached data for incident no longer in list", "incident", id)
			}
		}

		var totalIncidentCount int
		var rows []table.Row

		for _, i := range m.incidentList {
			totalIncidentCount++
			state := stateShorthand(i, m.config.CurrentUser.ID)
			if AssignedToUser(i, m.config.CurrentUser.ID) || m.teamMode {
				rows = append(rows, table.Row{state, i.ID, i.Title, i.Service.Summary})
			}
		}

		m.table.SetRows(rows)

		if totalIncidentCount == 1 {
			m.setStatus(fmt.Sprintf("showing %d/%d incident...", len(m.table.Rows()), totalIncidentCount))
		} else {
			m.setStatus(fmt.Sprintf("showing %d/%d incidents...", len(m.table.Rows()), totalIncidentCount))
		}

		if m.selectedIncident != nil {
			// Check if the m.selectedIncident is still in the list
			idx := slices.IndexFunc(m.incidentList, func(incident pagerduty.Incident) bool {
				return incident.ID == m.selectedIncident.ID
			})

			if idx == -1 {
				m.clearSelectedIncident("selected incident no longer in list after update")
			}
		}

	case parseTemplateForNoteMsg:
		if m.selectedIncident == nil {
			m.setStatus("failed to open editor - no selected incident")
		}
		cmds = append(cmds, parseTemplateForNote(m.selectedIncident))

	case parsedTemplateForNoteMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		cmds = append(cmds, openEditorCmd(m.editor, msg.content))

	case editorFinishedMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		if m.selectedIncident == nil {
			m.setStatus("failed to add note - no selected incident")
			return m, nil
		}

		cmds = append(cmds, addNoteToIncident(m.config, m.selectedIncident, msg.file))

	// Refresh the local copy of the incident after the note is added
	case addedIncidentNoteMsg:
		if msg.err != nil {
			if msg.err.Error() == nilNoteErr {
				m.status = "skipping adding empty note to incident"
				return m, nil
			}
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		if m.selectedIncident == nil {
			m.setStatus("unable to refresh incident - no selected incident")
			return m, nil
		}
		cmds = append(cmds, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) })

	case loginMsg:
		var cluster string

		switch len(m.selectedIncidentAlerts) {
		case 0:
			log.Debug("Update", reflect.TypeOf(msg), fmt.Sprintf("no alerts found for incident %s - requeuing", m.selectedIncident.ID))
			return m, func() tea.Msg { return loginMsg("sender: loginMsg; requeue") }
		case 1:
			cluster = getDetailFieldFromAlert("cluster_id", m.selectedIncidentAlerts[0])
			m.setStatus(fmt.Sprintf("logging into cluster %s from alert %s", cluster, m.selectedIncidentAlerts[0].ID))
		default:
			// TODO https://github.com/clcollins/srepd/issues/1: Figure out how to prompt with list to select from
			cluster = getDetailFieldFromAlert("cluster_id", m.selectedIncidentAlerts[0])
			m.setStatus(fmt.Sprintf("multiple alerts found - logging into cluster %s from first alert %s", cluster, m.selectedIncidentAlerts[0].ID))
		}

		// NOTE: It's important that **ALL** of these variables' values are NOT NIL.
		// They can be empty strings, but the must not be nil.
		var vars = map[string]string{
			"%%CLUSTER_ID%%":  cluster,
			"%%INCIDENT_ID%%": m.selectedIncident.ID,
		}

		cmds = append(cmds, login(vars, m.launcher))

	case loginFinishedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("failed to login: %s", msg.err)
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

	case openBrowserMsg:
		if defaultBrowserOpenCommand == "" {
			return m, func() tea.Msg { return errMsg{fmt.Errorf("unsupported OS: no browser open command available")} }
		}
		c := []string{defaultBrowserOpenCommand}
		return m, openBrowserCmd(c, m.selectedIncident.HTMLURL)

	case browserFinishedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("failed to open browser: %s", msg.err))
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		m.setStatus(fmt.Sprintf("opened incident %s in browser - check browser window", m.selectedIncident.ID))
		return m, nil

	// This is a catch all for any action that requires a selected incident
	case waitForSelectedIncidentThenDoMsg:
		if msg.action == nil {
			m.setStatus("failed to perform action: no action included in msg")
			return m, nil
		}
		if msg.msg == nil {
			m.setStatus("failed to perform action: no data included in msg")
			return m, nil
		}

		// If the user has closed the incident view (via ESC), abort the action
		// instead of waiting forever for an incident that will never be set
		if m.selectedIncident == nil && !m.viewingIncident {
			log.Debug("Update", "waitForSelectedIncidentThenDoMsg", "aborting action - incident view closed", "msg", msg.msg)
			m.setStatus("action cancelled - incident view closed")
			return m, nil
		}

		// Re-queue the message if the selected incident is not yet available
		if m.selectedIncident == nil {
			m.setStatus("waiting for incident info...")
			return m, func() tea.Msg { return msg }
		}

		// Perform the action once the selected incident is available
		log.Debug("Update", "waitForSelectedIncidentThenDoMsg", "performing action", "action", msg.action, "incident", m.selectedIncident.ID)
		return m, msg.action

	case renderIncidentMsg:
		if m.selectedIncident == nil {
			m.setStatus("failed render incidents - no incidents provided")
			m.viewingIncident = false
			return m, nil
		}

		cmds = append(cmds, renderIncident(&m))

	case renderedIncidentMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		log.Debug("renderedIncidentMsg")

		// Only set viewing state if we still have a selected incident
		// This prevents late-arriving render messages from reopening the incident view
		// after the user has already closed it with ESC
		if m.selectedIncident != nil {
			m.incidentViewer.SetContent(msg.content)
			m.viewingIncident = true
		} else {
			log.Debug("renderedIncidentMsg", "action", "discarding render - incident was closed")
		}

	case acknowledgeIncidentsMsg:
		// If incidents are provided in the message, use those
		// Otherwise, use the selected incident from the model
		incidents := msg.incidents
		if incidents == nil {
			if m.selectedIncident == nil {
				m.setStatus("failed acknowledging incidents - no incidents provided and no incident selected")
				return m, nil
			}
			incidents = []pagerduty.Incident{*m.selectedIncident}
		}

		return m, tea.Sequence(
			acknowledgeIncidents(m.config, incidents),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: acknowledgeIncidentsMsg") },
		)

	case unAcknowledgeIncidentsMsg:
		// If incidents are provided in the message, use those
		// Otherwise, use the selected incident from the model
		incidents := msg.incidents
		if incidents == nil {
			if m.selectedIncident == nil {
				m.setStatus("failed re-escalating incidents - no incidents provided and no incident selected")
				return m, nil
			}
			incidents = []pagerduty.Incident{*m.selectedIncident}
		}

		// Skip un-acknowledge step - go directly to re-escalation
		// Re-escalation will reassign to the current on-call at the escalation level
		// Group incidents by escalation policy
		policyGroups := make(map[string][]pagerduty.Incident)
		for _, incident := range incidents {
			policyKey := getEscalationPolicyKey(incident.Service.ID, m.config.EscalationPolicies)
			policyGroups[policyKey] = append(policyGroups[policyKey], incident)
		}

		// Create re-escalate commands for each policy group
		var cmds []tea.Cmd
		for policyKey, incidents := range policyGroups {
			policy := m.config.EscalationPolicies[policyKey]
			if policy != nil && policy.ID != "" {
				cmds = append(cmds, reEscalateIncidents(m.config, incidents, policy, reEscalateDefaultPolicyLevel))
			}
		}

		// Add clear selected incidents after re-escalation
		cmds = append(cmds, func() tea.Msg { return clearSelectedIncidentsMsg("sender: unAcknowledgeIncidentsMsg") })

		if len(cmds) > 0 {
			return m, tea.Sequence(cmds...)
		}

		return m, func() tea.Msg { return updateIncidentListMsg("sender: unAcknowledgeIncidentsMsg") }

	case acknowledgedIncidentsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		incidentIDs := strings.Join(getIDsFromIncidents(msg.incidents), " ")
		m.setStatus(fmt.Sprintf("acknowledged incidents: %s", incidentIDs))

		return m, func() tea.Msg { return updateIncidentListMsg("sender: acknowledgedIncidentsMsg") }


	case reassignIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed reassigning incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			reassignIncidents(m.config, msg.incidents, msg.users),
			func() tea.Msg { return clearSelectedIncidentsMsg("reassign incidents") },
		)

	case reassignedIncidentsMsg:
		incidentIDs := getIDsFromIncidents(msg)
		m.setStatus(fmt.Sprintf("reassigned incidents %v; refreshing Incident List ", incidentIDs))
		return m, func() tea.Msg { return updateIncidentListMsg("sender: reassignedIncidentsMsg") }

	case reEscalateIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed re-escalating incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			reEscalateIncidents(m.config, msg.incidents, msg.policy, msg.level),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: reEscalatedIncidentsMsg") },
		)

	case reEscalatedIncidentsMsg:
		incidentIDs := getIDsFromIncidents(msg)
		m.setStatus(fmt.Sprintf("re-escalated incidents %v; refreshing Incident List ", incidentIDs))
		return m, func() tea.Msg { return updateIncidentListMsg("sender: reEscalatedIncidentsMsg") }

	case silenceSelectedIncidentMsg:
		if m.selectedIncident == nil {
			return m, func() tea.Msg {
				return waitForSelectedIncidentThenDoMsg{
					msg:    "silence",
					action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
				}
			}
		}

		policyKey := getEscalationPolicyKey(m.selectedIncident.Service.ID, m.config.EscalationPolicies)

		return m, tea.Sequence(
			silenceIncidents([]pagerduty.Incident{*m.selectedIncident}, m.config.EscalationPolicies[policyKey], silentDefaultPolicyLevel),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: silenceSelectedIncidentMsg") },
		)

	case silenceIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed silencing incidents - no incidents provided")
			return m, nil
		}

		incidents := msg.incidents
		if m.selectedIncident != nil {
			incidents = append(msg.incidents, *m.selectedIncident)
		}
		return m, tea.Sequence(
			silenceIncidents(incidents, m.config.EscalationPolicies["silent_default"], silentDefaultPolicyLevel),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: silenceIncidentsMsg") },
		)

	case clearSelectedIncidentsMsg:
		m.clearSelectedIncident(msg)
		return m, nil
	}

	return m, tea.Batch(cmds...)

}
