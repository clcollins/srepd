package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"charm.land/glamour/v2"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/clcollins/srepd/pkg/backplane"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/spf13/viper"
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

// confirmActionState stores the pending confirmation state for destructive actions.
// When set, the UI shows a prompt and only accepts y/n/Escape.
type confirmActionState struct {
	prompt string  // e.g., "Acknowledge P1234567? [y/n]"
	action tea.Cmd // Command to execute on 'y'
}

var initialScheduledJobs = []*scheduledJob{
	{
		jobMsg:    func() tea.Msg { return PollIncidentsMsg{} },
		frequency: time.Second * 15,
	},
	{
		jobMsg:    func() tea.Msg { return lazyEnrichMsg{} },
		frequency: 3 * time.Second,
	},
	{
		jobMsg:    checkForUpdate(false, ""),
		frequency: time.Hour,
	},
}

type model struct {
	err error

	config               *pd.Config
	editor               []string
	launcher             launcher.ClusterLauncher
	rosaBoundaryLauncher launcher.ClusterLauncher

	table          table.Model
	input          textinput.Model
	tagInputActive bool
	// This is a hack since viewport.Model doesn't have a Focused() method
	viewingIncident  bool
	incidentViewer   viewport.Model
	viewingLog       bool
	logViewer        viewport.Model
	logFilePath      string
	logDestination   string
	startupTime      time.Time
	help             help.Model
	spinner          spinner.Model
	apiInProgress    bool
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

	autoAcknowledge bool
	autoRefresh     bool
	teamMode        bool
	showLowUrgency  bool
	debug           bool

	// pendingConfirmation holds the state of a destructive action awaiting user confirmation.
	// When non-nil, the UI shows the prompt and only accepts y/n/Escape.
	pendingConfirmation *confirmActionState

	// clusterSelectMode is true when the user must choose which cluster to log into
	// because the incident has alerts referencing multiple distinct cluster_ids.
	clusterSelectMode         bool
	clusterSelectOptions      []string
	clusterSelectTable        table.Model
	clusterSelectPrompt       string
	rosaBoundaryClusterSelect bool

	// chordPending is true when the chord prefix key has been pressed and
	// the system is waiting for the second key to complete the chord.
	chordPending bool
	// chordPrefix is the configurable prefix key for chord commands (default "ctrl+x").
	chordPrefix string
	// chordHelpActive is true when chord help is being displayed in the help
	// section at the bottom of the screen, temporarily replacing the regular help.
	chordHelpActive bool

	// claudeQuerying is true while a Claude CLI query is in progress
	claudeQuerying      bool
	agentCLICommand     string
	cmdExecutor         CommandExecutor
	agentSystemPrompt   string
	watcherSystemPrompt string

	// reescalateLevel is the escalation level that ctrl+e re-escalates to. Default
	// is reEscalateDefaultPolicyLevel (2, "skips Nobody"); configurable via the
	// reescalate_level config key for policies with a different shape.
	reescalateLevel uint

	// aiProvider is the configured LLM API provider, or nil when unconfigured
	aiProvider ai.Provider
	// aiHealthy tracks whether the last LLM provider health check succeeded
	aiHealthy bool

	// watcherExpanded is true when the AI watcher pane is visible below the table
	watcherExpanded     bool
	watcherViewport     viewport.Model
	watcherBuffer       *watcherBuffer
	watcherMarker       string
	agentMarker         string
	watcherDedup        *watcherDedup
	watcherAnalyzing    bool
	watcherQueryStart   time.Time
	watcherQueryTimeout time.Duration
	typewriter          *typewriterState

	// Live streaming state. When streamResponses is true and the provider supports
	// streaming, watcher responses are appended token-by-token as they arrive
	// (watcherStreamPartial accumulates the in-progress response). watcherStreamCancel
	// aborts the in-flight stream when a new query supersedes it.
	streamResponses      bool
	watcherStreamPartial string
	watcherStreamCancel  context.CancelFunc

	// Incident viewer tab state
	activeTab int // 0=details, 1=alerts, 2=notes

	// Merge mode state
	mergeMode           bool
	mergeSourceIncident *pagerduty.Incident
	mergeTargetID       string
	mergeTable          table.Model
	mergeTeamMode       bool

	// Bulk silence state — triggered via chord ctrl+x s
	bulkSilenceMode bool
	bulkSilenceForm *huh.Form
	bulkSilenceIDs  []string

	// Team selection state — shown on first run or via --pick-teams
	teamSelectMode  bool
	teamSelectForm  *huh.Form
	teamSelectIDs   []string
	teamSelectNames map[string]string

	// Config wizard state — shown via "srepd config" or on first run
	configMode          bool
	configForm          *huh.Form
	configExisting      pkgconfig.ExistingConfig
	configIsNewFile     bool
	configState         *configFormState
	configTeamNames     map[string]string
	configPolicyNames   map[string]string
	configModeRequested bool
	configWizardPending *configWizardReadyMsg

	// OCM enrichment state
	ocmClient      ocm.OCMClient
	ocmAuthPending bool
	// ocmConnect overrides the post-wizard OCM connection for tests; nil
	// means the real ocm.Connect (which may run browser auth).
	ocmConnect            func() (ocm.OCMClient, error)
	incidentClusterMap    map[string][]string // incident ID → cluster IDs
	clusterEnrichInFlight map[string]bool     // cluster IDs currently being enriched
	clusterEnrichFailed   map[string]int      // failure count per cluster ID
	clusterCache          map[string]*ocm.ClusterInfo
	serviceLogCache       map[string][]ocm.ServiceLog
	limitedSupportCache   map[string][]ocm.LimitedSupportReason

	// Backplane enrichment state
	backplaneClient    backplane.BackplaneClient
	backplaneConfig    *backplane.Config
	clusterReportCache map[string][]backplane.Report

	// Prior alerts state
	priorAlertCache   map[string]*PriorAlertData
	priorAlertPending map[string]int

	// Flag conditions state
	flagConditions []FlagCondition
	flagNextID     int
	flagMarker     string
	flagMatchCache map[string][]int // incident ID → matching condition IDs

	// Dependency injection for testability
	pdClientFactory func(string) pd.PagerDutyClient
	configFS        pkgconfig.ConfigFS

	// Computed layout dimensions
	layout Layout

	// Color theme and derived styles
	theme  Theme
	styles Styles

	// Auto-update state
	devMode          bool
	updateAvailable  bool
	updateVersion    string
	updateReleaseURL string
}

// resolveReescalateLevel returns the configured re-escalate target level from the
// reescalate_level config key, falling back to reEscalateDefaultPolicyLevel (2) when
// unset or non-positive. Level 1 is the "Nobody" placeholder tier on the standard
// policy, so re-escalation defaults to level 2 (the first real on-call human).
func resolveReescalateLevel() uint {
	lvl := viper.GetInt("reescalate_level")
	if lvl <= 0 {
		return reEscalateDefaultPolicyLevel
	}
	return uint(lvl)
}

// resolveStreamResponses returns whether watcher/LLM responses should stream token
// by token. Defaults to true (stream when the provider supports it); set the
// stream_responses config key to false to force blocking responses.
func resolveStreamResponses() bool {
	if !viper.IsSet("stream_responses") {
		return true
	}
	return viper.GetBool("stream_responses")
}

func InitialModel(
	token string,
	teams []string,
	escalation_policies map[string]string,
	ignoredusers []string,
	editor []string,
	launcher launcher.ClusterLauncher,
	rosaBoundaryLauncher launcher.ClusterLauncher,
	debug bool,
	ocmClient ocm.OCMClient,
	colors map[string]string,
	defaultSilentPolicy string,
	customSilentPolicies map[string]string,
	configMode bool,
	ocmAuthPending bool,
	aiProvider ai.Provider,
	agentCLICommand string,
	backplaneClient backplane.BackplaneClient,
	backplaneConfig *backplane.Config,
) (tea.Model, tea.Cmd) {
	var err error

	theme := ThemeFromConfig(colors)
	styles := BuildStyles(theme)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(theme.Tab)

	// Create markdown renderer once - reusing it is much faster than creating new ones
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(styles.GlamourStyle),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		log.Error("tui.InitialModel()", "msg", "failed to create markdown renderer", "error", err)
		renderer = nil
	}

	t := newTableWithStyles()
	t.SetStyles(styles.Table)

	m := model{

		editor:                editor,
		launcher:              launcher,
		rosaBoundaryLauncher:  rosaBoundaryLauncher,
		debug:                 debug,
		help:                  newHelp(),
		table:                 t,
		input:                 newTextInput(),
		incidentViewer:        newIncidentViewer(),
		logViewer:             newLogViewer(),
		logFilePath:           defaultLogFilePath(),
		logDestination:        LogDestination,
		startupTime:           time.Now(),
		spinner:               s,
		markdownRenderer:      renderer,
		apiInProgress:         false,
		status:                "",
		incidentCache:         make(map[string]*cachedIncidentData),
		scheduledJobs:         append([]*scheduledJob{}, initialScheduledJobs...),
		autoRefresh:           true,
		showLowUrgency:        true,
		ocmClient:             ocmClient,
		ocmAuthPending:        ocmAuthPending,
		incidentClusterMap:    make(map[string][]string),
		clusterEnrichInFlight: make(map[string]bool),
		clusterEnrichFailed:   make(map[string]int),
		clusterCache:          make(map[string]*ocm.ClusterInfo),
		serviceLogCache:       make(map[string][]ocm.ServiceLog),
		limitedSupportCache:   make(map[string][]ocm.LimitedSupportReason),
		backplaneClient:       backplaneClient,
		backplaneConfig:       backplaneConfig,
		clusterReportCache:    make(map[string][]backplane.Report),
		priorAlertCache:       make(map[string]*PriorAlertData),
		priorAlertPending:     make(map[string]int),
		chordPrefix:           "ctrl+x",
		theme:                 theme,
		styles:                styles,
		configModeRequested:   configMode,
		aiProvider:            aiProvider,
		agentCLICommand:       agentCLICommand,
		cmdExecutor:           &execCommandExecutor{},
		watcherViewport:       newWatcherViewport(),
		watcherBuffer:         newWatcherBuffer(50),
	}

	mk := resolveMarkers(viper.GetBool("emoji"))
	m.flagMarker = mk.flag
	m.watcherMarker = mk.watcher
	m.agentMarker = mk.agent
	m.watcherDedup = newWatcherDedup(5 * time.Minute)
	m.agentSystemPrompt = viper.GetString("agent_system_prompt")
	m.watcherSystemPrompt = viper.GetString("watcher_system_prompt")
	m.reescalateLevel = resolveReescalateLevel()
	m.streamResponses = resolveStreamResponses()

	if aiProvider != nil {
		m.scheduledJobs = append(m.scheduledJobs, &scheduledJob{
			jobMsg:    aiHealthCheckCmd(aiProvider),
			frequency: 60 * time.Second,
		})
	}

	if configMode {
		return m, nil
	}

	// This is an ugly way to handle this error
	// We have to set the m.err here instead of how the errMsg is handled
	// because the Init() occurs before the Update() and the errMsg is not
	// preserved
	pd, err := pd.NewConfig(token, teams, escalation_policies, ignoredusers, defaultSilentPolicy, customSilentPolicies)
	m.config = pd

	if err != nil {
		log.Error("InitialModel", "error", err)
		m.err = err
	}

	return m, func() tea.Msg {
		return errMsg{err}
	}
}

// InitialModelWithConfig creates the initial TUI model using a pre-built pd.Config.
// This is used by dev mode to bypass live PagerDuty API initialization.
// The ocmClient parameter is optional — pass nil to disable OCM features.
func InitialModelWithConfig(
	config *pd.Config,
	editor []string,
	launcher launcher.ClusterLauncher,
	rosaBoundaryLauncher launcher.ClusterLauncher,
	debug bool,
	ocmClient ocm.OCMClient,
	aiProvider ai.Provider,
	agentCLICommand string,
	backplaneClient backplane.BackplaneClient,
) (tea.Model, tea.Cmd) {

	theme := DefaultTheme()
	styles := BuildStyles(theme)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(theme.Tab)

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(styles.GlamourStyle),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		log.Error("tui.InitialModelWithConfig()", "msg", "failed to create markdown renderer", "error", err)
		renderer = nil
	}

	t := newTableWithStyles()
	t.SetStyles(styles.Table)

	m := model{

		editor:                editor,
		launcher:              launcher,
		rosaBoundaryLauncher:  rosaBoundaryLauncher,
		debug:                 debug,
		help:                  newHelp(),
		table:                 t,
		input:                 newTextInput(),
		incidentViewer:        newIncidentViewer(),
		logViewer:             newLogViewer(),
		logFilePath:           defaultLogFilePath(),
		logDestination:        LogDestination,
		startupTime:           time.Now(),
		spinner:               s,
		markdownRenderer:      renderer,
		apiInProgress:         false,
		status:                "",
		incidentCache:         make(map[string]*cachedIncidentData),
		scheduledJobs:         append([]*scheduledJob{}, initialScheduledJobs...),
		autoRefresh:           true,
		showLowUrgency:        true,
		devMode:               true,
		ocmClient:             ocmClient,
		incidentClusterMap:    make(map[string][]string),
		clusterEnrichInFlight: make(map[string]bool),
		clusterEnrichFailed:   make(map[string]int),
		clusterCache:          make(map[string]*ocm.ClusterInfo),
		serviceLogCache:       make(map[string][]ocm.ServiceLog),
		limitedSupportCache:   make(map[string][]ocm.LimitedSupportReason),
		backplaneClient:       backplaneClient,
		clusterReportCache:    make(map[string][]backplane.Report),
		priorAlertCache:       make(map[string]*PriorAlertData),
		priorAlertPending:     make(map[string]int),
		theme:                 theme,
		styles:                styles,
		aiProvider:            aiProvider,
		agentCLICommand:       agentCLICommand,
		cmdExecutor:           &execCommandExecutor{},
		watcherViewport:       newWatcherViewport(),
		watcherBuffer:         newWatcherBuffer(50),
	}

	mk2 := resolveMarkers(viper.GetBool("emoji"))
	m.flagMarker = mk2.flag
	m.watcherMarker = mk2.watcher
	m.agentMarker = mk2.agent
	m.watcherDedup = newWatcherDedup(5 * time.Minute)
	m.agentSystemPrompt = viper.GetString("agent_system_prompt")
	m.watcherSystemPrompt = viper.GetString("watcher_system_prompt")
	m.reescalateLevel = resolveReescalateLevel()
	m.streamResponses = resolveStreamResponses()

	if aiProvider != nil {
		m.scheduledJobs = append(m.scheduledJobs, &scheduledJob{
			jobMsg:    aiHealthCheckCmd(aiProvider),
			frequency: 60 * time.Second,
		})
	}

	m.config = config

	if config == nil {
		m.err = fmt.Errorf("InitialModelWithConfig: config is nil")
	}

	return m, func() tea.Msg {
		return errMsg{m.err}
	}
}

func (m *model) readLog() tea.Cmd {
	if m.logDestination == "journal" {
		return readJournalLog(m.startupTime)
	}
	return readLogFile(m.logFilePath, m.startupTime)
}

func (m *model) clearSelectedIncident(reason interface{}) {
	if m.selectedIncident != nil {
		log.Debug("clearSelectedIncident", "selectedIncident", m.selectedIncident.ID, "cleared", false)
	}
	m.selectedIncident = nil
	m.selectedIncidentNotes = nil
	m.selectedIncidentAlerts = nil
	m.viewingIncident = false
	// Clear loading flags
	m.incidentDataLoaded = false
	m.incidentNotesLoaded = false
	m.incidentAlertsLoaded = false
	// Clear any pending confirmation or cluster selection on view transition
	m.pendingConfirmation = nil
	m.clusterSelectMode = false
	m.clusterSelectOptions = nil
	// Reset tab state
	m.activeTab = 0
	log.Debug("clearSelectedIncident", "selectedIncident", m.selectedIncident, "cleared", true, "reason", reason)
}

// syncSelectedIncidentToHighlightedRow updates m.selectedIncident to match the currently
func (m *model) clearOCMCacheForIncident(incidentID string) {
	clusterIDs, ok := m.incidentClusterMap[incidentID]
	if !ok {
		return
	}
	delete(m.incidentClusterMap, incidentID)
	for _, cid := range clusterIDs {
		stillReferenced := false
		for _, otherClusters := range m.incidentClusterMap {
			for _, other := range otherClusters {
				if other == cid {
					stillReferenced = true
					break
				}
			}
			if stillReferenced {
				break
			}
		}
		if !stillReferenced {
			delete(m.clusterCache, cid)
			delete(m.serviceLogCache, cid)
			delete(m.limitedSupportCache, cid)
			delete(m.clusterReportCache, cid)
			delete(m.priorAlertCache, cid)
			delete(m.priorAlertPending, cid)
			delete(m.clusterEnrichInFlight, cid)
		}
	}
	log.Debug("clearOCMCacheForIncident", "incident_id", incidentID)
}

// highlighted table row. Sets to nil if no row is highlighted. Uses cached data if available,
// otherwise uses stub data from m.incidentList.
//
// Returns a tea.Cmd to pre-fetch incident details if the highlighted incident is not fully
// cached (data + alerts + notes). Returns nil if no fetch is needed. The rate limiter
// handles debouncing for rapid navigation.
func (m *model) syncSelectedIncidentToHighlightedRow() tea.Cmd {
	row := m.table.SelectedRow()
	if row == nil {
		// Clear selection regardless of viewing state
		// If user scrolls table out of bounds, selection should be nil
		m.selectedIncident = nil
		m.incidentDataLoaded = false
		m.incidentNotesLoaded = false
		m.incidentAlertsLoaded = false
		log.Debug("syncSelectedIncidentToHighlightedRow", "no row highlighted", "cleared selection")
		return nil
	}

	incidentID := row[1] // Column [1] is the incident ID

	// If already viewing this incident, don't change anything
	if m.selectedIncident != nil && m.selectedIncident.ID == incidentID {
		return nil
	}

	// Track whether the cache is fully loaded to decide on pre-fetch
	fullyLoaded := false

	// Find incident in list
	found := false
	for i := range m.incidentList {
		if m.incidentList[i].ID == incidentID {
			found = true
			// Check if we have cached full data
			if cached, exists := m.incidentCache[incidentID]; exists && cached.incident != nil {
				// Check if cache is fresh relative to incident list data
				listIncident := &m.incidentList[i]
				if cached.incident.LastStatusChangeAt != listIncident.LastStatusChangeAt {
					// Cache is stale relative to list - use list data but keep cached notes/alerts
					log.Debug("syncSelectedIncidentToHighlightedRow", "cache stale, using updated list data", "incident", incidentID)
					incidentCopy := m.incidentList[i]
					m.selectedIncident = &incidentCopy
					m.incidentDataLoaded = false
					// Keep cached notes/alerts if available
					if cached.notesLoaded {
						m.selectedIncidentNotes = cached.notes
						m.incidentNotesLoaded = true
					} else {
						m.selectedIncidentNotes = nil
						m.incidentNotesLoaded = false
					}
					if cached.alertsLoaded {
						m.selectedIncidentAlerts = cached.alerts
						m.incidentAlertsLoaded = true
					} else {
						m.selectedIncidentAlerts = nil
						m.incidentAlertsLoaded = false
					}
					// Stale cache is not fully loaded
					fullyLoaded = false
				} else {
					// Cache is fresh - use it
					log.Debug("syncSelectedIncidentToHighlightedRow", "using cached data", "incident", incidentID)
					// Create copy to avoid pointer issues
					incidentCopy := *cached.incident
					m.selectedIncident = &incidentCopy
					m.incidentDataLoaded = cached.dataLoaded
					m.incidentNotesLoaded = cached.notesLoaded
					m.incidentAlertsLoaded = cached.alertsLoaded
					if cached.notes != nil {
						m.selectedIncidentNotes = cached.notes
					} else {
						m.selectedIncidentNotes = nil
					}
					if cached.alerts != nil {
						m.selectedIncidentAlerts = cached.alerts
					} else {
						m.selectedIncidentAlerts = nil
					}
					fullyLoaded = cached.dataLoaded && cached.notesLoaded && cached.alertsLoaded
				}
			} else {
				// Use stub data from incidentList
				log.Debug("syncSelectedIncidentToHighlightedRow", "using stub data", "incident", incidentID)
				// Create a copy to avoid pointer aliasing when slice is reallocated
				incidentCopy := m.incidentList[i]
				m.selectedIncident = &incidentCopy
				m.incidentDataLoaded = false
				m.incidentNotesLoaded = false
				m.incidentAlertsLoaded = false
				m.selectedIncidentNotes = nil
				m.selectedIncidentAlerts = nil
				fullyLoaded = false
			}
			break
		}
	}

	if !found {
		// Incident not found in list
		log.Debug("syncSelectedIncidentToHighlightedRow", "incident not found in list", incidentID)
		if !m.viewingIncident {
			m.selectedIncident = nil
			m.incidentDataLoaded = false
			m.incidentNotesLoaded = false
			m.incidentAlertsLoaded = false
		}
		return nil
	}

	// Pre-fetch: if the incident is not fully cached, trigger a background fetch.
	// The getIncidentMsg handler in Update will fetch details, alerts, and notes
	// concurrently and populate the cache. The rate limiter handles rapid scrolling.
	if !fullyLoaded {
		id := incidentID
		log.Debug("syncSelectedIncidentToHighlightedRow", "triggering pre-fetch", "incident", id)
		return func() tea.Msg { return getIncidentMsg(id) }
	}

	return nil
}

func (m *model) setStatus(msg string) {
	log.Debug("setStatus", "status", msg)
	m.status = msg
}

func (m *model) toggleHelp() {
	m.help.ShowAll = !m.help.ShowAll
	m.recomputeLayout()
}

// flashNotification sets a status message that auto-dismisses after 4 seconds.
// The clearFlashMsg handler only clears the status if it still matches, so newer
// messages are not prematurely dismissed.
func (m *model) flashNotification(msg string) tea.Cmd {
	m.setStatus(msg)
	flashMsg := msg
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return clearFlashMsg{message: flashMsg}
	})
}

// findRowIndex returns the index of the row in rows whose incident ID column
// (column 1) matches incidentID. Returns -1 if not found or rows is empty.
func findRowIndex(rows []table.Row, incidentID string) int {
	for i, row := range rows {
		if len(row) > 1 && row[1] == incidentID {
			return i
		}
	}
	return -1
}

func newTableWithStyles() table.Model {
	t := table.New(table.WithFocused(true))
	return t
}

func newTextInput() textinput.Model {
	i := textinput.New()
	i.Prompt = " $ "
	i.CharLimit = 500
	i.Width = 120
	return i
}

func newHelp() help.Model {
	h := help.New()
	h.ShowAll = false
	return h
}

func newIncidentViewer() viewport.Model {
	vp := viewport.New(100, 100)
	return vp
}

func newLogViewer() viewport.Model {
	vp := viewport.New(100, 100)
	return vp
}

func newWatcherViewport() viewport.Model {
	vp := viewport.New(100, layoutDefaultWatcherRows)
	return vp
}

// logFilePathForOS returns the platform-appropriate log file path for the
// given GOOS value. This is separated from defaultLogFilePath so it can be
// tested with arbitrary OS values (runtime.GOOS is a compile-time constant).
func logFilePathForOS(goos string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch goos {
	case "linux":
		return filepath.Join(home, ".config", "srepd", "debug.log")
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "srepd.log")
	default:
		return ""
	}
}

// defaultLogFilePath returns the default path for the srepd debug log file,
// selected based on the current operating system.
func defaultLogFilePath() string {
	return logFilePathForOS(runtime.GOOS)
}
