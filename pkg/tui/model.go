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
	"github.com/charmbracelet/glamour"
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

// actionLogEntry stores a record of a write action performed on an incident
type actionLogEntry struct {
	key       string    // Keypress that triggered action (e.g., "a", "^e", "n", "%R" for resolved)
	id        string    // Incident ID
	summary   string    // Incident summary/title
	action    string    // Action performed (e.g., "acknowledge", "re-escalate", "resolved")
	timestamp time.Time // When the entry was added (used for aging out resolved incidents)
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

	table           table.Model
	actionLogTable  table.Model
	actionLog       []actionLogEntry
	input           textinput.Model
	// This is a hack since viewport.Model doesn't have a Focused() method
	viewingIncident bool
	incidentViewer  viewport.Model
	help            help.Model
	spinner         spinner.Model
	apiInProgress   bool
	markdownRenderer *glamour.TermRenderer

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

	autoAcknowledge  bool
	autoRefresh      bool
	teamMode         bool
	showActionLog    bool
	debug            bool
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

	// Create markdown renderer once - reusing it is much faster than creating new ones
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100), // Default width, will be adjusted on window resize
	)
	if err != nil {
		log.Error("InitialModel", "failed to create markdown renderer", err)
		// Continue without renderer - rendering will fall back to plain text
		renderer = nil
	}

	m := model{

		editor:           editor,
		launcher:         launcher,
		debug:            debug,
		help:             newHelp(),
		table:            newTableWithStyles(),
		actionLogTable:   newActionLogTable(),
		actionLog:        []actionLogEntry{},
		input:            newTextInput(),
		incidentViewer:   newIncidentViewer(),
		spinner:          s,
		markdownRenderer: renderer,
		apiInProgress:    false,
		status:           "",
		incidentCache:    make(map[string]*cachedIncidentData),
		scheduledJobs:    append([]*scheduledJob{}, initialScheduledJobs...),
		autoRefresh:      true, // Start watching for updates on startup
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

// addActionLogEntry adds an action to the action log, maintaining only the last 5 entries
func (m *model) addActionLogEntry(key, id, summary, action string) {
	entry := actionLogEntry{
		key:       key,
		id:        id,
		summary:   summary,
		action:    action,
		timestamp: time.Now(),
	}

	// Prepend new entry
	m.actionLog = append([]actionLogEntry{entry}, m.actionLog...)

	// Keep only last 5 entries
	if len(m.actionLog) > 5 {
		m.actionLog = m.actionLog[:5]
	}

	// Update action log table rows
	m.updateActionLogTable()
}

// updateActionLogTable refreshes the action log table with current entries
func (m *model) updateActionLogTable() {
	var rows []table.Row
	for _, entry := range m.actionLog {
		// 4 columns matching main table: keypress, ID, summary, action
		rows = append(rows, table.Row{entry.key, entry.id, entry.summary, entry.action})
	}
	m.actionLogTable.SetRows(rows)
}

// ageOutResolvedIncidents removes resolved incidents from the action log that are older than maxStaleAge
// Also ensures the action log never exceeds 5 entries total
func (m *model) ageOutResolvedIncidents(maxAge time.Duration) {
	var kept []actionLogEntry
	for _, entry := range m.actionLog {
		// Only age out resolved incidents (key == "%R")
		if entry.key == "%R" {
			age := time.Since(entry.timestamp)
			if age < maxAge {
				kept = append(kept, entry)
			} else {
				log.Debug("ageOutResolvedIncidents", "removing aged out resolved incident", "incident", entry.id, "age", age)
			}
		} else {
			// Keep all non-resolved entries (user actions)
			kept = append(kept, entry)
		}
	}

	// Ensure we don't exceed 5 entries total (newest entries are at the front)
	if len(kept) > 5 {
		log.Debug("ageOutResolvedIncidents", "trimming action log from", len(kept), "to 5 entries")
		kept = kept[:5]
	}

	m.actionLog = kept
	m.updateActionLogTable()
}

func newTableWithStyles() table.Model {
	t := table.New(table.WithFocused(true))
	t.SetStyles(tableStyle)
	return t
}

func newActionLogTable() table.Model {
	t := table.New(table.WithFocused(false))
	t.SetStyles(actionLogTableStyle)
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
