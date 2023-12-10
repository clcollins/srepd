package tui

import (
	"fmt"
	"log"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
)

const DEBUG = true
const waitTime = time.Millisecond * 1

func debug(msg ...string) {
	if !DEBUG {
		return
	}
	log.Printf("%s\n", msg)
}

func (m *model) setStatus(msg string) {
	var d []string

	m.status = fmt.Sprint(msg)

	d = append(d, "setStatus")
	d = append(d, msg)

	log.Printf("%s\n", d)
}

type errMsg struct{ error }

type model struct {
	err error

	config *pd.Config
	editor string

	currentUser *pagerduty.User

	table table.Model
	input textinput.Model
	// This is a hack since viewport.Model doesn't have a Focused() method
	viewingIncident bool
	incidentViewer  viewport.Model
	help            help.Model

	status string

	incidentList           []pagerduty.Incident
	selectedIncident       *pagerduty.Incident
	selectedIncidentNotes  []pagerduty.IncidentNote
	selectedIncidentAlerts []pagerduty.IncidentAlert

	teamMode bool
}

func newTableWithStyles() table.Model {
	debug("newTableWithStyles")
	t := table.New(table.WithFocused(true))
	t.SetStyles(tableStyle)
	return t
}

func newTextInput() textinput.Model {
	debug("newTextInput")
	i := textinput.New()
	i.Prompt = " $ "
	i.CharLimit = 32
	i.Width = 50
	return i
}

func newHelp() help.Model {
	debug("newHelp")
	h := help.New()
	h.ShowAll = false
	return h
}

func newIncidentViewer() viewport.Model {
	debug("newIncidentViewer")
	vp := viewport.New(100, 100)
	vp.Style = incidentViewerStyle
	return vp
}

func InitialModel(token string, teams []string, user string, ignoredusers []string, editor string) (tea.Model, tea.Cmd) {
	debug("InitialModel")
	var err error

	m := model{
		editor: editor,
		help:   newHelp(),
		table:  newTableWithStyles(),
		input:  newTextInput(),
		// INCIDENTVIEWER
		incidentViewer: newIncidentViewer(),
		status:         loadingIncidentsStatus,
	}

	// This is an ugly way to handle this error
	pd, err := pd.NewConfig(token, teams, user, ignoredusers)
	if err != nil {
		panic(err)
	}
	m.config = pd

	return m, func() tea.Msg {
		return errMsg{err}
	}
}

func (m model) Init() tea.Cmd {
	debug("Init")
	return tea.Batch(
		updateIncidentList(m.config),
		getCurrentUser(m.config),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	debug("Update")
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case errMsg:
		debug("errMsg")
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		debug("tea.WindowSizeMsg")
		windowSize = msg
		top, _, bottom, _ := mainStyle.GetMargin()
		eighthWindow := windowSize.Width / 8
		cellPadding := (horizontalPadding * 2) * 4
		borderEdges := 2 + 10

		m.help.Width = windowSize.Width - borderEdges

		m.table.SetColumns([]table.Column{
			{Title: dot, Width: 1},
			{Title: "ID", Width: eighthWindow + cellPadding - borderEdges},
			{Title: "Summary", Width: eighthWindow * 3},
			{Title: "Service", Width: eighthWindow * 3},
		})

		height := windowSize.Height - top - bottom - 10

		m.table.SetHeight(height)

		// INCIDENTVIEWER
		m.incidentViewer.Width = windowSize.Width - borderEdges
		m.incidentViewer.Height = height

	case tea.KeyMsg:
		debug("tea.KeyMsg", fmt.Sprint(msg))
		if key.Matches(msg, defaultKeyMap.Quit) {
			return m, tea.Quit
		}

		// Default commands for the table view
		switch {
		case m.viewingIncident:
			return switchIncidentFocusedMode(m, msg)
		case m.table.Focused():
			return switchTableFocusMode(m, msg)
		case m.input.Focused():
			return switchInputFocusMode(m, msg)
		}

	// Command to get an incident by ID
	case getIncidentMsg:
		debug("getIncidentMsg", fmt.Sprint(msg))
		m.setStatus(fmt.Sprintf("getting details for incident %v...", msg))
		cmds = append(cmds, getIncident(m.config, string(msg)))

	// Set the selected incident to the incident returned from the getIncident command
	case gotIncidentMsg:
		debug("gotIncidentMsg", fmt.Sprint("TRUNCATED"))
		if msg.err != nil {
			m.setStatus(msg.err.Error())
			log.Fatal(msg.err)
			return m, nil
		}

		m.setStatus(fmt.Sprintf("got incident %s", msg.incident.ID))
		m.selectedIncident = msg.incident
		return m, tea.Batch(
			getIncidentAlerts(m.config, msg.incident.ID),
			getIncidentNotes(m.config, msg.incident.ID),
		)

	case gotIncidentNotesMsg:
		debug("gotIncidentNotesMsg", fmt.Sprint("TRUNCATED"))
		if msg.err != nil {
			m.setStatus(msg.err.Error())
			log.Fatal(msg.err)
			return m, nil
		}

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		m.setStatus(fmt.Sprintf("got %d notes for incident", len(msg.notes)))
		m.selectedIncidentNotes = msg.notes
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	case gotIncidentAlertsMsg:
		//debug("gotIncidentAlertsMsg", fmt.Sprint("TRUNCATED"))
		debug("gotIncidentAlertsMsg", fmt.Sprint(msg))
		if msg.err != nil {
			m.setStatus(msg.err.Error())
			log.Fatal(msg.err)
			return m, nil
		}

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		m.setStatus(fmt.Sprintf("got %d alerts for incident", len(msg.alerts)))
		m.selectedIncidentAlerts = msg.alerts
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	// Command to get the current user
	case getCurrentUserMsg:
		debug("getCurrentUserMsg", fmt.Sprint(msg))
		m.setStatus(gettingUserStatus)
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	// Set the current user to the user returned from the getCurrentUser command
	case gotCurrentUserMsg:
		debug("gotCurrentUserMsg", fmt.Sprint(msg.user.ID))
		m.currentUser = msg.user
		if msg.err != nil {
			m.setStatus(msg.err.Error())
			log.Fatal(msg.err)
			return m, nil
		}
		m.setStatus(fmt.Sprintf("got user %s", m.currentUser.Email))

	// Nothing directly calls this yet
	case updateIncidentListMsg:
		debug("updateIncidentListMsg", fmt.Sprint(msg))
		m.setStatus(loadingIncidentsStatus)
		cmds = append(cmds, updateIncidentList(m.config))

	case updatedIncidentListMsg:
		debug("updatedIncidentListMsg", fmt.Sprint("TRUNCATED"))
		if msg.err != nil {
			m.setStatus(msg.err.Error())
			log.Fatal(msg.err)
			return m, nil
		}
		m.incidentList = msg.incidents
		var rows []table.Row

		var ignoredUsersList []string
		for _, i := range m.config.IgnoredUsers {
			ignoredUsersList = append(ignoredUsersList, i.ID)
		}
		for _, i := range msg.incidents {
			if m.teamMode {
				if !AssignedToAnyUsers(i, ignoredUsersList) {
					rows = append(rows, table.Row{dot, i.ID, i.Title, i.Service.Summary})
				}
			} else {
				if AssignedToUser(i, m.currentUser.ID) {
					acked := "T"
					for _, a := range i.Acknowledgements {
						debug(fmt.Sprintf("Acknowledger ID: %v, CurrentUserID: %v", a.Acknowledger.ID, m.currentUser.ID))
						if a.Acknowledger.ID == m.currentUser.ID {
							acked = "A"
						}
					}
					rows = append(rows, table.Row{acked, i.ID, i.Title, i.Service.Summary})
				}
			}
		}
		m.table.SetRows(rows)
		if len(msg.incidents) == 1 {
			m.setStatus(fmt.Sprintf("retrieved %d incident...", len(m.table.Rows())))
		} else {
			m.setStatus(fmt.Sprintf("retrieved %d incidents...", len(m.table.Rows())))
		}

	case editorFinishedMsg:
		debug("editorFinishedMsg", fmt.Sprint(msg))
		if msg.err != nil {
			m.setStatus(msg.err.Error())
			log.Fatal(msg.err)
			return m, nil
		}

		if m.selectedIncident == nil {
			m.setStatus("failed to add note - no selected incident")
			return m, nil
		}

		cmds = append(cmds, addNoteToIncident(m.config, m.selectedIncident, msg.file))

	// Refresh the local copy of the incident after the note is added
	case addedIncidentNoteMsg:
		debug("addedIncidentNoteMsg", fmt.Sprint(msg))

		if m.selectedIncident == nil {
			m.setStatus("unable to refresh incident - no selected incident")
			return m, nil
		}
		cmds = append(cmds, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) })

	case openOCMContainerMsg:
		debug("openOCMContainerMsg", fmt.Sprint(msg))
		if len(m.selectedIncidentAlerts) == 0 {
			debug(fmt.Sprintf("no alerts found for incident %s - requeuing", m.selectedIncident.ID))
			return m, func() tea.Msg { return openOCMContainerMsg("sender: openOCMContainerMsg; requeue") }

		}
		if len(m.selectedIncidentAlerts) == 1 {
			cluster := getDetailFieldFromAlert("cluster_id", m.selectedIncidentAlerts[0])
			m.setStatus(fmt.Sprintf("opening cluster %s", cluster))
			return m, func() tea.Msg { return openOCMContainer(cluster) }
		}

		// TODO: Figure out how to prompt with list to select from
		cluster := getDetailFieldFromAlert("cluster_id", m.selectedIncidentAlerts[0])
		m.setStatus(fmt.Sprintf("multiple alerts for incident - opening cluster %s from first alert %s", cluster, m.selectedIncidentAlerts[0].ID))
		return m, func() tea.Msg { return openOCMContainer(cluster) }

	case waitForSelectedIncidentThenDoMsg:
		debug("waitForSelectedIncidentThenDoMsg", fmt.Sprint(msg.action, msg.msg))
		if msg.action == "" {
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
			return m, func() tea.Msg { return waitForSelectedIncidentThenDoMsg(msg) }
		}

		// TODO: Figure out how to use an interface for the msg.action to write this once
		// cmds = append(cmds, func() tea.Msg { return msg.action(msg.msg) })

		switch msg.action {
		// TODO: See TODO above
		// case "acknowledgeIncidentsMsg":
		// 	if msg.msg.incidents == nil {
		// 		m.setStatus("failed acknowledging incidents - no incidents provided")
		// 		return m, nil
		// 	}

		// 	return m, tea.Sequence(
		// 		acknowledgeIncidents(m.config, msg.incidents),
		// 		func() tea.Msg { return clearSelectedIncidentsMsg("sender: acknowledgeIncidentsMsg") },
		// 	)
		// case "annotateIncidentsMsg":
		case "openOCMContainerMsg":
			return m, func() tea.Msg { return openOCMContainerMsg("open") }
		// case "reassignIncidentsMsg":
		case "renderIncidentMsg":
			return m, func() tea.Msg { return renderIncidentMsg("render") }
		// case "silenceIncidentsMsg":
		default:
			debug(fmt.Sprintf("%v not implemented", msg.action))
			return m, nil
		}

	case renderIncidentMsg:
		debug("renderIncidentMsg", fmt.Sprint(msg))
		if m.selectedIncident == nil {
			m.setStatus("failed render incidents - no incidents provided")
			return m, nil
		}

		cmds = append(cmds, renderIncident(&m))

	case renderedIncidentMsg:
		debug("renderedIncidentMsg", fmt.Sprint(msg))
		// TODO - check the msg.err properly
		// not in the renderIncident() function
		m.incidentViewer.SetContent(msg.content)
		m.viewingIncident = true

	case waitForSelectedIncidentsThenAnnotateMsg:
		debug("waitForSelectedIncidentsThenAnnotateMsg", fmt.Sprint(msg))
		if m.selectedIncident == nil {
			time.Sleep(waitTime)
			m.setStatus("waiting for incident info...")
			return m, func() tea.Msg { return waitForSelectedIncidentsThenAnnotateMsg(msg) }
		}
		// TODO: This needs to be a tea.Cmd to allow waitForSelectedIncidentsThenDo
		cmds = append(cmds, openEditorCmd(m.editor))

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

	// There is no "silencedIncidentsMsg" - silences are really reassignments under the hood

	case waitForSelectedIncidentsThenSilenceMsg:
		debug("waitForSelectedIncidentsThenSilenceMsg", fmt.Sprint(msg))
		if m.selectedIncident == nil {
			time.Sleep(waitTime)
			m.setStatus("waiting for incident info...")
			return m, func() tea.Msg { return waitForSelectedIncidentsThenSilenceMsg(msg) }
		}
		return m, func() tea.Msg { return silenceIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }

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

func (m model) View() string {
	debug("View")
	helpView := helpStyle.Render(m.help.View(defaultKeyMap))

	switch {
	case m.viewingIncident:
		debug("viewingIncident")
		// INCIDENTVIEWER
		//m.incidentViewer.Width = 50
		//m.incidentViewer.Height = 50
		return mainStyle.Render(m.renderHeader() + "\n" + m.incidentViewer.View() + "\n" + helpView)
		//return mainStyle.Render(m.renderHeader() + "\n" + m.incidentViewer + "\n" + helpView)
	default:
		tableView := tableContainerStyle.Render(m.table.View())
		if m.input.Focused() {
			debug("viewingTable and input")
			return mainStyle.Render(m.renderHeader() + "\n" + tableView + "\n" + m.input.View() + "\n" + helpView)
		}
		debug("viewingTable")
		return mainStyle.Render(m.renderHeader() + "\n" + tableView + "\n" + helpView)
	}
}

// tableFocusedMode is the main mode for the application
func switchTableFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	debug("switchTableFocusMode")
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.help.ShowAll = !m.help.ShowAll

		case key.Matches(msg, defaultKeyMap.Up):
			m.table.MoveUp(1)

		case key.Matches(msg, defaultKeyMap.Down):
			m.table.MoveDown(1)

		case key.Matches(msg, defaultKeyMap.Enter):
			m.viewingIncident = true
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				// func() tea.Msg { return waitForSelectedIncidentThenRenderMsg("wait") },
				func() tea.Msg { return waitForSelectedIncidentThenDoMsg{action: "renderIncidentMsg", msg: "render"} },
			)

		case key.Matches(msg, defaultKeyMap.Team):
			m.teamMode = !m.teamMode
			cmds = append(cmds, func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} })

		case key.Matches(msg, defaultKeyMap.Refresh):
			m.setStatus(loadingIncidentsStatus)
			cmds = append(cmds, updateIncidentList(m.config))

		// In table mode, highlighted incidents are not selected yet, so they need to be retrieved
		// and then can be acted upon.  Since tea.Sequence does not wait for completion, the
		// "waitForSelectedIncidentsThen..." functions are used to wait for the selected incident
		// to be retrieved from PagerDuty
		case key.Matches(msg, defaultKeyMap.Silence):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentsThenSilenceMsg("wait") },
			)

		case key.Matches(msg, defaultKeyMap.Ack):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentsThenAcknowledgeMsg("wait") },
			)

		case key.Matches(msg, defaultKeyMap.Note):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentsThenAnnotateMsg("wait") },
			)

		case key.Matches(msg, defaultKeyMap.Open):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentThenDoMsg{action: "openOCMContainerMsg", msg: "wait"} },
			)
		}
	}
	return m, tea.Batch(cmds...)
}

func switchInputFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	debug("switchInputFocusMode")
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Enter):

		}
	}
	return m, tea.Batch(cmds...)
}

func switchIncidentFocusedMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	debug("switchIncidentFocusedMode")
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		// This un-sets the selected incident and returns to the table view
		case key.Matches(msg, defaultKeyMap.Back):
			m.viewingIncident = false
			m.selectedIncident = nil
			m.selectedIncidentAlerts = nil
			m.selectedIncidentNotes = nil

		case key.Matches(msg, defaultKeyMap.Ack):
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }

		case key.Matches(msg, defaultKeyMap.Silence):
			return m, func() tea.Msg { return silenceIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }

		case key.Matches(msg, defaultKeyMap.Note):
			cmds = append(cmds, openEditorCmd(m.editor))

		case key.Matches(msg, defaultKeyMap.Refresh):
			return m, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) }

		case key.Matches(msg, defaultKeyMap.Open):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentThenDoMsg{action: "openOCMContainerMsg", msg: "wait"} },
			)
		}
	}

	// INCIDENTVIEWER
	m.incidentViewer, cmd = m.incidentViewer.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
