package tui

import (
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
)

// cachedIncidentData stores fetched incident data for reuse
type cachedIncidentData struct {
	incident     *pagerduty.Incident
	notes        []pagerduty.IncidentNote
	alerts       []pagerduty.IncidentAlert
	dataLoaded   bool
	notesLoaded  bool
	alertsLoaded bool
	lastFetched  time.Time
}

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
	spinner         spinner.Model
	apiInProgress   bool

	status string

	incidentList           []pagerduty.Incident
	selectedIncident       *pagerduty.Incident
	selectedIncidentNotes  []pagerduty.IncidentNote
	selectedIncidentAlerts []pagerduty.IncidentAlert

	// Loading state tracking - enables progressive rendering and action guards
	incidentDataLoaded   bool
	incidentNotesLoaded  bool
	incidentAlertsLoaded bool

	// Incident data cache - stores fetched data for reuse and pre-fetching
	incidentCache map[string]*cachedIncidentData

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

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := model{

		editor:         editor,
		launcher:       launcher,
		debug:          debug,
		help:           newHelp(),
		table:          newTableWithStyles(),
		input:          newTextInput(),
		incidentViewer: newIncidentViewer(),
		spinner:        s,
		apiInProgress:  false,
		status:         "",
		incidentCache:  make(map[string]*cachedIncidentData),
		scheduledJobs:  append([]*scheduledJob{}, initialScheduledJobs...),
		autoRefresh:    true, // Start watching for updates on startup
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

// getHighlightedIncident returns the incident object for the currently highlighted table row
// by looking it up in m.incidentList. Returns nil if no row is highlighted or incident not found.
func (m *model) getHighlightedIncident() *pagerduty.Incident {
	row := m.table.SelectedRow()
	if row == nil {
		return nil
	}

	incidentID := row[1] // Column [1] is the incident ID

	// Look up the incident in the incident list
	for i := range m.incidentList {
		if m.incidentList[i].ID == incidentID {
			return &m.incidentList[i]
		}
	}

	log.Debug("getHighlightedIncident", "incident not found in list", incidentID)
	return nil
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
