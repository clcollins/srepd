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
	t := table.New(table.WithFocused(true))
	t.SetStyles(tableStyle)
	return t
}

func newTextInput() textinput.Model {
	i := textinput.New()
	i.Prompt = " $ "
	i.CharLimit = 32
	i.Width = 50
	return i
}

func newHelp() help.Model {
	h := help.New()
	h.ShowAll = false
	return h
}

func newIncidentViewer() viewport.Model {
	vp := viewport.New(windowSize.Width, windowSize.Height-5)
	vp.Style = incidentViewerStyle
	return vp
}

func InitialModel(token string, teams []string, user string, ignoredusers []string, editor string) (tea.Model, tea.Cmd) {
	var err error

	m := model{
		editor:         editor,
		help:           newHelp(),
		table:          newTableWithStyles(),
		input:          newTextInput(),
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
	return tea.Batch(
		updateIncidentList(m.config),
		getCurrentUser(m.config),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case errMsg:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
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

	case tea.KeyMsg:
		if key.Matches(msg, defaultKeyMap.Quit) {
			return m, tea.Quit
		}

		// Default commands for the table view
		switch {
		case m.selectedIncident != nil:
			return switchIncidentFocusedMode(m, msg)
		case m.table.Focused():
			return switchTableFocusMode(m, msg)
		case m.input.Focused():
			return switchInputFocusMode(m, msg)

		}

	// Command to get an incident by ID
	case getIncidentMsg:
		m.status = fmt.Sprintf("getting incident %s...", msg)
		cmds = append(cmds, getIncident(m.config, string(msg)))

	// Set the selected incident to the incident returned from the getIncident command
	case gotIncidentMsg:
		if msg.err != nil {
			log.Fatal(msg.err)
			m.status = msg.err.Error()
			return m, nil
		}

		m.status = fmt.Sprintf("got incident %s", msg.incident.ID)
		m.selectedIncident = msg.incident

	case gotIncidentNotesMsg:
		if msg.err != nil {
			log.Fatal(msg.err)
			m.status = msg.err.Error()
			return m, nil
		}

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		m.status = fmt.Sprintf("got %d notes for incident", len(msg.notes))
		m.selectedIncidentNotes = msg.notes

	case gotIncidentAlertsMsg:
		if msg.err != nil {
			log.Fatal(msg.err)
			m.status = msg.err.Error()
			return m, nil
		}

		// CANNOT refer to the m.SelectedIncident, because it may not have
		// completed yet, and will be nil
		m.status = fmt.Sprintf("got %d alerts for incident", len(msg.alerts))
		m.selectedIncidentAlerts = msg.alerts

	// Command to get the current user
	case getCurrentUserMsg:
		m.status = gettingUserStatus
		cmds = append(cmds, getCurrentUser(m.config))

	// Set the current user to the user returned from the getCurrentUser command
	case gotCurrentUserMsg:
		m.currentUser = msg.user
		if msg.err != nil {
			log.Fatal(msg.err)
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("got user %s", m.currentUser.Email)

	// Nothing directly calls this yet
	case updateIncidentListMsg:
		m.status = loadingIncidentsStatus
		cmds = append(cmds, updateIncidentList(m.config))

	case updatedIncidentListMsg:
		if msg.err != nil {
			log.Fatal(msg.err)
			m.status = msg.err.Error()
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
					rows = append(rows, table.Row{"", i.ID, i.Title, i.Service.Summary})
				}
			} else {
				if AssignedToUser(i, m.currentUser.ID) {
					rows = append(rows, table.Row{"", i.ID, i.Title, i.Service.Summary})
				}
			}
		}
		m.table.SetRows(rows)
		if len(msg.incidents) == 1 {
			m.status = fmt.Sprintf("retrieved %d incident...", len(m.table.Rows()))
		} else {
			m.status = fmt.Sprintf("retrieved %d incidents...", len(m.table.Rows()))
		}

	case editorFinishedMsg:
		if msg.err != nil {
			log.Fatal(msg.err)
			m.status = msg.err.Error()
			return m, nil
		}

		cmds = append(cmds, addNoteToIncident(m.config, m.selectedIncident, m.currentUser, msg.file))

	case waitForSelectedIncidentThenRenderMsg:
		if m.selectedIncident == nil {
			time.Sleep(time.Second * 1)
			m.status = "waiting for incident info..."
			return m, func() tea.Msg { return waitForSelectedIncidentThenRenderMsg(msg) }
		}
		renderIncidentMarkdown(m.template())
		m.incidentViewer.SetContent(m.template())
		m.viewingIncident = true

	case waitForSelectedIncidentsThenAnnotateMsg:
		if m.selectedIncident == nil {
			time.Sleep(time.Second * 1)
			m.status = "waiting for incident info..."
			return m, func() tea.Msg { return waitForSelectedIncidentsThenAnnotateMsg(msg) }
		}
		cmds = append(cmds, openEditorCmd(m.editor))

	case acknowledgeIncidentsMsg:
		return m, acknowledgeIncidents(m.config, msg.incidents)

	case waitForSelectedIncidentsThenAcknowledgeMsg:
		if m.selectedIncident == nil {
			time.Sleep(time.Second * 1)
			m.status = "waiting for incident info..."
			return m, func() tea.Msg { return waitForSelectedIncidentsThenAcknowledgeMsg(msg) }
		}
		return m, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }

	case reassignIncidentsMsg:
		return m, reassignIncidents(m.config, msg.incidents, m.currentUser, msg.users)

	case reassignedIncidentsMsg:
		m.status = fmt.Sprintf("reassigned incidents %v; refreshing Incident List ", msg)
		return m, func() tea.Msg { return updateIncidentListMsg("get incidents") }

	case silenceIncidentsMsg:
		var incidents []*pagerduty.Incident = msg.incidents
		var users []*pagerduty.User
		incidents = append(incidents, m.selectedIncident)
		users = append(users, m.config.SilentUser)
		return m, tea.Sequence(
			silenceIncidents(incidents, users),
			func() tea.Msg { return clearSelectedIncidentsMsg("clear incidents") },
		)

	case waitForSelectedIncidentsThenSilenceMsg:
		if m.selectedIncident == nil {
			time.Sleep(time.Second * 1)
			m.status = "waiting for incident info..."
			return m, func() tea.Msg { return waitForSelectedIncidentsThenSilenceMsg(msg) }
		}
		return m, func() tea.Msg { return silenceIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }

	case clearSelectedIncidentsMsg:
		m.selectedIncident = nil
		m.selectedIncidentNotes = nil
		m.selectedIncidentAlerts = nil
		return m, nil
	}

	return m, tea.Batch(cmds...)

}

func (m model) View() string {
	helpView := helpStyle.Render(m.help.View(defaultKeyMap))
	tableView := tableContainerStyle.Render(m.table.View())

	switch {
	// case m.input.Focused():
	// 	return mainStyle.Render(m.renderHeader() + "\n" + tableView + "\n" + m.input.View() + "\n" + helpView)
	case m.viewingIncident:
		return mainStyle.Render(m.renderHeader() + "\n" + m.incidentViewer.View() + "\n" + helpView)
	default:
		return mainStyle.Render(m.renderHeader() + "\n" + tableView + "\n" + helpView)
	}
}

// tableFocusedMode is the main mode for the application
func switchTableFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
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
			//renderIncidentMarkdown(m.template())
			return m, tea.Sequence(
				func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
				func() tea.Msg { return waitForSelectedIncidentThenRenderMsg("wait") },
			)

		case key.Matches(msg, defaultKeyMap.Team):
			m.teamMode = !m.teamMode
			cmds = append(cmds, func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} })

		case key.Matches(msg, defaultKeyMap.Refresh):
			m.status = loadingIncidentsStatus
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
		}
	}
	return m, tea.Batch(cmds...)
}

func switchInputFocusMode(m model, msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case key.Matches(msg, defaultKeyMap.Up):
			m.incidentViewer.LineUp(1)
		case key.Matches(msg, defaultKeyMap.Down):
			m.incidentViewer.LineDown(1)
		case key.Matches(msg, defaultKeyMap.Ack):
			return m, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }
		case key.Matches(msg, defaultKeyMap.Silence):
			return m, func() tea.Msg { return silenceIncidentsMsg{incidents: []*pagerduty.Incident{m.selectedIncident}} }
		case key.Matches(msg, defaultKeyMap.Note):
			cmds = append(cmds, openEditorCmd(m.editor))
		}
	}
	return m, tea.Batch(cmds...)
}
