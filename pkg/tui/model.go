package tui

import (
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
)

var initialScheduledJobs = []*scheduledJob{
	{
		jobMsg:    func() tea.Msg { return PollIncidentsMsg{} },
		frequency: time.Second * 15,
	},
}

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

	// Loading state tracking - enables progressive rendering and action guards
	incidentDataLoaded   bool
	incidentNotesLoaded  bool
	incidentAlertsLoaded bool

	scheduledJobs []*scheduledJob

	autoAcknowledge bool
	autoRefresh     bool
	teamMode        bool
	debug           bool
}

func InitialModel(
	token string,
	teams []string,
	escalation_policies map[string]string,
	ignoredusers []string,
	editor []string,
	launcher launcher.ClusterLauncher,
	debug bool,
) (tea.Model, tea.Cmd) {
	var err error

	m := model{

		editor:         editor,
		launcher:       launcher,
		debug:          debug,
		help:           newHelp(),
		table:          newTableWithStyles(),
		input:          newTextInput(),
		incidentViewer: newIncidentViewer(),
		status:         "",
		scheduledJobs:  append([]*scheduledJob{}, initialScheduledJobs...),
	}

	// This is an ugly way to handle this error
	// We have to set the m.err here instead of how the errMsg is handled
	// because the Init() occurs before the Update() and the errMsg is not
	// preserved
	pd, err := pd.NewConfig(token, teams, escalation_policies, ignoredusers)
	m.config = pd

	if err != nil {
		log.Error("InitialModel", "error", err)
		m.err = err
	}

	log.Debug("InitialModel", "config", m.config)

	return m, func() tea.Msg {
		return errMsg{err}
	}
}

func (m *model) clearSelectedIncident(reason interface{}) {
	if m.selectedIncident != nil {
		log.Debug("clearSelectedIncident", "selectedIncident", m.selectedIncident.ID, "cleared", false)
		// Don't return here - we still want to clear out any notes/alerts and viewingIncident
		// even if the incident might be nil
	}
	m.selectedIncident = nil
	m.selectedIncidentNotes = nil
	m.selectedIncidentAlerts = nil
	m.viewingIncident = false
	// Clear loading flags
	m.incidentDataLoaded = false
	m.incidentNotesLoaded = false
	m.incidentAlertsLoaded = false
	log.Debug("clearSelectedIncident", "selectedIncident", m.selectedIncident, "cleared", true, "reason", reason)
}

func (m *model) setStatus(msg string) {
	log.Info("setStatus", "status", msg)
	m.status = msg
}

func (m *model) toggleHelp() {
	m.help.ShowAll = !m.help.ShowAll
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
	vp := viewport.New(100, 100)
	vp.Style = incidentViewerStyle
	return vp
}
