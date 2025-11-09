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
	// TickMsg are not helpful for logging
	if msgType != reflect.TypeOf(TickMsg{}) {
		log.Debug("Update", msgType, filterMsgContent(msg))
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

		m.setStatus(fmt.Sprintf("got incident %s", msg.incident.ID))
		m.selectedIncident = msg.incident
		m.incidentDataLoaded = true
		if m.viewingIncident {
			return m, func() tea.Msg { return renderIncidentMsg("refresh") }
		}

	case gotIncidentNotesMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		switch {
		case len(msg.notes) == 1:
			m.setStatus(fmt.Sprintf("got %d note for incident", len(msg.notes)))
		case len(msg.notes) > 1:
			m.setStatus(fmt.Sprintf("got %d notes for incident", len(msg.notes)))
		}

		m.selectedIncidentNotes = msg.notes
		m.incidentNotesLoaded = true
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	case gotIncidentAlertsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		switch {
		case len(msg.alerts) == 1:
			m.setStatus(fmt.Sprintf("got %d alert for incident", len(msg.alerts)))
		case len(msg.alerts) > 1:
			m.setStatus(fmt.Sprintf("got %d alerts for incident", len(msg.alerts)))
		}

		m.selectedIncidentAlerts = msg.alerts
		m.incidentAlertsLoaded = true
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
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

		// Check if any incidents should be auto-acknowledged;
		// This must be done before adding the stale incidents
		for _, i := range m.incidentList {
			if ShouldBeAcknowledged(m.config, i, m.config.CurrentUser.ID, m.autoAcknowledge) {
				acknowledgeIncidentsList = append(acknowledgeIncidentsList, i)
			}
		}

		if len(acknowledgeIncidentsList) > 0 {
			cmds = append(cmds, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: acknowledgeIncidentsList} })
		}

		// Add stale incidents to the list
		m.incidentList = append(m.incidentList, staleIncidentList...)

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
		return m, func() tea.Msg { return openBrowserCmd(c, m.selectedIncident.HTMLURL) }

	case browserFinishedMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		} else {
			m.setStatus(fmt.Sprintf("opened incident %s in browser - check browser window", m.selectedIncident.ID))
			return m, nil
		}

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
		m.incidentViewer.SetContent(msg.content)
		m.viewingIncident = true

	case acknowledgeIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed acknowledging incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			acknowledgeIncidents(m.config, msg.incidents, false),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: acknowledgeIncidentsMsg") },
		)

	case unAcknowledgeIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed re-escalating incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			acknowledgeIncidents(m.config, msg.incidents, true),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: unAcknowledgeIncidentsMsg") },
		)

	case acknowledgedIncidentsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		incidentIDs := strings.Join(getIDsFromIncidents(msg.incidents), " ")
		m.setStatus(fmt.Sprintf("acknowledged incidents: %s", incidentIDs))

		return m, func() tea.Msg { return updateIncidentListMsg("sender: acknowledgedIncidentsMsg") }

	case unAcknowledgedIncidentsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		incidentIDs := strings.Join(getIDsFromIncidents(msg.incidents), " ")
		m.setStatus(fmt.Sprintf("re-escalated incidents: %s", incidentIDs))

		return m, func() tea.Msg { return updateIncidentListMsg("sender: unAcknowledgedIncidentsMsg") }

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

		incidents := append(msg.incidents, *m.selectedIncident)
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
