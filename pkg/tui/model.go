package tui

import (
	"fmt"
	"log"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/jira"
	"github.com/clcollins/srepd/pkg/pd"
)

type model struct {
	err error

	config     *pd.Config
	jiraConfig *jira.Config
	editor     []string
	launcher   ClusterLauncher

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

func InitialModel(
	// "d" is to avoid a conflict with the "debug" function
	d bool,
	pdToken string,
	jiraToken string,
	jiraHost string,
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
	pd, err := pd.NewConfig(pdToken, teams, user, ignoredusers)
	m.config = pd
	if err != nil {
		m.err = err
		return m, func() tea.Msg {
			return errMsg{err}
		}
	}

	jira, err := jira.NewConfig(jiraHost, jiraToken)
	m.jiraConfig = jira
	if err != nil {
		m.err = err
		return m, func() tea.Msg {
			return errMsg{err}
		}
	}

	return m, func() tea.Msg {
		return errMsg{err}
	}
}

func (m *model) setStatus(msg string) {
	var d []string

	m.status = fmt.Sprint(msg)

	d = append(d, "setStatus")
	d = append(d, msg)

	log.Printf("%s\n", d)
}

func (m *model) toggleHelp() {
	m.help.ShowAll = !m.help.ShowAll
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
	h.ShowAll = true
	return h
}

func newIncidentViewer() viewport.Model {
	debug("newIncidentViewer")
	vp := viewport.New(100, 100)
	vp.Style = incidentViewerStyle
	return vp
}
