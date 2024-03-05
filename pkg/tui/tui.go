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

var debugLogging = false

const waitTime = time.Millisecond * 1
const defaultInputPrompt = " $ "

func debug(msg ...string) {
	if !debugLogging {
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

	config   *pd.Config
	editor   []string
	launcher ClusterLauncher

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

func InitialModel(
	// "d" is to avoid a conflict with the "debug" function
	d bool,
	token string,
	teams []string,
	user string,
	ignoredusers []string,
	editor []string,
	launcher ClusterLauncher,
) (tea.Model, tea.Cmd) {
	if d {
		debugLogging = true
	}

	debug("InitialModel")
	var err error

	m := model{
		editor:   editor,
		launcher: launcher,
		help:     newHelp(),
		table:    newTableWithStyles(),
		input:    newTextInput(),
		// INCIDENTVIEWER
		incidentViewer: newIncidentViewer(),
		status:         "",
	}

	// This is an ugly way to handle this error
	// We have to set the m.err here instead of how the errMsg is handled
	// because the Init() occurs before the Update() and the errMsg is not
	// preserved
	pd, err := pd.NewConfig(token, teams, user, ignoredusers)
	m.config = pd
	m.err = err

	return m, func() tea.Msg {
		return errMsg{err}
	}
}

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
		debug(fmt.Sprintf("errMsg: %v", msg))
		m.setStatus(msg.Error())
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

		m.incidentViewer.Width = windowSize.Width - borderEdges
		m.incidentViewer.Height = height

	case tea.KeyMsg:
		debug("tea.KeyMsg", fmt.Sprint(msg))
		if key.Matches(msg, defaultKeyMap.Quit) {
			return m, tea.Quit
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

	// Command to get an incident by ID
	case getIncidentMsg:
		debug("getIncidentMsg", fmt.Sprint(msg))
		m.setStatus(fmt.Sprintf("getting details for incident %v...", msg))
		cmds = append(cmds, getIncident(m.config, string(msg)))

	// Set the selected incident to the incident returned from the getIncident command
	case gotIncidentMsg:
		debug("gotIncidentMsg", "TRUNCATED")
		if msg.err != nil {
			return m, func() tea.Msg {
				return errMsg{msg.err}
			}
		}

		m.setStatus(fmt.Sprintf("got incident %s", msg.incident.ID))
		m.selectedIncident = msg.incident
		return m, tea.Batch(
			getIncidentAlerts(m.config, msg.incident.ID),
			getIncidentNotes(m.config, msg.incident.ID),
		)

	case gotIncidentNotesMsg:
		debug("gotIncidentNotesMsg", "TRUNCATED")
		if msg.err != nil {
			return m, func() tea.Msg {
				return errMsg{msg.err}
			}
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
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	case gotIncidentAlertsMsg:
		debug("gotIncidentAlertsMsg", "TRUNCATED")
		if msg.err != nil {
			return m, func() tea.Msg {
				return errMsg{msg.err}
			}
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
		if m.viewingIncident {
			cmds = append(cmds, renderIncident(&m))
		}

	// Nothing directly calls this yet
	case updateIncidentListMsg:
		debug("updateIncidentListMsg", fmt.Sprint(msg))
		m.setStatus(loadingIncidentsStatus)
		cmds = append(cmds, updateIncidentList(m.config))

	case updatedIncidentListMsg:
		debug("updatedIncidentListMsg", "TRUNCATED")
		if msg.err != nil {
			return m, func() tea.Msg {
				return errMsg{msg.err}
			}
		}

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
		debug("editorFinishedMsg", fmt.Sprint(msg))
		if msg.err != nil {
			return m, func() tea.Msg {
				return errMsg{msg.err}
			}
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

		// TODO https://github.com/clcollins/srepd/issues/2: Figure out how to use an interface for the msg.action to write this once
		// cmds = append(cmds, func() tea.Msg { return msg.action(msg.msg) })

		switch msg.action {
		// TODO https://github.com/clcollins/srepd/issues/2: See TODO above
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
		// TODO: The "ACTION" here needs to be a tea.Msg, not a string
		case "loginMsg":
			return m, func() tea.Msg { return loginMsg("login") }
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
		// TODO https://github.com/clcollins/srepd/issues/3 - check the msg.err properly
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
		// TODO - https://github.com/clcollins/srepd/issues/2: This needs to be a tea.Cmd to allow waitForSelectedIncidentsThenDo
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
	case m.err != nil:
		debug("error")
		return (errorStyle.Render(dot+"ERROR"+dot+"\n\n"+m.err.Error()) + "\n" + helpView)

	case m.viewingIncident:
		debug("viewingIncident")
		return mainStyle.Render(m.renderHeader() + "\n" + m.incidentViewer.View() + "\n" + helpView)
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

// tableFocusMode is the main mode for the application
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

		case key.Matches(msg, defaultKeyMap.Top):
			m.table.GotoTop()

		case key.Matches(msg, defaultKeyMap.Bottom):
			m.table.GotoBottom()

		case key.Matches(msg, defaultKeyMap.Enter):
			m.viewingIncident = true
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
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

		case key.Matches(msg, defaultKeyMap.Login):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentThenDoMsg{action: "loginMsg", msg: "wait"} },
			)

		case key.Matches(msg, defaultKeyMap.Input):
			return m, tea.Sequence(
				m.input.Focus(),
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
		case key.Matches(msg, defaultKeyMap.Help):
			m.help.ShowAll = !m.help.ShowAll

		case key.Matches(msg, defaultKeyMap.Back):
			m.input.Blur()
			m.table.Focus()
			m.input.Prompt = defaultInputPrompt
			return m, nil

		case key.Matches(msg, defaultKeyMap.Enter):

		}
	}
	return m, tea.Batch(cmds...)
}

func switchIncidentFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	debug("switchIncidentFocusMode")
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.help.ShowAll = !m.help.ShowAll

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

		case key.Matches(msg, defaultKeyMap.Login):
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentThenDoMsg{action: "loginMsg", msg: "wait"} },
			)
		}
	}

	m.incidentViewer, cmd = m.incidentViewer.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func switchErrorFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
	debug("switchErrorFocusMode")
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Help):
			m.help.ShowAll = !m.help.ShowAll

		case key.Matches(msg, defaultKeyMap.Back):
			m.err = nil
			m.setStatus("")
		}
	}
	return m, nil
}
