package tui

import (
	"fmt"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/charmbracelet/log"
)

type model struct {
	err error

	config   *pd.Config
	editor   []string
	launcher launcher.ClusterLauncher

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
	token string,
	teams []string,
	user string,
	ignoredusers []string,
	editor []string,
	launcher launcher.ClusterLauncher,
) (tea.Model, tea.Cmd) {
	log.Debug("InitialModel")
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

func (m *model) setStatus(msg string) {
	var d []string

	m.status = fmt.Sprint(msg)

	d = append(d, "setStatus")
	d = append(d, msg)

	log.Infof("%s", d)
}

func (m *model) toggleHelp() {
	m.help.ShowAll = !m.help.ShowAll
}

func newTableWithStyles() table.Model {
	log.Debug("newTableWithStyles")
	t := table.New(table.WithFocused(true))
	t.SetStyles(tableStyle)
	return t
}

func newTextInput() textinput.Model {
	log.Debug("newTextInput")
	i := textinput.New()
	i.Prompt = " $ "
	i.CharLimit = 32
	i.Width = 50
	return i
}

func newHelp() help.Model {
	log.Debug("newHelp")
	h := help.New()
	h.ShowAll = true
	return h
}

func newIncidentViewer() viewport.Model {
	log.Debug("newIncidentViewer")
	vp := viewport.New(100, 100)
	vp.Style = incidentViewerStyle
	return vp
}
