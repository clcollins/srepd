package tui

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/alert"
)

const (
	title              = "SREPD: It really whips the PDs' ACKs!"
	waitTime           = time.Millisecond * 1
	defaultInputPrompt = " $ "
	nilNoteErr         = "incident note content is empty"
	nilIncidentMsg     = "no incident selected"
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
		m.spinner.Tick,
		func() tea.Msg { return updateIncidentListMsg("sender: Init") },
		checkForUpdate(m.devMode, ""),
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
	// Reduce logging for high-frequency messages to prevent I/O overhead
	shouldLog := true

	// Skip logging for very frequent messages
	switch msgType {
	case reflect.TypeOf(TickMsg{}),
		reflect.TypeOf(spinner.TickMsg{}):
		shouldLog = false
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok && shouldLog {
		// Skip logging for arrow keys and other navigation used in scrolling
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
		if strings.Contains(keyStr, "rgb:") || // Color queries: ]11;rgb:1d1d/1d1d/2020
			strings.Contains(keyStr, ":1d1d/") || // Partial color responses
			strings.Contains(keyStr, "gb:1d1d/") || // Truncated color responses (missing 'r')
			strings.Contains(keyStr, "b:1c1c/") || // Another partial rgb: response
			strings.Contains(keyStr, "1c/1f1f") || // Bare color hex values
			strings.Contains(keyStr, "/1f1f") || // Fragment of hex color
			strings.Contains(keyStr, "1c1c/") || // Fragment of hex color
			strings.Contains(keyStr, "alt+]") || // OSC start sequence
			strings.Contains(keyStr, "alt+\\") || // OSC/DCS end sequence
			strings.Contains(keyStr, "CSI") || // Control Sequence Introducer
			keyStr == "OP" || // SS3 sequence (function keys)
			keyStr == "[A" || keyStr == "[B" || // Broken arrow key sequences
			keyStr == "[C" || keyStr == "[D" || // (should be handled by bubbletea)
			(strings.HasPrefix(keyStr, "[") && strings.HasSuffix(keyStr, "R")) || // CPR: [row;colR
			(strings.HasPrefix(keyStr, "]11;") || strings.HasPrefix(keyStr, "11;")) { // OSC 11 fragments
			// Drop these fake key messages - they're terminal responses, not user input
			// Don't log them - they're noise
			return m, nil
		}

		// All real user keypresses get priority handling
		// This ensures the UI is always responsive even when async messages are queued
		return m.keyMsgHandler(keyMsg)
	}

	// Filter out unknown CSI sequences (cursor position reports, etc.)
	// These are private bubble tea types, so we check the string representation
	msgStr := fmt.Sprintf("%T", msg)
	if strings.Contains(msgStr, "unknownCSISequenceMsg") {
		// Don't log these - they're noise from terminal queries
		return m, nil
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case errMsg:
		return m.errMsgHandler(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case TickMsg:
		return m, tea.Batch(runScheduledJobs(&m)...)

	case tea.WindowSizeMsg:
		return m.windowSizeMsgHandler(msg)

	case tea.KeyMsg:
		return m.keyMsgHandler(msg)

	case setStatusMsg:
		return m.setStatusMsgHandler(msg)

	case clearFlashMsg:
		// Only clear the status if it still matches the flash message.
		// This prevents a newer message from being prematurely dismissed.
		if m.status == msg.message {
			m.setStatus("")
		}
		return m, nil

	// Command to trigger a regular poll for new incidents
	case PollIncidentsMsg:
		if !m.autoRefresh {
			return m, nil
		}
		m.apiInProgress = true
		return m, tea.Batch(m.spinner.Tick, updateIncidentList(m.config))

	// Command to get an incident by ID
	case getIncidentMsg:
		if msg == "" {
			return m, func() tea.Msg {
				return setStatusMsg{"no incident selected"}
			}
		}

		m.setStatus(fmt.Sprintf("getting details for incident %v...", msg))
		id := string(msg)
		m.apiInProgress = true
		cmds = append(cmds,
			m.spinner.Tick,
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
		// Check if this message is still relevant to the current selection
		// Prevents late-arriving messages from overwriting when user navigated away
		shouldUpdate := false
		if m.selectedIncident == nil {
			shouldUpdate = true
		} else if m.selectedIncident.ID == msg.incident.ID {
			shouldUpdate = true
		}

		if shouldUpdate {
			m.setStatus(fmt.Sprintf("got incident %s", msg.incident.ID))
			m.selectedIncident = msg.incident
			m.incidentDataLoaded = true

			// Stop spinner if all incident data is loaded (details, notes, alerts)
			if m.incidentDataLoaded && m.incidentNotesLoaded && m.incidentAlertsLoaded {
				m.apiInProgress = false
			}

			// Re-render if we're viewing the incident to show updated details progressively
			if m.viewingIncident {
				return m, func() tea.Msg { return renderIncidentMsg("incident details arrived") }
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

			// Stop spinner if all incident data is loaded (details, notes, alerts)
			if m.incidentDataLoaded && m.incidentNotesLoaded && m.incidentAlertsLoaded {
				m.apiInProgress = false
			}

			// Re-render if we're viewing the incident to show the notes progressively
			if m.viewingIncident && m.selectedIncident != nil && msg.incidentID == m.selectedIncident.ID {
				return m, func() tea.Msg { return renderIncidentMsg("notes arrived") }
			}
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

			// Stop spinner if all incident data is loaded (details, notes, alerts)
			if m.incidentDataLoaded && m.incidentNotesLoaded && m.incidentAlertsLoaded {
				m.apiInProgress = false
			}

			// Re-render if we're viewing the incident to show the alerts progressively
			if m.viewingIncident && m.selectedIncident != nil && msg.incidentID == m.selectedIncident.ID {
				return m, func() tea.Msg { return renderIncidentMsg("alerts arrived") }
			}
		}

	case updateIncidentListMsg:
		m.setStatus(loadingIncidentsStatus)
		m.apiInProgress = true
		cmds = append(cmds, m.spinner.Tick, updateIncidentList(m.config))

	case updatedIncidentListMsg:
		if msg.err != nil {
			m.apiInProgress = false
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		m.apiInProgress = false

		var acknowledgeIncidentsList []pagerduty.Incident

		// Flash notification for resolved incidents
		var resolvedIDs []string
		for _, i := range m.incidentList {
			idx := slices.IndexFunc(msg.incidents, func(incident pagerduty.Incident) bool {
				return incident.ID == i.ID
			})
			if idx == -1 {
				resolvedIDs = append(resolvedIDs, i.ID)
			}
		}
		if len(resolvedIDs) > 0 {
			for _, id := range resolvedIDs {
				log.Info("incident resolved", "incident_id", id)
			}
			resolvedMsg := fmt.Sprintf("Resolved: %s", strings.Join(resolvedIDs, ", "))
			cmds = append(cmds, m.flashNotification(resolvedMsg))
		}

		// Overwrite m.incidentList with current incidents
		m.incidentList = msg.incidents

		// Note: We no longer pre-fetch all incident details, alerts, and notes here.
		// This was inefficient because:
		// 1. Most incidents are never viewed or acted upon
		// 2. The incident list already contains sufficient data for most actions
		// 3. getHighlightedIncident() uses data from m.incidentList directly
		// 4. Details/alerts/notes are now fetched on-demand when actually needed:
		//    - When user presses Enter to view an incident
		//    - When user presses 'l' to login (needs alerts)
		// This reduces unnecessary API calls from O(n) to O(1) per incident list update.

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

		// Clean up cache - remove entries for incidents no longer in the list
		incidentIDs := make(map[string]bool)
		for _, i := range m.incidentList {
			incidentIDs[i.ID] = true
		}
		for id := range m.incidentCache {
			if incidentIDs[id] {
				// Incident still exists - mark cache as potentially stale
				// Find incident in new list and check if LastStatusChangeAt differs
				for _, newIncident := range m.incidentList {
					if newIncident.ID == id {
						if cached, exists := m.incidentCache[id]; exists {
							// Compare timestamps to detect changes
							if cached.incident != nil &&
								cached.incident.LastStatusChangeAt != newIncident.LastStatusChangeAt {
								// Incident changed - invalidate cached details
								log.Debug("Update", "updatedIncidentListMsg", "invalidating cache for updated incident", "id", id)
								delete(m.incidentCache, id)
							}
						}
						break
					}
				}
			} else {
				// Incident no longer in list - remove from cache
				delete(m.incidentCache, id)
				log.Debug("Update", "updatedIncidentListMsg", "removing cached data for incident no longer in list", "incident", id)
			}
		}

		// Capture the currently highlighted incident ID before rebuilding rows
		var highlightedID string
		if currentRow := m.table.SelectedRow(); len(currentRow) > 1 {
			highlightedID = currentRow[1]
		}

		totalIncidentCount := len(m.incidentList)

		// Apply urgency filter before building table rows
		filteredIncidents := filterByUrgency(m.incidentList, m.showLowUrgency)

		var rows []table.Row

		for _, i := range filteredIncidents {
			state := stateShorthand(i, m.config.CurrentUser.ID)
			if AssignedToUser(i, m.config.CurrentUser.ID) || m.teamMode {
				rows = append(rows, table.Row{state, i.ID, i.Title, i.Service.Summary})
			}
		}

		m.table.SetRows(rows)

		// Restore cursor to the previously highlighted incident
		if highlightedID != "" {
			if idx := findRowIndex(rows, highlightedID); idx >= 0 {
				m.table.SetCursor(idx)
			}
		}

		// Build status message with filter and count info
		var filterSuffix string
		if !m.showLowUrgency {
			filterSuffix = " (high only)"
		}

		if totalIncidentCount == 1 {
			m.setStatus(fmt.Sprintf("showing %d/%d incident%s...", len(m.table.Rows()), totalIncidentCount, filterSuffix))
		} else {
			m.setStatus(fmt.Sprintf("showing %d/%d incidents%s...", len(m.table.Rows()), totalIncidentCount, filterSuffix))
		}

		// Re-sync selectedIncident to match highlighted row
		// This handles cases where the incident list changed but cursor position stayed same
		if cmd := m.syncSelectedIncidentToHighlightedRow(); cmd != nil {
			cmds = append(cmds, cmd)
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

		log.Info("added note to incident", "incident_id", m.selectedIncident.ID)

		// Flash notification for note addition
		cmds = append(cmds, m.flashNotification(fmt.Sprintf("Added note to %s", m.selectedIncident.ID)))

		cmds = append(cmds, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) })

	case loginMsg:
		if m.selectedIncident == nil {
			m.setStatus("unable to login - no selected incident")
			return m, nil
		}

		if len(m.selectedIncidentAlerts) == 0 {
			log.Debug("tui.Update()", "msg_type", reflect.TypeOf(msg), "msg", "no alerts found for incident - requeuing", "incident", m.selectedIncident.ID)
			return m, func() tea.Msg { return loginMsg("sender: loginMsg; requeue") }
		}

		clusters := getUniqueClusters(m.selectedIncidentAlerts)

		var cluster string
		switch len(clusters) {
		case 0:
			// Alerts exist but none carry a cluster_id
			cluster = ""
			m.setStatus("no cluster_id found in alerts - launching without cluster")
		case 1:
			cluster = clusters[0]
			m.setStatus(fmt.Sprintf("logging into cluster %s", cluster))
		default:
			// Multiple distinct clusters - open scrollable selection view
			m.clusterSelectMode = true
			m.clusterSelectOptions = clusters
			m.clusterSelectPrompt = "Select cluster to log into (Enter=select, Esc=cancel):"
			cols := []table.Column{{Title: "Cluster ID", Width: 60}}
			var rows []table.Row
			for _, c := range clusters {
				rows = append(rows, table.Row{c})
			}
			m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))
			m.clusterSelectTable.SetStyles(tableStyle)
			return m, nil
		}

		// NOTE: It's important that **ALL** of these variables' values are NOT NIL.
		// They can be empty strings, but the must not be nil.
		var vars = map[string]string{
			"%%CLUSTER_ID%%":  cluster,
			"%%INCIDENT_ID%%": m.selectedIncident.ID,
		}

		log.Info("login initiated",
			"user_id", m.config.CurrentUser.ID,
			"cluster_id", cluster,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		cmds = append(cmds, login(vars, m.launcher, m.selectedIncident, m.selectedIncidentAlerts, m.selectedIncidentNotes))

	case clusterSelectedMsg:
		if m.selectedIncident == nil {
			m.setStatus("unable to login - no selected incident")
			return m, nil
		}

		cluster := string(msg)
		m.setStatus(fmt.Sprintf("logging into cluster %s", cluster))

		var vars = map[string]string{
			"%%CLUSTER_ID%%":  cluster,
			"%%INCIDENT_ID%%": m.selectedIncident.ID,
		}

		log.Info("login initiated",
			"user_id", m.config.CurrentUser.ID,
			"cluster_id", cluster,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		cmds = append(cmds, login(vars, m.launcher, m.selectedIncident, m.selectedIncidentAlerts, m.selectedIncidentNotes))

	case loginFinishedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("failed to login: %s", msg.err)
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		if m.selectedIncident != nil {
			log.Info("login completed", "incident_id", m.selectedIncident.ID)
		} else {
			log.Info("login completed")
		}

	case openBrowserMsg:
		if m.selectedIncident == nil {
			m.setStatus("no incident selected")
			return m, nil
		}
		if defaultBrowserOpenCommand == "" {
			return m, func() tea.Msg { return errMsg{fmt.Errorf("unsupported OS: no browser open command available")} }
		}

		log.Debug("openBrowserMsg", "incident", m.selectedIncident.ID, "title", m.selectedIncident.Title, "service", m.selectedIncident.Service.Summary)
		log.Info("opened incident in browser", "incident_id", m.selectedIncident.ID)

		c := []string{defaultBrowserOpenCommand}
		return m, tea.Batch(m.flashNotification(fmt.Sprintf("Opened %s in browser", m.selectedIncident.ID)), openBrowserCmd(c, m.selectedIncident.HTMLURL))

	case openSOPMsg:
		if m.selectedIncident == nil {
			m.setStatus("no incident selected")
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

		log.Debug("openSOPMsg", "incident", m.selectedIncident.ID, "link", link)
		log.Info("opened SOP in browser", "incident_id", m.selectedIncident.ID, "link", link)

		c := []string{defaultBrowserOpenCommand}
		return m, tea.Batch(m.flashNotification(fmt.Sprintf("Opened SOP for %s", m.selectedIncident.ID)), openBrowserCmd(c, link))

	case browserFinishedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("failed to open browser: %s", msg.err))
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		if m.selectedIncident != nil {
			m.setStatus(fmt.Sprintf("opened incident %s in browser - check browser window", m.selectedIncident.ID))
		} else {
			m.setStatus("opened incident in browser - check browser window")
		}
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

		// If the user has closed the incident view (via ESC) AND there's no highlighted row in the table,
		// abort the action instead of waiting forever for an incident that will never be set
		if m.selectedIncident == nil && !m.viewingIncident && m.table.SelectedRow() == nil {
			log.Debug("Update", "waitForSelectedIncidentThenDoMsg", "aborting action - no incident selected or highlighted", "msg", msg.msg)
			m.setStatus("action cancelled - no incident selected")
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

	case logFileContentMsg:
		m.logViewer.SetContent(string(msg))
		m.logViewer.GotoBottom()
		m.viewingLog = true
		return m, nil

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
			wasViewingBefore := m.viewingIncident
			m.incidentViewer.SetContent(msg.content)
			// Only go to top on first render, not on progressive updates
			if !wasViewingBefore {
				m.incidentViewer.GotoTop()
			}
			m.viewingIncident = true
		} else {
			log.Debug("renderedIncidentMsg", "action", "discarding render - incident was closed")
		}

	case acknowledgeIncidentsMsg:
		// If incidents are provided in the message, use those
		// Otherwise, use the selected incident (which is always synced to highlighted row)
		incidents := msg.incidents
		if incidents == nil {
			if m.selectedIncident == nil {
				m.setStatus("failed acknowledging incidents - no incident selected")
				return m, nil
			}
			incidents = []pagerduty.Incident{*m.selectedIncident}
		}

		m.apiInProgress = true
		return m, tea.Sequence(
			m.spinner.Tick,
			acknowledgeIncidents(m.config, incidents),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: acknowledgeIncidentsMsg") },
		)

	case unAcknowledgeIncidentsMsg:
		// If incidents are provided in the message, use those
		// Otherwise, use the selected incident (which is always synced to highlighted row)
		incidents := msg.incidents
		if incidents == nil {
			if m.selectedIncident == nil {
				m.setStatus("failed re-escalating incidents - no incident selected")
				return m, nil
			}
			incidents = []pagerduty.Incident{*m.selectedIncident}
		}

		// Skip un-acknowledge step - go directly to re-escalation
		// Re-escalation will reassign to the current on-call at the escalation level
		// Group incidents by their current escalation policy ID
		policyGroups := make(map[string][]pagerduty.Incident)
		for _, incident := range incidents {
			// Use the incident's actual escalation policy, not a service-based lookup
			if incident.EscalationPolicy.ID != "" {
				policyGroups[incident.EscalationPolicy.ID] = append(policyGroups[incident.EscalationPolicy.ID], incident)
			} else {
				log.Warn("tui.unAcknowledgeIncidentsMsg", "incident has no escalation policy", "incident_id", incident.ID)
			}
		}

		// Create re-escalate commands for each policy group
		var cmds []tea.Cmd
		for policyID, incidents := range policyGroups {
			// Fetch the full escalation policy details for this policy ID
			cmd := fetchEscalationPolicyAndReEscalate(m.config, incidents, policyID, reEscalateDefaultPolicyLevel)
			cmds = append(cmds, cmd)
		}

		// Add clear selected incidents after re-escalation
		cmds = append(cmds, func() tea.Msg { return clearSelectedIncidentsMsg("sender: unAcknowledgeIncidentsMsg") })

		if len(cmds) > 0 {
			m.apiInProgress = true
			cmds = append([]tea.Cmd{m.spinner.Tick}, cmds...)
			return m, tea.Sequence(cmds...)
		}

		return m, func() tea.Msg { return updateIncidentListMsg("sender: unAcknowledgeIncidentsMsg") }

	case acknowledgedIncidentsMsg:
		m.apiInProgress = false
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		incidentIDs := strings.Join(getIDsFromIncidents(msg.incidents), " ")
		log.Info("acknowledged incident", "incident_id", incidentIDs)

		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Acknowledged %s", incidentIDs)),
			func() tea.Msg { return updateIncidentListMsg("sender: acknowledgedIncidentsMsg") },
		)

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
		m.apiInProgress = false
		incidentIDs := strings.Join(getIDsFromIncidents(msg), " ")
		log.Info("re-escalated incident",
			"user_id", m.config.CurrentUser.ID,
			"reason", func() string {
				if m.selectedIncident != nil {
					return m.selectedIncident.HTMLURL
				}
				return ""
			}(),
			"alert", func() string {
				if m.selectedIncident != nil {
					return alert.ExtractAlertName(m.selectedIncident.Title)
				}
				return ""
			}())

		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Re-escalated %s", incidentIDs)),
			func() tea.Msg { return updateIncidentListMsg("sender: reEscalatedIncidentsMsg") },
		)

	case silenceSelectedIncidentMsg:
		if m.selectedIncident == nil {
			m.setStatus("failed silencing incident - no incident selected")
			return m, nil
		}

		incidentID := m.selectedIncident.ID
		log.Info("silenced incident",
			"user_id", m.config.CurrentUser.ID,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		policyKey := getEscalationPolicyKey(m.selectedIncident.Service.ID, m.config.EscalationPolicies)

		m.apiInProgress = true
		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Silenced %s", incidentID)),
			tea.Sequence(
				m.spinner.Tick,
				silenceIncidents([]pagerduty.Incident{*m.selectedIncident}, m.config.EscalationPolicies[policyKey], silentDefaultPolicyLevel),
				func() tea.Msg { return clearSelectedIncidentsMsg("sender: silenceSelectedIncidentMsg") },
			),
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

		incidentIDs := strings.Join(getIDsFromIncidents(incidents), " ")

		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Silenced %s", incidentIDs)),
			tea.Sequence(
				silenceIncidents(incidents, m.config.EscalationPolicies["silent_default"], silentDefaultPolicyLevel),
				func() tea.Msg { return clearSelectedIncidentsMsg("sender: silenceIncidentsMsg") },
			),
		)

	case clearSelectedIncidentsMsg:
		m.clearSelectedIncident(msg)
		return m, nil

	case claudePromptMsg:
		return m.handleClaudePrompt(msg, defaultHasClaudeCode)

	case claudeResponseMsg:
		return m.handleClaudeResponse(msg)

	case mergeIncidentMsg:
		if m.mergeSourceIncident == nil || m.mergeTargetID == "" {
			m.setStatus("merge failed - missing source or target")
			return m, nil
		}
		sourceID := m.mergeSourceIncident.ID
		targetID := m.mergeTargetID
		m.mergeMode = false
		m.mergeSourceIncident = nil
		m.mergeTargetID = ""
		m.table.Focus()
		m.apiInProgress = true
		return m, tea.Batch(m.spinner.Tick, mergeIncidents(m.config, sourceID, targetID))

	case mergedIncidentMsg:
		m.apiInProgress = false
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		log.Info("merged incident",
			"user_id", m.config.CurrentUser.ID,
			"source", msg.sourceID,
			"target", msg.targetID)
		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Merged %s into %s", msg.sourceID, msg.targetID)),
			func() tea.Msg { return updateIncidentListMsg("sender: mergedIncidentMsg") },
		)

	case updateAvailableMsg:
		m.updateAvailable = true
		m.updateVersion = msg.latest
		m.updateReleaseURL = msg.releaseURL
		return m, nil
	}

	return m, tea.Batch(cmds...)

}
