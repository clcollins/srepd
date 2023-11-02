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

type Config struct {
	Debug   bool
	Verbose bool
}

var errorLog []error

// Type and function for capturing error messages with tea.Msg
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type model struct {
	help                  help.Model
	cliConfig             *Config
	pdConfig              *pd.Config
	context               context.Context
	currentUser           *pagerduty.User
	incidentList          []pagerduty.Incident
	selectedIncident      *pagerduty.Incident
	table                 table.Model
	toggleCurrentUserOnly bool
	statusMessage         string
	// Not Implemented
	// debugMessage          string
	// confirm               bool
}

func InitialModel(ctx context.Context, config *Config, pdConfig *pd.Config) model {
	return model{
		help:      help.New(),
		context:   ctx,
		cliConfig: config,
		pdConfig:  pdConfig,
	}
}

func (m model) Init() tea.Cmd {
	if m.cliConfig.Debug {
		logLevel = "debug"
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
	debug(fmt.Sprintf("Update: %s", msg))

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		// There's a couple of things that need to update on resize
		// Gotta figure out how to do that
		m.help.Width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch {

		// Pressing "q" or "ctrl-c" quits the program
		case key.Matches(msg, defaultKeyMap.Quit):
			return m, tea.Quit

		// Pressing "h" shows the full help message
		case key.Matches(msg, defaultKeyMap.Help):
			m.help.ShowAll = !m.help.ShowAll

		// Pressing "f" refreshes the incident list from PagerDuty
		case key.Matches(msg, defaultKeyMap.Refresh):
			m.statusMessage = refreshLogMessage
			return m, getIncidents(m.context, m.pdConfig)

		// Up and down arrows, and j/k keys move the cursor
		case key.Matches(msg, defaultKeyMap.Up):
			m.table.MoveUp(1)
			return m, nil
		case key.Matches(msg, defaultKeyMap.Down):
			m.table.MoveDown(1)
			return m, nil

		// Pressing "Enter" selects the incident to view
		// Get the incident id from the selected row, set the status message,
		// and call "getSingleIncident" to get the incident details
		case key.Matches(msg, defaultKeyMap.Enter):
			i := m.table.SelectedRow()[1]
			m.statusMessage = fmt.Sprintf("getting incident %s", i)
			return m, getSingleIncident(m.context, m.pdConfig, i)

		// Pressing "Esc" clears the selected incident
		case key.Matches(msg, defaultKeyMap.Esc):
			m.selectedIncident = nil
			return m, nil

		// Toggle team view vs. current user view
		// And call "gotIncidentsMsg" with the unfiltered list of incidents
		// to rebuild the table base on the current criteria
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

	// Receipt of an errMsg
	// Set the status message to the error message and append the error to the error log
	case errMsg:
		m.statusMessage = "ERROR: " + msg.Error()
		errorLog = append(errorLog, msg.err)
		return m, nil

	// Command to create a table with styles
	// Not sure if this is the best way to do this - maybe should just live in init
	case createTableWithStylesMsg:
		return m, createTableWithStyles()

	// Set the table model to the table created in the createTableWithStyles command
	case createdTableWithStylesMsg:
		m.table = msg.table
		return m, nil

	// Command to get the current user
	case getCurrentUserMsg:
		m.statusMessage = gettingUserMessage
		return m, getCurrentUser(m.context, m.pdConfig)

	// Set the current user to the user returned from the getCurrentUser command
	case gotCurrentUserMsg:
		m.currentUser = msg
		return m, nil

	// Just validate the silent user exists
	case getSilentUserMsg:
		m.statusMessage = gettingSilentUserMsg
		return m, getUser(m.context, m.pdConfig, m.pdConfig.SilentUser.ID)

	// Do nothing; this is just a placeholder for the future if necessary
	case gotSilentUserMsg:
		return m, nil

	// Command to retrieve a single incident
	// Nothing currently sends this message (see the "enter" case above, which sends the command)
	// This is just included for completeness
	case getSingleIncidentMsg:
		return m, getSingleIncident(m.context, m.pdConfig, m.selectedIncident.ID)

	// Set the selected incident to the incident returned from the getSingleIncident command
	case gotSingleIncidentMsg:
		m.statusMessage = fmt.Sprintf("got incident %s", msg.ID)
		m.selectedIncident = msg
		return m, nil

	// Command to retrieve the list of incidents from PagerDuty
	case getIncidentsMsg:
		return m, getIncidents(m.context, m.pdConfig)

	// Set the incident list to the list returned from the getIncidents command
	case gotIncidentsMsg:
		m.incidentList = msg
		var rows []table.Row
		m.table.SetRows(rows)
		for _, p := range m.incidentList {
			// If toggleCurrentUserOnly is true, rebuild the table from the incident list
			// including only the incidents assigned to the current user
			if m.toggleCurrentUserOnly {
				if AssignedToUser(p, m.currentUser.ID) {
					rows = append(rows, table.Row{"", p.ID, p.Title, p.Service.Summary})
				}
			} else {
				// Otherwise rebuild the table from the incident list, excluding the incidents
				// assigned to the silent user - we never want to see those
				if !AssignedToUser(p, m.pdConfig.SilentUser.ID) {
					rows = append(rows, table.Row{"", p.ID, p.Title, p.Service.Summary})
				}
			}
		}
		m.statusMessage = fmt.Sprintf("got %d incidents", len(rows))
		m.table.SetRows(rows)
		return m, nil

	// Do nothing
	default:
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.selectedIncident != nil {
		return m.renderIncidentView()
	}
	return m.renderIncidentTable()
}

func debug(s string) {
	if logLevel == "debug" {
		log.Print(s)
	}
}
