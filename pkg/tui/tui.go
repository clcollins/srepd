package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	waitTime           = time.Millisecond * 1
	defaultInputPrompt = " $ "
	u
	nilNoteErr = "incident note content is empty"
)

type errMsg struct{ error }

func (m model) Init() tea.Cmd {
	debug("Init")
	if m.err != nil {
		return func() tea.Msg { return errMsg{m.err} }
	}
	return func() tea.Msg { return updateIncidentListMsg("sender: Init") }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	debug("Update")
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case errMsg:
		return m.errMsgHandler(msg)

	case tea.WindowSizeMsg:
		return m.windowSizeMsgHandler(msg)

	case tea.KeyMsg:
		return m.keyMsgHandler(msg)

	// Command to get an incident by ID
	case getIncidentMsg:
		debug("getIncidentMsg", fmt.Sprint(msg))
		m.setStatus(fmt.Sprintf("getting details for incident %v...", msg))
		cmds = append(cmds, getIncident(m.config, string(msg)))

	// Set the selected incident to the incident returned from the getIncident command
	case gotIncidentMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		debug("gotIncidentMsg", msg.incident.ID)

		m.setStatus(fmt.Sprintf("got incident %s", msg.incident.ID))
		m.selectedIncident = msg.incident
		return m, tea.Batch(
			getIncidentAlerts(m.config, msg.incident.ID),
			getIncidentNotes(m.config, msg.incident.ID),
		)

	case gotIncidentNotesMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		getIDs := func(x []pagerduty.IncidentNote) []string {
			var ids []string
			for _, n := range x {
				ids = append(ids, n.ID)
			}
			return ids
		}

		debug("gotIncidentNotesMsg", strings.Join(getIDs(msg.notes), ", "))

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		switch {
		case len(msg.notes) == 1:
			m.setStatus(fmt.Sprintf("got %d note for incident", len(msg.notes)))
		case len(msg.notes) > 1:
			m.setStatus(fmt.Sprintf("got %d notes for incident", len(msg.notes)))
		}

		m.selectedIncidentNotes = msg.notes
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	case gotIncidentAlertsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		getIDs := func(x []pagerduty.IncidentAlert) []string {
			var ids []string
			for _, n := range x {
				ids = append(ids, n.ID)
			}
			return ids
		}

		debug("gotIncidentAlertsMsg", strings.Join(getIDs(msg.alerts), ", "))

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		switch {
		case len(msg.alerts) == 1:
			m.setStatus(fmt.Sprintf("got %d alert for incident", len(msg.alerts)))
		case len(msg.alerts) > 1:
			m.setStatus(fmt.Sprintf("got %d alerts for incident", len(msg.alerts)))
		}

		m.selectedIncidentAlerts = msg.alerts
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	// Nothing directly calls this yet
	case updateIncidentListMsg:
		debug("updateIncidentListMsg", fmt.Sprint(msg))
		m.setStatus(loadingIncidentsStatus)
		cmds = append(cmds, updateIncidentList(m.config))

	case updatedIncidentListMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		getIDs := func(x []pagerduty.Incident) []string {
			var ids []string
			for _, n := range x {
				ids = append(ids, n.ID)
			}
			return ids
		}

		debug("updatedIncidentListMsg", strings.Join(getIDs(msg.incidents), ", "))

		m.incidentList = msg.incidents

		var totalIncidentCount int
		var rows []table.Row

		for _, i := range msg.incidents {
			totalIncidentCount++
			if m.teamMode {
				rows = append(rows, table.Row{acknowledged(i.Acknowledgements), i.ID, i.Title, i.Service.Summary})
			} else {
				if AssignedToUser(i, m.config.CurrentUser.ID) {
					rows = append(rows, table.Row{acknowledged(i.Acknowledgements), i.ID, i.Title, i.Service.Summary})
				}
			}
		}

		m.table.SetRows(rows)

		if totalIncidentCount == 1 {
			m.setStatus(fmt.Sprintf("showing %d/%d incident...", len(m.table.Rows()), totalIncidentCount))
		} else {
			m.setStatus(fmt.Sprintf("showing %d/%d incidents...", len(m.table.Rows()), totalIncidentCount))
		}

	case editorFinishedMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		debug("editorFinishedMsg", fmt.Sprint(msg))

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

		debug("addedIncidentNoteMsg", fmt.Sprint(msg))

		if m.selectedIncident == nil {
			m.setStatus("unable to refresh incident - no selected incident")
			return m, nil
		}
		cmds = append(cmds, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) })

	case loginMsg:
		debug("loginMsg", fmt.Sprint(msg))
		if len(m.selectedIncidentAlerts) == 0 {
			debug(fmt.Sprintf("no alerts found for incident %s - requeuing", m.selectedIncident.ID))
			return m, func() tea.Msg { return loginMsg("sender: loginMsg; requeue") }

		}
		if len(m.selectedIncidentAlerts) == 1 {
			cluster := getDetailFieldFromAlert("cluster_id", m.selectedIncidentAlerts[0])
			m.setStatus(fmt.Sprintf("logging into cluster %s", cluster))
			return m, func() tea.Msg { return login(cluster, m.launcher) }
		}

		// TODO https://github.com/clcollins/srepd/issues/1: Figure out how to prompt with list to select from
		cluster := getDetailFieldFromAlert("cluster_id", m.selectedIncidentAlerts[0])
		m.setStatus(fmt.Sprintf("multiple alerts for incident - logging into cluster %s from first alert %s", cluster, m.selectedIncidentAlerts[0].ID))
		return m, func() tea.Msg { return login(cluster, m.launcher) }

	case loginFinishedMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

	case openBrowserMsg:
		debug("openBrowserMsg", fmt.Sprint(msg))
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
	//
	case waitForSelectedIncidentThenDoMsg:
		debug("waitForSelectedIncidentThenDoMsg: ", fmt.Sprintf("%+v, %+v", msg.action, msg.msg))
		if msg.action == nil {
			m.setStatus("failed to perform action: no action included in msg")
			return m, nil
		}
		if msg.msg == nil {
			m.setStatus("failed to perform action: no data included in msg")
			return m, nil
		}

		if m.selectedIncident == nil {
			time.Sleep(waitTime)
			m.setStatus("waiting for incident info...")
			return m, func() tea.Msg { return waitForSelectedIncidentThenDoMsg{action: msg.action, msg: msg.msg} }
		}

		debug("Trying to do action")
		cmds = append(cmds, msg.action)

	case renderIncidentMsg:
		debug("renderIncidentMsg", fmt.Sprint(msg))
		if m.selectedIncident == nil {
			m.setStatus("failed render incidents - no incidents provided")
			return m, nil
		}

		cmds = append(cmds, renderIncident(&m))

	case renderedIncidentMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		debug("renderedIncidentMsg")
		m.incidentViewer.SetContent(msg.content)
		m.viewingIncident = true

	case acknowledgeIncidentsMsg:
		debug("acknowledgeIncidentsMsg", fmt.Sprint(msg))
		if msg.incidents == nil {
			m.setStatus("failed acknowledging incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			acknowledgeIncidents(m.config, msg.incidents),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: acknowledgeIncidentsMsg") },
		)

	case acknowledgedIncidentsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		debug("acknowledgedIncidentsMsg", fmt.Sprint(msg))
		m.setStatus(fmt.Sprintf("acknowledged incidents %v; refreshing Incident List ", msg))
		return m, func() tea.Msg { return updateIncidentListMsg("sender: acknowledgedIncidentsMsg") }

	case waitForSelectedIncidentsThenAcknowledgeMsg:
		debug("waitForSelectedIncidentsThenAcknowledgeMsg", fmt.Sprint(msg))
		if m.selectedIncident == nil {
			time.Sleep(waitTime)
			m.setStatus("waiting for incident info...")
			return m, func() tea.Msg { return waitForSelectedIncidentsThenAcknowledgeMsg(msg) }
		}
		return m, func() tea.Msg {
			return acknowledgeIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}}
		}

	case reassignIncidentsMsg:
		debug("reassignIncidentsMsg", fmt.Sprint(msg))
		if msg.incidents == nil {
			m.setStatus("failed reassigning incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			reassignIncidents(m.config, msg.incidents, msg.users),
			func() tea.Msg { return clearSelectedIncidentsMsg("clear incidents") },
		)

	case reassignedIncidentsMsg:
		debug("reassignedIncidentsMsg", fmt.Sprint(msg))
		m.setStatus(fmt.Sprintf("reassigned incidents %v; refreshing Incident List ", msg))
		return m, func() tea.Msg { return updateIncidentListMsg("sender: reassignedIncidentsMsg") }

	case silenceIncidentsMsg:
		debug("silenceIncidentsMsg", fmt.Sprint(msg))
		if (msg.incidents == nil) && (m.selectedIncident == nil) {
			m.setStatus("failed silencing incidents - no incidents provided")
			return m, nil
		}

		var incidents []*pagerduty.Incident = msg.incidents
		var users []*pagerduty.User
		incidents = append(incidents, m.selectedIncident)
		users = append(users, m.config.SilentUser)
		return m, tea.Sequence(
			silenceIncidents(incidents, users),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: silenceIncidentsMsg") },
		)

	case clearSelectedIncidentsMsg:
		debug("clearSelectedIncidentsMsg", fmt.Sprint(msg))
		m.viewingIncident = false
		m.selectedIncident = nil
		m.selectedIncidentNotes = nil
		m.selectedIncidentAlerts = nil
		return m, nil
	}

	return m, tea.Batch(cmds...)

}
