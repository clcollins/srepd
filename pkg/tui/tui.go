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
	"github.com/charmbracelet/lipgloss"
	"github.com/clcollins/srepd/pkg/pd"
)

const (
	gettingUserStatus      = "getting user info..."
	loadingIncidentsStatus = "loading incidents..."
)

type errMsg struct{ error }

type Model struct {
	config *pd.Config
	editor string

	currentUser *pagerduty.User

	table          table.Model
	input          textinput.Model
	incidentViewer viewport.Model
	help           help.Model

	status string

	incidentList           []pagerduty.Incident
	selectedIncident       *pagerduty.Incident
	selectedIncidentNotes  []pagerduty.IncidentNote
	selectedIncidentAlerts []pagerduty.IncidentAlert

	teamMode bool
}

func InitialModel(token string, teams []string, user string, ignoreusers []string, editor string) (tea.Model, tea.Cmd) {
	var err error

	input := textinput.New()
	input.Prompt = " $ "
	input.CharLimit = 32
	input.Width = 50

	m := Model{
		editor: editor,
		help:   help.New(),
		table: table.New(
			table.WithFocused(true),
		),
		input:    input,
		status:   loadingIncidentsStatus,
		teamMode: false,
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	m.table.SetStyles(s)

	// This is an ugly way to handle this error
	pd, err := pd.NewConfig(token, teams, user, ignoreusers)
	m.config = pd

	return m, func() tea.Msg {
		return errMsg{err}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		updateIncidentList(m.config),
		getCurrentUser(m.config),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {

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
			{Title: "ClusterID", Width: eighthWindow * 3},
		})

		height := windowSize.Height - top - bottom - 10

		m.table.SetHeight(height)

	case tea.KeyMsg:
		// Always allow for quitting with q or Esc no matter what view is focused
		if key.Matches(msg, defaultKeyMap.Quit) {
			return m, tea.Quit
		}

		if m.selectedIncident != nil {
			switch {
			case key.Matches(msg, defaultKeyMap.Up):
			case key.Matches(msg, defaultKeyMap.Down):
			case key.Matches(msg, defaultKeyMap.Back):
				m.status = ""
				m.selectedIncident = nil
				m.selectedIncidentAlerts = nil
				m.selectedIncidentNotes = nil
			}

		}

		// Key map behavior based on what view is currently focused
		if m.input.Focused() {
			// Command for focused "input" textarea

			switch {
			case key.Matches(msg, defaultKeyMap.Enter):
				// TODO: SAVE INPUT TO VARIABLE HERE WHEN ENTER IS PRESSED
				m.input.SetValue("")
				m.input.Blur()

			case key.Matches(msg, defaultKeyMap.Back):
				m.input.SetValue("")
				m.input.Blur()
			}

			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)

		} else {

			// Default commands for the table view
			switch {

			// case SOME KEY FOR INPUT MODE:
			// 	m.input.Focus()
			// 	m.input.SetValue("View Incident by ID")
			// 	m.input.Blink()

			case key.Matches(msg, defaultKeyMap.Help):
				m.help.ShowAll = !m.help.ShowAll

			case key.Matches(msg, defaultKeyMap.Up):
				m.table.MoveUp(1)
				m.incidentViewer.LineUp(1)

			case key.Matches(msg, defaultKeyMap.Down):
				m.table.MoveDown(1)
				m.incidentViewer.LineDown(1)

			case key.Matches(msg, defaultKeyMap.Enter):
				if m.table.SelectedRow() == nil {
					m.status = "no incident selected"
					return m, nil
				}
				cmds = append(cmds,
					getIncident(m.config, m.table.SelectedRow()[1]),
					getIncidentAlerts(m.config, m.table.SelectedRow()[1]),
					getIncidentNotes(m.config, m.table.SelectedRow()[1]),
				)

			case key.Matches(msg, defaultKeyMap.Back):
				m.status = ""
				m.selectedIncident = nil
				m.selectedIncidentAlerts = nil
				m.selectedIncidentNotes = nil

			case key.Matches(msg, defaultKeyMap.Team):
				m.teamMode = !m.teamMode
				cmds = append(cmds, func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} })

			case key.Matches(msg, defaultKeyMap.Refresh):
				m.status = loadingIncidentsStatus
				cmds = append(cmds, updateIncidentList(m.config))

			case key.Matches(msg, defaultKeyMap.Note):
				cmds = append(cmds, openEditorCmd(m.editor))

			case key.Matches(msg, defaultKeyMap.Silence):
				if m.selectedIncident == nil {
					return m, tea.Sequence(
						// These fire in sequence, but because they're messages, they don't wait for the previous one to finish
						// So we need some way of telling the system to poll
						func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
						func() tea.Msg { return waitForSelectedIncidentsThenSilenceMsg("wait") },
					)
				} else {
					return m, func() tea.Msg { return silenceIncidentsMsg([]pagerduty.Incident{*m.selectedIncident}) }
				}

			case key.Matches(msg, defaultKeyMap.Ack):
				if m.selectedIncident == nil {
					return m, tea.Sequence(
						// These fire in sequence, but because they're messages, they don't wait for the previous one to finish
						// So we need some way of telling the system to poll
						func() tea.Msg { return getIncidentMsg(m.table.SelectedRow()[1]) },
						func() tea.Msg { return waitForSelectedIncidentsThenAcknowledgeMsg("wait") },
					)
				} else {
					return m, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: []pagerduty.Incident{*m.selectedIncident}} }
				}
			}
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

		var ignoreUsersList []string
		for _, i := range m.config.IgnoreUsers {
			ignoreUsersList = append(ignoreUsersList, i.ID)
		}
		for _, i := range msg.incidents {
			if m.teamMode {
				if !AssignedToAnyUsers(i, ignoreUsersList) {
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

		cmds = append(cmds, addNoteToIncident(m.config, m.selectedIncident.ID, m.currentUser, msg.file))

	case waitForSelectedIncidentsThenAcknowledgeMsg:
		if m.selectedIncident == nil {
			time.Sleep(time.Second * 1)
			m.status = "waiting for incident info..."
			return m, func() tea.Msg { return waitForSelectedIncidentsThenAcknowledgeMsg(msg) }
		}
		return m, func() tea.Msg { return acknowledgeIncidentsMsg{incidents: []pagerduty.Incident{*m.selectedIncident}} }

	case reassignIncidentsMsg:
		return m, reassignIncidents(m.config, m.currentUser.Email, msg.incidents, msg.users)

	case reassignedIncidentsMsg:
		m.status = fmt.Sprintf("reassigned incidents %v; refreshing Incident List ", msg)
		return m, func() tea.Msg { return updateIncidentListMsg("get incidents") }

	case silenceIncidentsMsg:
		var incidents []pagerduty.Incident = msg
		var users []*pagerduty.User
		incidents = append(incidents, *m.selectedIncident)
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
		return m, func() tea.Msg { return silenceIncidentsMsg([]pagerduty.Incident{*m.selectedIncident}) }

	case clearSelectedIncidentsMsg:
		m.selectedIncident = nil
		m.selectedIncidentNotes = nil
		m.selectedIncidentAlerts = nil
		return m, nil
	}

	return m, tea.Batch(cmds...)

}

func (m Model) View() string {
	helpView := helpStyle.Render(m.help.View(defaultKeyMap))

	if m.selectedIncident != nil {
		m.incidentViewer = newIncidentViewer(m.template())
		return mainStyle.Render(m.renderHeader() + "\n" + m.incidentViewer.View() + "\n" + helpView)
	}
	if m.input.Focused() {
		return mainStyle.Render(m.renderHeader() + "\n" + tableStyle.Render(m.table.View()) + "\n" + m.input.View() + "\n" + helpView)
	}
	return mainStyle.Render(m.renderHeader() + "\n" + tableStyle.Render(m.table.View()) + "\n" + helpView)
}
