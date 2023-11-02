package tui

import (
	"context"
	"fmt"
	"log"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/clcollins/srepd/pkg/pd"
)

const (
	dot                  = "•"
	upArrow              = "↑"
	downArrow            = "↓"
	refreshLogMessage    = "refreshing..."
	gettingUserMessage   = "getting user info..."
	gettingSilentUserMsg = "getting 'Silent' user info..."
)

var debug bool
var errorLog []error
var viewLog bool

// Gotta figure out how to accurately update the width on screen resize
var logArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(1).
	Align(lipgloss.Left).
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240")).
	Bold(false)

// Gotta figure out how to accurately update the width on screen resize
var assigneeArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(1).
	Align(lipgloss.Right, lipgloss.Bottom).
	BorderStyle(lipgloss.HiddenBorder())

var helpArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(1).
	Align(lipgloss.Right, lipgloss.Bottom).
	BorderStyle(lipgloss.HiddenBorder())

var incidentScreenArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(initialTableHeight+2).
	Align(lipgloss.Center, lipgloss.Center).
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240")).
	Bold(false)

var logScreenArea = lipgloss.NewStyle().
	Width(initialTableWidth).
	Height(initialTableHeight).
	Align(lipgloss.Left).
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240")).
	Bold(false)

// Type and function for capturing error messages with tea.Msg
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type model struct {
	help                  help.Model
	context               context.Context
	pdConfig              *pd.Config
	currentUser           *pagerduty.User
	table                 table.Model
	incidentList          []pagerduty.Incident
	selected              *pagerduty.Incident
	toggleCurrentUserOnly bool
	statusMessage         string
	confirm               bool
	debugMessage          string
}

func InitialModel(ctx context.Context, pdConfig *pd.Config) model {
	return model{
		help:     help.New(),
		context:  ctx,
		pdConfig: pdConfig,
	}
}

func (m model) Init() tea.Cmd {
	if debug {
		log.Print("init")
	}
	return tea.Batch(
		func() tea.Msg { return tea.ClearScreen() },
		func() tea.Msg { return createTableWithStylesMsg("create table") },
		func() tea.Msg { return getCurrentUserMsg("get user") },
		// Currently get silent user during root cmd startup; maybe better here?
		// func() tea.Msg { return getSilentUserMsg("get silent user") },
		func() tea.Msg { return getIncidentsMsg("get incidents") },
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if debug {
		log.Print("update")
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// There's a couple of things that need to update on resize
		// Gotta figure out how to do that
		m.help.Width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, defaultKeyMap.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, defaultKeyMap.Refresh):
			m.statusMessage = refreshLogMessage
			return m, getIncidents(m.context, m.pdConfig)
		case key.Matches(msg, defaultKeyMap.Up):
			m.table.MoveUp(1)
			return m, nil
		case key.Matches(msg, defaultKeyMap.Down):
			m.table.MoveDown(1)
			return m, nil
		case key.Matches(msg, defaultKeyMap.Enter):
			i := m.table.SelectedRow()[1]
			m.statusMessage = fmt.Sprintf("getting incident %s", i)
			return m, getSingleIncident(m.context, m.pdConfig, i)
		case key.Matches(msg, defaultKeyMap.Esc):
			if m.confirm {
				m.confirm = false
				return m, nil
			}
			m.selected = nil
			return m, nil
		case key.Matches(msg, defaultKeyMap.Team):
			m.toggleCurrentUserOnly = !m.toggleCurrentUserOnly
			// pass the unfiltered incident list stored in the model
			return m, func() tea.Msg { return gotIncidentsMsg(m.incidentList) }
		case key.Matches(msg, defaultKeyMap.Silence):
			// not implemented
			return m, nil
		case key.Matches(msg, defaultKeyMap.Ack):
			// not implemented
			return m, nil
		case key.Matches(msg, defaultKeyMap.Escalate):
			// not implemented
			return m, nil
		default:
			return m, nil
		}

	case errMsg:
		m.statusMessage = "ERROR: " + msg.Error()
		errorLog = append(errorLog, msg.err)
		return m, nil
	case createTableWithStylesMsg:
		if debug {
			log.Printf("createTableWithStylesMsg")
		}
		return m, createTableWithStyles()
	case createdTableWithStylesMsg:
		if debug {
			log.Printf("createdTableWithStylesMsg")
		}
		m.table = msg.table
		return m, nil
	case getCurrentUserMsg:
		if debug {
			log.Printf("getCurrentUserMsg")
		}
		m.statusMessage = gettingUserMessage
		return m, getCurrentUser(m.context, m.pdConfig)
	case gotCurrentUserMsg:
		if debug {
			log.Printf("gotCurrentUserMsg")
		}
		m.currentUser = msg
	// Just validate the silent user exists
	case getSilentUserMsg:
		m.statusMessage = gettingSilentUserMsg
		return m, getUser(m.context, m.pdConfig, m.pdConfig.SilentUser.ID)
	case gotSilentUserMsg:
		return m, nil
	// Nothing currently sends this message (see the "enter" case above, which just does the thing)
	case getSingleIncidentMsg:
		return m, getSingleIncident(m.context, m.pdConfig, m.selected.ID)
	case gotSingleIncidentMsg:
		m.statusMessage = fmt.Sprintf("got incident %s", msg.ID)
		m.selected = msg
		return m, nil
	case getIncidentsMsg:
		if debug {
			log.Printf("getIncidentsMsg")
		}
		return m, getIncidents(m.context, m.pdConfig)
	case gotIncidentsMsg:
		m.incidentList = msg
		var rows []table.Row
		m.table.SetRows(rows)
		for _, p := range m.incidentList {
			if m.toggleCurrentUserOnly {
				if AssignedToUser(p, m.currentUser.ID) {
					rows = append(rows, table.Row{"", p.ID, p.Title, p.Service.Summary})
				}
			} else {
				if !AssignedToUser(p, m.pdConfig.SilentUser.ID) {
					rows = append(rows, table.Row{"", p.ID, p.Title, p.Service.Summary})
				}
			}
		}
		m.statusMessage = fmt.Sprintf("got %d incidents", len(rows))
		m.table.SetRows(rows)
		return m, nil
	default:
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	// this viewLog is ugly; need to do this better
	viewLog = false
	if viewLog {
		var errorLogOutput string
		for _, e := range errorLog {
			errorLogOutput += fmt.Sprintf("%s\n", e.Error())
		}

		return logScreenArea.Render(errorLogOutput)
	}

	if m.selected != nil {
		// This is a cheat-y way to make the layout match, with a hidden render area
		return assigneeArea.Render("") + "\n" +
			incidentScreenArea.Render(m.selected.Summary) + "\n" +
			logArea.Render("") + "\n" +
			helpArea.Render(m.help.View(defaultKeyMap))
	}
	var assignedTo string = "Assigned to Team"

	if m.toggleCurrentUserOnly {
		assignedTo = fmt.Sprintf("Assigned to %s", m.currentUser.Name)
	}

	s := assigneeArea.Render(assignedTo)
	s += "\n" + baseStyle.Render(m.table.View()) + "\n"
	s += logArea.Render(fmt.Sprintf("msg > "+m.statusMessage)) + "\n"
	s += helpArea.Render(m.help.View(defaultKeyMap))

	return s
}

type getCurrentUserMsg string
type gotCurrentUserMsg *pagerduty.User

func getCurrentUser(ctx context.Context, pdConfig *pd.Config) tea.Cmd {
	if debug {
		log.Printf("getCurrentUser")
	}
	return func() tea.Msg {
		u, err := pdConfig.Client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})
		if err != nil {
			return errMsg{err}
		}
		return gotCurrentUserMsg(u)
	}
}

type getSilentUserMsg string
type gotSilentUserMsg *pagerduty.User

func getUser(ctx context.Context, pdConfig *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		u, err := pdConfig.Client.GetUserWithContext(ctx, id, pagerduty.GetUserOptions{})
		if err != nil {
			return errMsg{err}
		}
		return gotSilentUserMsg(u)
	}
}

func AssignedToUser(i pagerduty.Incident, id string) bool {
	for _, a := range i.Assignments {
		if a.Assignee.ID == id {
			return true
		}
	}
	return false
}

type getIncidentsMsg string
type gotIncidentsMsg []pagerduty.Incident

func getIncidents(ctx context.Context, pdConfig *pd.Config) tea.Cmd {
	if debug {
		log.Printf("getIncidents")
	}
	return func() tea.Msg {
		opts := pagerduty.ListIncidentsOptions{
			TeamIDs:  pdConfig.DefaultListOpts.TeamIDs,
			Limit:    pdConfig.DefaultListOpts.Limit,
			Offset:   pdConfig.DefaultListOpts.Offset,
			Statuses: pdConfig.DefaultListOpts.Statuses,
		}

		i, err := pd.GetIncidents(ctx, pdConfig, opts)
		if err != nil {
			return errMsg{err}
		}
		return gotIncidentsMsg(i)
	}
}

type getSingleIncidentMsg string
type gotSingleIncidentMsg *pagerduty.Incident

func getSingleIncident(ctx context.Context, pdConfig *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		i, err := pd.GetSingleIncident(ctx, pdConfig, id)
		if err != nil {
			return errMsg{err}
		}
		return gotSingleIncidentMsg(i)

	}
}
