package tui

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/clcollins/srepd/pkg/alert"
	"github.com/clcollins/srepd/pkg/backplane"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/spf13/viper"
)

const (
	title              = "SREPD: It really whips the PDs' ACKs!"
	waitTime           = time.Millisecond * 1
	defaultInputPrompt = " $ "
	nilNoteErr         = "incident note content is empty"
	nilIncidentMsg     = "no incident selected"
)

const (
	reEscalateDefaultPolicyLevel = 2 // Skips Nobody
	silentDefaultPolicyKey       = "SILENT_DEFAULT"
	silentDefaultPolicyLevel     = 1 // Nobody
)

func (s setStatusMsg) Status() string {
	return s.string
}

func (m model) Init() tea.Cmd {
	if m.err != nil {
		return func() tea.Msg { return errMsg{m.err} }
	}

	initCmds := []tea.Cmd{
		tea.SetWindowTitle(title),
		m.spinner.Tick,
		func() tea.Msg { return updateIncidentListMsg("sender: Init") },
		checkForUpdate(m.devMode, ""),
	}

	if m.config != nil && m.config.Client != nil && pkgconfig.HasPlaceholderTeams(viper.GetStringSlice("teams")) {
		initCmds = append(initCmds, fetchUserTeams(m.config.Client))
	}

	if m.configModeRequested {
		initCmds = append(initCmds, prepareConfigWizardCmd(m))
	}

	return tea.Batch(initCmds...)
}

type filteredMsg struct {
	msg       tea.Msg
	truncated bool
}

type scheduledJob struct {
	jobMsg    tea.Cmd
	lastRun   time.Time
	frequency time.Duration
}

func wrapLines(text string, width int) string {
	if width <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	var wrapped []string
	for _, line := range lines {
		for len(line) > width {
			wrapped = append(wrapped, line[:width])
			line = line[width:]
		}
		wrapped = append(wrapped, line)
	}
	return strings.Join(wrapped, "\n")
}

func filterMsgContent(msg tea.Msg) tea.Msg {
	var truncatedMsg string
	switch msg := msg.(type) {
	default:
		return msg
	case renderedIncidentMsg:
		truncatedMsg = "template rendered"
	case updatedIncidentListMsg:
		var ids []string
		for _, i := range msg.incidents {
			ids = append(ids, i.ID)
		}
		truncatedMsg = fmt.Sprintf("%v", ids)
	case gotIncidentAlertsMsg:
		var ids []string
		for _, i := range msg.alerts {
			ids = append(ids, i.ID)
		}
		truncatedMsg = fmt.Sprintf("%v", ids)
	case gotIncidentNotesMsg:
		var ids []string
		for _, i := range msg.notes {
			ids = append(ids, i.ID)
		}
		truncatedMsg = fmt.Sprintf("%v", ids)
	}
	return filteredMsg{
		msg:       truncatedMsg,
		truncated: true,
	}
}

// The Update function is called for every message that is sent to the model,
// and it is responsible for updating the model based on the message and returning
// the new model and a command to execute.  These commands should be actual
// tea.Cmds, not functions that return tea.Msgs, though that signature is also a
// tea.Cmd, unless Update should handle the msg, or the msg is a tea.Batch or
// tea.Sequence.
//
// eg, good:
// return m, getIncident(m.config, msg.incident.ID)
//
// eg, ok:
// return m, func() tea.Msg { return errMsg{msg.err} }
//
// eg, bad:
// return m, func() tea.Msg { getIncident(m.config, msg.incident.ID) }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	msgType := reflect.TypeOf(msg)
	// Reduce logging for high-frequency messages to prevent I/O overhead
	shouldLog := true

	// Skip logging for very frequent messages
	switch msgType {
	case reflect.TypeOf(TickMsg{}),
		reflect.TypeOf(spinner.TickMsg{}),
		reflect.TypeOf(tea.MouseMsg{}),
		reflect.TypeOf(ocmServiceLogsMsg{}),
		reflect.TypeOf(limitedSupportMsg{}),
		reflect.TypeOf(clusterReportsMsg{}),
		reflect.TypeOf(priorAlertsMsg{}),
		reflect.TypeOf(lazyEnrichMsg{}):
		shouldLog = false
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok && shouldLog {
		// Skip logging for arrow keys and other navigation used in scrolling
		if keyMsg.Type == tea.KeyUp || keyMsg.Type == tea.KeyDown {
			shouldLog = false
		}
	}

	if shouldLog {
		log.Debug("Update", msgType, filterMsgContent(msg))
	}

	// PRIORITY HANDLING: Process user input keys immediately, before any queued messages
	// This ensures navigation and interaction keys are always responsive
	// even when the message queue is backed up with async responses
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Filter out terminal escape sequences that aren't real keypresses
		// These come from terminal queries (colors, cursor position, etc.)
		keyStr := keyMsg.String()

		// Drop terminal response sequences (OSC, CSI, etc.)
		if strings.Contains(keyStr, "rgb:") || // Color queries: ]11;rgb:1d1d/1d1d/2020
			strings.Contains(keyStr, ":1d1d/") || // Partial color responses
			strings.Contains(keyStr, "gb:1d1d/") || // Truncated color responses (missing 'r')
			strings.Contains(keyStr, "b:1c1c/") || // Another partial rgb: response
			strings.Contains(keyStr, "1c/1f1f") || // Bare color hex values
			strings.Contains(keyStr, "/1f1f") || // Fragment of hex color
			strings.Contains(keyStr, "1c1c/") || // Fragment of hex color
			strings.Contains(keyStr, "alt+]") || // OSC start sequence
			strings.Contains(keyStr, "alt+\\") || // OSC/DCS end sequence
			strings.Contains(keyStr, "CSI") || // Control Sequence Introducer
			keyStr == "OP" || // SS3 sequence (function keys)
			keyStr == "[A" || keyStr == "[B" || // Broken arrow key sequences
			keyStr == "[C" || keyStr == "[D" || // (should be handled by bubbletea)
			(strings.HasPrefix(keyStr, "[") && strings.HasSuffix(keyStr, "R")) || // CPR: [row;colR
			(strings.HasPrefix(keyStr, "]11;") || strings.HasPrefix(keyStr, "11;")) { // OSC 11 fragments
			// Drop these fake key messages - they're terminal responses, not user input
			// Don't log them - they're noise
			return m, nil
		}

		// All real user keypresses get priority handling
		// This ensures the UI is always responsive even when async messages are queued
		return m.keyMsgHandler(keyMsg)
	}

	// Filter out unknown CSI sequences (cursor position reports, etc.)
	// These are private bubble tea types, so we check the string representation
	msgStr := fmt.Sprintf("%T", msg)
	if strings.Contains(msgStr, "unknownCSISequenceMsg") {
		// Don't log these - they're noise from terminal queries
		return m, nil
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case errMsg:
		return m.errMsgHandler(msg)

	case fetchedTeamsMsg:
		if msg.err != nil {
			log.Warn("Failed to fetch teams for selection", "error", msg.err)
			m.setStatus("could not fetch teams: " + msg.err.Error())
			return m, nil
		}
		if len(msg.teams) == 0 {
			m.setStatus("no teams found in your PagerDuty account")
			return m, nil
		}
		var options []huh.Option[string]
		m.teamSelectNames = make(map[string]string)
		for _, team := range msg.teams {
			options = append(options, huh.NewOption(fmt.Sprintf("%s — %s", team.Name, team.ID), team.ID))
			m.teamSelectNames[team.ID] = team.Name
		}
		m.teamSelectIDs = nil

		theme := huh.ThemeCharm()
		theme.Focused.Title = theme.Focused.Title.Foreground(m.theme.Highlight)
		theme.Focused.Description = theme.Focused.Description.Foreground(m.theme.Muted)
		theme.Focused.SelectedOption = theme.Focused.SelectedOption.Foreground(m.theme.Highlight)
		theme.Focused.UnselectedOption = theme.Focused.UnselectedOption.Foreground(m.theme.Text)
		theme.Focused.MultiSelectSelector = theme.Focused.MultiSelectSelector.Foreground(m.theme.Text)
		theme.Focused.SelectedPrefix = theme.Focused.SelectedPrefix.Foreground(m.theme.Highlight)
		theme.Focused.UnselectedPrefix = theme.Focused.UnselectedPrefix.Foreground(m.theme.Muted)
		theme.Focused.Base = theme.Focused.Base.BorderForeground(m.theme.Border)

		m.teamSelectForm = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select your PagerDuty teams").
					Description("Use space to toggle, enter to confirm, esc to skip").
					Options(options...).
					Value(&m.teamSelectIDs),
			),
		).WithTheme(theme).WithHeight(m.layout.TeamSelectFormHeight)
		m.teamSelectMode = true
		return m, m.teamSelectForm.Init()

	case teamsSelectedMsg:
		if len(msg.ids) == 0 {
			m.setStatus("no teams selected")
			return m, nil
		}
		viper.Set("teams", msg.ids)
		m.setStatus(fmt.Sprintf("selected %d team(s) — updating config...", len(msg.ids)))
		return m, writeTeamsToConfigCmd(msg.ids, msg.names)

	case teamsConfigUpdatedMsg:
		if msg.err != nil {
			log.Warn("Failed to write team config", "error", msg.err)
			m.setStatus("teams selected but config write failed: " + msg.err.Error())
		} else {
			m.setStatus("teams saved to config")
		}
		return m, func() tea.Msg { return updateIncidentListMsg("teams updated") }

	case configWizardReadyMsg:
		if windowSize.Height == 0 {
			m.configWizardPending = &msg
			return m, nil
		}
		m.configExisting = msg.existing
		m.configIsNewFile = msg.isNewFile
		m.configTeamNames = msg.teamNames
		m.configPolicyNames = msg.policyNames
		m.configState = &configFormState{
			SilentPolicy: msg.existing.SilentPolicy,
			CustomInput:  pkgconfig.FormatCustomMappings(msg.existing.CustomPolicies),
			KeepTeams:    msg.kd.KeepTeams,
			KeepSilent:   msg.kd.KeepSilent,
			KeepCustom:   msg.kd.KeepCustom,
			Confirm:      true,
		}

		existingTeamSet := make(map[string]bool)
		for _, id := range msg.existing.Teams {
			existingTeamSet[id] = true
		}

		tokenHelp := "Create one at PagerDuty → My Profile → User Settings → API Access → Create New API Key."
		tokenDesc := "Your PagerDuty API OAuth token.\n" + tokenHelp
		if msg.existing.Token != "" {
			tokenDesc = fmt.Sprintf("Current: %s — leave blank to keep.\n%s", pkgconfig.MaskToken(msg.existing.Token), tokenHelp)
		}

		var teamDisplayList []string
		for _, id := range msg.existing.Teams {
			if name, ok := msg.teamNames[id]; ok {
				teamDisplayList = append(teamDisplayList, fmt.Sprintf("%s (%s)", name, id))
			} else {
				teamDisplayList = append(teamDisplayList, id)
			}
		}
		keepTeamsDesc := fmt.Sprintf("Current teams: %s", strings.Join(teamDisplayList, ", "))

		silentDisplay := msg.existing.SilentPolicy
		if name, ok := msg.policyNames[msg.existing.SilentPolicy]; ok {
			silentDisplay = fmt.Sprintf("%s (%s)", name, msg.existing.SilentPolicy)
		}
		keepSilentDesc := fmt.Sprintf("Current: %s", silentDisplay)

		var customDisplayParts []string
		for svcID, polID := range msg.existing.CustomPolicies {
			polDisplay := polID
			if name, ok := msg.policyNames[polID]; ok {
				polDisplay = fmt.Sprintf("%s (%s)", name, polID)
			}
			customDisplayParts = append(customDisplayParts, fmt.Sprintf("%s → %s", svcID, polDisplay))
		}
		keepCustomDesc := fmt.Sprintf("Current: %s", strings.Join(customDisplayParts, ", "))

		m.configForm = m.buildConfigForm(msg, tokenDesc, keepTeamsDesc, keepSilentDesc, keepCustomDesc, existingTeamSet)
		m.configMode = true
		return m, m.configForm.Init()

	case configCompletedMsg:
		fs := m.configFS
		if fs == nil {
			fs = realFS{}
		}
		return m, writeConfigCmd(msg.final, msg.changes, msg.teamNames, msg.customPolicies, msg.isNewFile, fs)

	case configSavedMsg:
		m.configMode = false
		m.configModeRequested = false
		if msg.err != nil {
			log.Warn("Failed to save config", "error", msg.err)
			m.setStatus("config save failed: " + msg.err.Error())
			m.table.Focus()
			return m, nil
		}
		m.setStatus("config saved — initializing...")
		m.table.Focus()
		return m, initPDClientCmd()

	case pdClientInitializedMsg:
		if msg.err != nil {
			log.Warn("Failed to initialize PD client", "error", msg.err)
			m.setStatus("config saved but PD init failed: " + msg.err.Error())
			return m, nil
		}
		m.config = msg.config
		m.setStatus("config saved")
		return m, tea.Batch(
			func() tea.Msg { return updateIncidentListMsg("config saved") },
			func() tea.Msg { return tea.WindowSizeMsg{Width: windowSize.Width, Height: windowSize.Height} },
		)

	case OCMClientReadyMsg:
		m.ocmAuthPending = false
		if msg.Err != nil {
			log.Warn("OCM authentication failed", "error", msg.Err)
			return m, m.flashNotification("OCM auth failed — cluster enrichment disabled")
		}
		if msg.Client == nil {
			log.Warn("OCM authentication cancelled")
			return m, m.flashNotification("OCM auth cancelled — cluster enrichment disabled")
		}
		m.ocmClient = msg.Client
		log.Info("OCM connected (async)")

		if m.backplaneClient == nil && m.backplaneConfig != nil {
			if m.backplaneConfig.URL == "" {
				resolvedURL, urlErr := msg.Client.GetBackplaneURL()
				if urlErr != nil {
					log.Warn("Backplane URL resolution from OCM failed (deferred)", "error", urlErr)
				} else {
					m.backplaneConfig.URL = resolvedURL
					log.Info("Backplane URL resolved from OCM (deferred)", "url", resolvedURL)
				}
			}
			if m.backplaneConfig.URL != "" {
				m.backplaneClient = backplane.NewClient(m.backplaneConfig, msg.Client.GetAccessToken)
				log.Info("Backplane client initialized (deferred)")
			} else {
				log.Warn("Backplane client not created (deferred): no URL available")
			}
		}

		var enrichCmds []tea.Cmd
		for _, clusterIDs := range m.incidentClusterMap {
			var uncached []string
			for _, id := range clusterIDs {
				if _, cached := m.clusterCache[id]; cached {
					continue
				}
				if m.clusterEnrichInFlight[id] || m.clusterEnrichFailed[id] >= 3 {
					continue
				}
				uncached = append(uncached, id)
				m.clusterEnrichInFlight[id] = true
			}
			enrichCmds = append(enrichCmds, enrichClusters(m.ocmClient, uncached, m.devMode)...)
		}

		enrichCmds = append(enrichCmds, m.flashNotification("OCM connected — enriching cluster data"))
		return m, tea.Batch(enrichCmds...)

	case addFlagConditionMsg:
		m.flagNextID++
		msg.condition.ID = m.flagNextID
		m.flagConditions = append(m.flagConditions, msg.condition)
		m.rebuildFlagMatchCache()
		watcherCmds := m.runDetectors()
		flashCmd := m.flashNotification(fmt.Sprintf("flag #%d added: %s", msg.condition.ID, msg.condition.Label))
		rebuildCmd := func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} }
		return m, tea.Batch(append(watcherCmds, flashCmd, rebuildCmd)...)

	case removeFlagConditionMsg:
		for i, c := range m.flagConditions {
			if c.ID == msg.id {
				m.flagConditions = append(m.flagConditions[:i], m.flagConditions[i+1:]...)
				break
			}
		}
		m.rebuildFlagMatchCache()
		watcherCmds := m.runDetectors()
		flashCmd := m.flashNotification(fmt.Sprintf("flag #%d removed", msg.id))
		rebuildCmd := func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} }
		return m, tea.Batch(append(watcherCmds, flashCmd, rebuildCmd)...)

	case clearFlagConditionsMsg:
		m.flagConditions = nil
		m.rebuildFlagMatchCache()
		return m, tea.Batch(
			m.flashNotification("all flags cleared"),
			func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} },
		)

	case listFlagConditionsMsg:
		content := formatFlagsList(m.flagConditions)
		rendered, renderErr := renderIncidentMarkdown(&m, content)
		if renderErr != nil {
			rendered = content
		}
		m.incidentViewer.SetContent(rendered)
		m.incidentViewer.GotoTop()
		m.viewingIncident = true
		m.table.Blur()
		return m, nil

	case flagsSavedMsg:
		if msg.err != nil {
			return m, m.flashNotification("flags save failed: " + msg.err.Error())
		}
		return m, m.flashNotification(fmt.Sprintf("flags saved (%d conditions)", len(m.flagConditions)))

	case flagsLoadedMsg:
		if msg.err != nil {
			return m, m.flashNotification("flags load failed: " + msg.err.Error())
		}
		m.flagConditions = msg.conditions
		maxID := 0
		for _, c := range m.flagConditions {
			if c.ID > maxID {
				maxID = c.ID
			}
		}
		m.flagNextID = maxID
		m.rebuildFlagMatchCache()

		enrichCmds := []tea.Cmd{
			m.flashNotification(fmt.Sprintf("flags loaded (%d conditions)", len(m.flagConditions))),
			func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} },
		}
		for _, inc := range m.incidentList {
			if _, ok := m.incidentClusterMap[inc.ID]; !ok {
				if m.config != nil {
					enrichCmds = append(enrichCmds, getIncidentAlerts(m.config, inc.ID))
				}
			}
		}
		return m, tea.Batch(enrichCmds...)

	case updatedIncidentTitleMsg:
		if msg.err != nil {
			m.setStatus("tag update failed: " + msg.err.Error())
			return m, nil
		}
		for i, inc := range m.incidentList {
			if inc.ID == msg.incidentID {
				m.incidentList[i].Title = msg.newTitle
				if cached, ok := m.incidentCache[msg.incidentID]; ok && cached.incident != nil {
					cached.incident.Title = msg.newTitle
				}
				if m.selectedIncident != nil && m.selectedIncident.ID == msg.incidentID {
					m.selectedIncident.Title = msg.newTitle
				}
				break
			}
		}
		return m, tea.Batch(
			m.flashNotification("tags updated"),
			func() tea.Msg { return updatedIncidentListMsg{m.incidentList, nil} },
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case TickMsg:
		return m, tea.Batch(runScheduledJobs(&m)...)

	case typewriterTickMsg:
		return m, m.advanceTypewriter()

	case aiHealthCheckMsg:
		m.aiHealthy = msg.healthy
		return m, nil

	case watcherPromptMsg:
		if m.aiProvider == nil {
			return m, m.flashNotification("LLM provider not configured — add llm_api section to config")
		}
		if !m.aiHealthy {
			return m, m.flashNotification("LLM provider offline")
		}

		log.Info("watcher query initiated", "prompt", truncatePrompt(msg.prompt, 80))
		m.setStatus(fmt.Sprintf("querying watcher: %s", truncatePrompt(msg.prompt, 40)))
		m.watcherAnalyzing = true
		m.apiInProgress = true
		m.watcherQueryStart = time.Now()
		m.watcherQueryTimeout = watcherQueryTimeout

		if !m.watcherExpanded {
			m.watcherExpanded = true
			m.recomputeLayout()
		}

		incidentContext := buildWatcherContext(&m)

		// Stream token-by-token when the provider supports it and streaming is
		// enabled; otherwise fall back to the blocking query + typewriter.
		if m.streamResponses && ai.SupportsStreaming(m.aiProvider) {
			m.watcherStreamPartial = ""
			return m, tea.Batch(
				m.spinner.Tick,
				streamWatcherCmd(m.aiProvider, m.watcherSystemPrompt, msg.prompt, incidentContext),
			)
		}

		return m, tea.Batch(
			m.spinner.Tick,
			watcherQueryCmd(m.aiProvider, m.watcherSystemPrompt, msg.prompt, incidentContext),
		)

	case watcherStreamStartedMsg:
		// Abort any prior in-flight stream, then begin a fresh buffer entry that
		// subsequent chunks grow in place.
		if m.watcherStreamCancel != nil {
			m.watcherStreamCancel()
		}
		m.watcherStreamCancel = msg.cancel
		m.watcherStreamPartial = ""
		if !m.watcherExpanded {
			m.watcherExpanded = true
			m.recomputeLayout()
		}
		m.watcherBuffer.Append(prefixLines(m.watcherMarker, ""))
		m.updateWatcherViewport()
		return m, readStreamCmd(msg.ch)

	case watcherStreamChunkMsg:
		m.watcherStreamPartial += msg.text
		m.watcherBuffer.SetLast(prefixLines(m.watcherMarker, m.watcherStreamPartial))
		m.updateWatcherViewport()
		return m, readStreamCmd(msg.ch)

	case watcherStreamDoneMsg:
		m.watcherAnalyzing = false
		m.apiInProgress = false
		m.watcherStreamCancel = nil
		if msg.err != nil {
			// Keep whatever partial text streamed; surface the error via flash.
			return m, m.flashNotification(fmt.Sprintf("watcher stream error: %s", msg.err))
		}
		m.setStatus("watcher response received")
		return m, nil

	case watcherResponseMsg:
		m.watcherAnalyzing = false
		m.apiInProgress = false

		if msg.err != nil {
			return m, m.flashNotification(fmt.Sprintf("watcher query failed: %s", msg.err))
		}

		if !m.watcherExpanded {
			m.watcherExpanded = true
			m.recomputeLayout()
		}
		m.setStatus("watcher response received")
		m.watcherBuffer.Append("")
		return m, m.startTypewriter(m.watcherMarker, msg.response)

	case watcherSynthesisMsg:
		m.watcherAnalyzing = false
		if !m.watcherExpanded {
			m.watcherExpanded = true
			m.recomputeLayout()
		}
		if msg.err != nil {
			m.watcherBuffer.Append(prefixLines(m.watcherMarker, msg.observation))
			m.updateWatcherViewport()
			return m, nil
		}
		m.watcherBuffer.Append("")
		return m, m.startTypewriter(m.watcherMarker, msg.response)

	case tea.WindowSizeMsg:
		return m.windowSizeMsgHandler(msg)

	case tea.MouseMsg:
		if m.watcherExpanded {
			var cmd tea.Cmd
			m.watcherViewport, cmd = m.watcherViewport.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.keyMsgHandler(msg)

	case setStatusMsg:
		return m.setStatusMsgHandler(msg)

	case clearFlashMsg:
		// Only clear the status if it still matches the flash message.
		// This prevents a newer message from being prematurely dismissed.
		if m.status == msg.message {
			m.setStatus("")
		}
		return m, nil

	// Command to trigger a regular poll for new incidents
	case PollIncidentsMsg:
		if !m.autoRefresh {
			return m, nil
		}
		m.apiInProgress = true
		return m, tea.Batch(m.spinner.Tick, updateIncidentList(m.config))

	case lazyEnrichMsg:
		cmd := pickNextEnrichment(&m)
		if cmd != nil {
			return m, cmd
		}
		return m, nil

	// Command to get an incident by ID
	case getIncidentMsg:
		if msg == "" {
			return m, func() tea.Msg {
				return setStatusMsg{"no incident selected"}
			}
		}

		m.setStatus(fmt.Sprintf("getting details for incident %v...", msg))
		id := string(msg)
		m.apiInProgress = true
		cmds = append(cmds,
			m.spinner.Tick,
			getIncident(m.config, id),
			getIncidentAlerts(m.config, id),
			getIncidentNotes(m.config, id),
		)

	// Set the selected incident to the incident returned from the getIncident command
	case gotIncidentMsg:
		if msg.err != nil {
			m.selectedIncident = nil
			m.viewingIncident = false
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// Update cache with fetched incident data
		if cached, exists := m.incidentCache[msg.incident.ID]; exists {
			log.Debug("Update", "gotIncidentMsg", "refreshing cached incident data", "incident", msg.incident.ID)
			cached.incident = msg.incident
			cached.dataLoaded = true
			cached.lastFetched = time.Now()
		} else {
			log.Debug("Update", "gotIncidentMsg", "caching new incident data", "incident", msg.incident.ID)
			m.incidentCache[msg.incident.ID] = &cachedIncidentData{
				incident:    msg.incident,
				dataLoaded:  true,
				lastFetched: time.Now(),
			}
		}

		// Only update selected incident if no incident is selected or this matches the selected one
		// Skip if we're viewing a different incident (don't let background pre-fetch overwrite it)
		// Check if this message is still relevant to the current selection
		// Prevents late-arriving messages from overwriting when user navigated away
		shouldUpdate := false
		if m.selectedIncident == nil {
			shouldUpdate = true
		} else if m.selectedIncident.ID == msg.incident.ID {
			shouldUpdate = true
		}

		if shouldUpdate {
			log.Info("incident details fetched", "incident_id", msg.incident.ID, "title", msg.incident.Title)
			m.setStatus(fmt.Sprintf("got incident %s", msg.incident.ID))
			m.selectedIncident = msg.incident
			m.incidentDataLoaded = true

			// Stop spinner if all incident data is loaded (details, notes, alerts)
			if m.incidentDataLoaded && m.incidentNotesLoaded && m.incidentAlertsLoaded {
				m.apiInProgress = false
			}

			// Re-render if we're viewing the incident to show updated details progressively
			if m.viewingIncident {
				return m, func() tea.Msg { return renderIncidentMsg("incident details arrived") }
			}
		}

	case gotIncidentNotesMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// Update cache with fetched notes
		if cached, exists := m.incidentCache[msg.incidentID]; exists {
			log.Debug("Update", "gotIncidentNotesMsg", "refreshing cached notes", "incident", msg.incidentID, "count", len(msg.notes))
			cached.notes = msg.notes
			cached.notesLoaded = true
		} else {
			log.Debug("Update", "gotIncidentNotesMsg", "caching new notes", "incident", msg.incidentID, "count", len(msg.notes))
			m.incidentCache[msg.incidentID] = &cachedIncidentData{
				notes:       msg.notes,
				notesLoaded: true,
			}
		}

		// Only update selected incident notes if no incident is selected or this matches the selected one
		// Skip if we're viewing a different incident (don't let background pre-fetch overwrite it)
		if m.selectedIncident == nil || msg.incidentID == m.selectedIncident.ID {
			switch {
			case len(msg.notes) == 1:
				m.setStatus(fmt.Sprintf("got %d note for incident", len(msg.notes)))
			case len(msg.notes) > 1:
				m.setStatus(fmt.Sprintf("got %d notes for incident", len(msg.notes)))
			}

			m.selectedIncidentNotes = msg.notes
			m.incidentNotesLoaded = true
			log.Info("notes fetched", "incident_id", msg.incidentID, "count", len(msg.notes))

			// Stop spinner if all incident data is loaded (details, notes, alerts)
			if m.incidentDataLoaded && m.incidentNotesLoaded && m.incidentAlertsLoaded {
				m.apiInProgress = false
			}

			// Re-render if we're viewing the incident to show the notes progressively
			if m.viewingIncident && m.selectedIncident != nil && msg.incidentID == m.selectedIncident.ID {
				return m, func() tea.Msg { return renderIncidentMsg("notes arrived") }
			}
		}

	case gotIncidentAlertsMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		// Update cache with fetched alerts
		if cached, exists := m.incidentCache[msg.incidentID]; exists {
			log.Debug("Update", "gotIncidentAlertsMsg", "refreshing cached alerts", "incident", msg.incidentID, "count", len(msg.alerts))
			cached.alerts = msg.alerts
			cached.alertsLoaded = true
		} else {
			log.Debug("Update", "gotIncidentAlertsMsg", "caching new alerts", "incident", msg.incidentID, "count", len(msg.alerts))
			m.incidentCache[msg.incidentID] = &cachedIncidentData{
				alerts:       msg.alerts,
				alertsLoaded: true,
			}
		}

		// Only update selected incident alerts if no incident is selected or this matches the selected one
		// Skip if we're viewing a different incident (don't let background pre-fetch overwrite it)
		if m.selectedIncident == nil || msg.incidentID == m.selectedIncident.ID {
			switch {
			case len(msg.alerts) == 1:
				m.setStatus(fmt.Sprintf("got %d alert for incident", len(msg.alerts)))
			case len(msg.alerts) > 1:
				m.setStatus(fmt.Sprintf("got %d alerts for incident", len(msg.alerts)))
			}

			m.selectedIncidentAlerts = msg.alerts
			m.incidentAlertsLoaded = true
			log.Info("alerts fetched", "incident_id", msg.incidentID, "count", len(msg.alerts))

			// Stop spinner if all incident data is loaded (details, notes, alerts)
			if m.incidentDataLoaded && m.incidentNotesLoaded && m.incidentAlertsLoaded {
				m.apiInProgress = false
			}

			// Map incident → cluster IDs and trigger OCM enrichment for uncached clusters
			clusterIDs := getUniqueClusters(msg.alerts)
			log.Debug("gotIncidentAlertsMsg", "incident_id", msg.incidentID, "alert_count", len(msg.alerts), "cluster_ids", clusterIDs)
			if len(clusterIDs) > 0 {
				if m.incidentClusterMap == nil {
					m.incidentClusterMap = make(map[string][]string)
				}
				if m.clusterEnrichInFlight == nil {
					m.clusterEnrichInFlight = make(map[string]bool)
				}
				m.incidentClusterMap[msg.incidentID] = clusterIDs
			}
			var uncachedClusters []string
			for _, id := range clusterIDs {
				if _, cached := m.clusterCache[id]; cached {
					continue
				}
				if m.clusterEnrichInFlight[id] {
					continue
				}
				if m.clusterEnrichFailed[id] >= 3 {
					continue
				}
				uncachedClusters = append(uncachedClusters, id)
				m.clusterEnrichInFlight[id] = true
			}
			enrichCmds := enrichClusters(m.ocmClient, uncachedClusters, m.devMode)
			if len(enrichCmds) > 0 {
				cmds = append(cmds, enrichCmds...)
			}

			if m.config != nil && len(clusterIDs) > 0 {
				var teamIDs []string
				for _, team := range m.config.Teams {
					teamIDs = append(teamIDs, team.ID)
				}

				currentAlertName := ""
				svcSeen := make(map[string]bool)
				var serviceIDs []string
				for _, a := range msg.alerts {
					if a.Service.ID != "" && !svcSeen[a.Service.ID] {
						svcSeen[a.Service.ID] = true
						serviceIDs = append(serviceIDs, a.Service.ID)
					}
					if currentAlertName == "" {
						normalized := alert.NormalizeAlert(a.Service.Summary, "", a)
						if normalized.AlertName != "" {
							currentAlertName = normalized.AlertName
						} else {
							name := getDetailFieldFromAlert("alert_name", a)
							if name != "" {
								currentAlertName = name
							}
						}
					}
				}

				for _, cid := range clusterIDs {
					if _, cached := m.priorAlertCache[cid]; cached {
						continue
					}
					if m.priorAlertPending[cid] > 0 {
						continue
					}
					weeks := buildPriorAlertWeeks()
					if len(weeks) == 0 {
						continue
					}
					m.priorAlertPending[cid] = len(weeks)
					cmds = append(cmds, fetchPriorAlertsWeek(
						m.config.Client,
						serviceIDs,
						teamIDs,
						cid,
						currentAlertName,
						msg.incidentID,
						weeks[0],
						weeks[1:],
					))
				}
			}

			// Re-render if we're viewing the incident to show the alerts progressively
			if m.viewingIncident && m.selectedIncident != nil && msg.incidentID == m.selectedIncident.ID {
				return m, tea.Batch(append(cmds, func() tea.Msg { return renderIncidentMsg("alerts arrived") })...)
			}
		}

	case updateIncidentListMsg:
		m.setStatus(loadingIncidentsStatus)
		m.apiInProgress = true
		cmds = append(cmds, m.spinner.Tick, updateIncidentList(m.config))

	case updatedIncidentListMsg:
		if msg.err != nil {
			m.apiInProgress = false
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		m.apiInProgress = false

		var acknowledgeIncidentsList []pagerduty.Incident

		// Flash notification for resolved incidents
		var resolvedIDs []string
		for _, i := range m.incidentList {
			idx := slices.IndexFunc(msg.incidents, func(incident pagerduty.Incident) bool {
				return incident.ID == i.ID
			})
			if idx == -1 {
				resolvedIDs = append(resolvedIDs, i.ID)
			}
		}
		if len(resolvedIDs) > 0 {
			for _, id := range resolvedIDs {
				log.Info("incident resolved", "incident_id", id)
			}
			resolvedMsg := fmt.Sprintf("Resolved: %s", strings.Join(resolvedIDs, ", "))
			cmds = append(cmds, m.flashNotification(resolvedMsg))
		}

		// Detect new incidents
		oldIDs := make(map[string]bool, len(m.incidentList))
		for _, i := range m.incidentList {
			oldIDs[i.ID] = true
		}
		var newIDs []string
		for _, i := range msg.incidents {
			if !oldIDs[i.ID] {
				newIDs = append(newIDs, i.ID)
			}
		}
		if len(newIDs) > 0 {
			log.Info("new incidents", "count", len(newIDs), "ids", newIDs)
		}

		oldCount := len(m.incidentList)
		newCount := len(msg.incidents)
		if oldCount != newCount && oldCount > 0 {
			log.Info("incident count changed", "old", oldCount, "new", newCount)
		}

		// Overwrite m.incidentList with current incidents
		m.incidentList = msg.incidents

		// Note: We no longer pre-fetch all incident details, alerts, and notes here.
		// This was inefficient because:
		// 1. Most incidents are never viewed or acted upon
		// 2. The incident list already contains sufficient data for most actions
		// 3. getHighlightedIncident() uses data from m.incidentList directly
		// 4. Details/alerts/notes are now fetched on-demand when actually needed:
		//    - When user presses Enter to view an incident
		//    - When user presses 'l' to login (needs alerts)
		// This reduces unnecessary API calls from O(n) to O(1) per incident list update.

		// Check if any incidents should be auto-acknowledged.
		// The on-call check is done OFF the Update loop (checkOnCallAndAcknowledge)
		// so a slow PagerDuty request never freezes the UI. On-call status is checked
		// live every refresh — never cached — so leaving SREPD running past a shift
		// stops auto-ack within one cycle.
		//
		// First compute the on-call-independent candidates (assigned && !acked). Only
		// if there are candidates do we dispatch the on-call check, avoiding an API
		// call every refresh when nothing is assigned to the user.
		if m.autoAcknowledge {
			for _, i := range m.incidentList {
				if AssignedToUser(i, m.config.CurrentUser.ID) && !AcknowledgedByUser(i, m.config.CurrentUser.ID) {
					acknowledgeIncidentsList = append(acknowledgeIncidentsList, i)
				}
			}

			if len(acknowledgeIncidentsList) > 0 {
				cmds = append(cmds, checkOnCallAndAcknowledge(m.config, m.config.CurrentUser.ID, acknowledgeIncidentsList))
			}
		}

		// Clean up cache - remove entries for incidents no longer in the list
		incidentIDs := make(map[string]bool)
		for _, i := range m.incidentList {
			incidentIDs[i.ID] = true
		}
		for id := range m.incidentCache {
			if incidentIDs[id] {
				// Incident still exists - mark cache as potentially stale
				// Find incident in new list and check if LastStatusChangeAt differs
				for _, newIncident := range m.incidentList {
					if newIncident.ID == id {
						if cached, exists := m.incidentCache[id]; exists {
							// Compare timestamps to detect changes
							if cached.incident != nil &&
								cached.incident.LastStatusChangeAt != newIncident.LastStatusChangeAt {
								// Incident changed - invalidate cached details
								log.Debug("Update", "updatedIncidentListMsg", "invalidating cache for updated incident", "id", id)
								delete(m.incidentCache, id)
							}
						}
						break
					}
				}
			} else {
				// Incident no longer in list - remove from cache and OCM data
				delete(m.incidentCache, id)
				m.clearOCMCacheForIncident(id)
				log.Debug("Update", "updatedIncidentListMsg", "removing cached data for incident no longer in list", "incident", id)
			}
		}

		// Capture the currently highlighted incident ID before rebuilding rows
		var highlightedID string
		if currentRow := m.table.SelectedRow(); len(currentRow) > 1 {
			highlightedID = currentRow[1]
		}

		totalIncidentCount := len(m.incidentList)

		// Apply urgency filter before building table rows
		filteredIncidents := filterByUrgency(m.incidentList, m.showLowUrgency)

		m.rebuildFlagMatchCache()

		var rows []table.Row

		for _, i := range filteredIncidents {
			state := stateShorthand(i, m.config.CurrentUser.ID)
			if AssignedToUser(i, m.config.CurrentUser.ID) || m.teamMode {
				serviceName := i.Service.Summary
				// Populate incidentClusterMap from cache if not already set
				if _, mapped := m.incidentClusterMap[i.ID]; !mapped {
					if cached, exists := m.incidentCache[i.ID]; exists && cached.alertsLoaded {
						ids := getUniqueClusters(cached.alerts)
						if len(ids) > 0 {
							if m.incidentClusterMap == nil {
								m.incidentClusterMap = make(map[string][]string)
							}
							m.incidentClusterMap[i.ID] = ids
						}
					}
				}
				if clusterIDs, ok := m.incidentClusterMap[i.ID]; ok {
					for _, clusterID := range clusterIDs {
						if info, exists := m.clusterCache[clusterID]; exists {
							displayName := info.DisplayName
							if displayName == "" {
								displayName = info.Name
							}
							if displayName != "" {
								serviceName = displayName
								break
							}
						}
					}
					if len(clusterIDs) > 1 {
						suffix := fmt.Sprintf(" (+%d)", len(clusterIDs)-1)
						cols := m.table.Columns()
						if len(cols) >= 4 {
							maxWidth := cols[3].Width - len(suffix) - 3
							if maxWidth > 0 && len(serviceName) > maxWidth {
								serviceName = serviceName[:maxWidth] + "..."
							}
						}
						serviceName = serviceName + suffix
					}
				}
				title := i.Title
				if matchedFlags, ok := m.flagMatchCache[i.ID]; ok && len(matchedFlags) > 0 {
					title = m.flagMarker + title
				}
				rows = append(rows, table.Row{state, i.ID, title, serviceName})
			}
		}

		m.table.SetRows(rows)

		// Restore cursor to the previously highlighted incident
		if highlightedID != "" {
			if idx := findRowIndex(rows, highlightedID); idx >= 0 {
				m.table.SetCursor(idx)
			}
		}

		// Build status message with filter and count info
		var filterSuffix string
		if !m.showLowUrgency {
			filterSuffix = " (high only)"
		}

		if totalIncidentCount == 1 {
			m.setStatus(fmt.Sprintf("showing %d/%d incident%s...", len(m.table.Rows()), totalIncidentCount, filterSuffix))
		} else {
			m.setStatus(fmt.Sprintf("showing %d/%d incidents%s...", len(m.table.Rows()), totalIncidentCount, filterSuffix))
		}

		// In dev mode, pre-fetch alerts for all incidents so OCM enrichment
		// covers the full list, not just the highlighted incident.
		if m.devMode && m.ocmClient != nil {
			for _, i := range m.incidentList {
				if _, cached := m.incidentCache[i.ID]; !cached {
					id := i.ID
					cmds = append(cmds, getIncidentAlerts(m.config, id))
				}
			}
		}

		// Kick off enrichment for the first un-enriched incident immediately
		if enrichCmd := pickNextEnrichment(&m); enrichCmd != nil {
			cmds = append(cmds, enrichCmd)
		}

		// Re-sync selectedIncident to match highlighted row
		// This handles cases where the incident list changed but cursor position stayed same
		if cmd := m.syncSelectedIncidentToHighlightedRow(); cmd != nil {
			cmds = append(cmds, cmd)
		}

		cmds = append(cmds, m.runDetectors()...)

	case parseTemplateForNoteMsg:
		if m.selectedIncident == nil {
			m.setStatus("failed to open editor - no selected incident")
		}
		cmds = append(cmds, parseTemplateForNote(m.selectedIncident))

	case parsedTemplateForNoteMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		cmds = append(cmds, openEditorCmd(m.editor, msg.content))

	case editorFinishedMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		if m.selectedIncident == nil {
			m.setStatus("failed to add note - no selected incident")
			return m, nil
		}

		cmds = append(cmds, addNoteToIncident(m.config, m.selectedIncident, msg.file))

	// Refresh the local copy of the incident after the note is added
	case addedIncidentNoteMsg:
		if msg.err != nil {
			if msg.err.Error() == nilNoteErr {
				m.status = "skipping adding empty note to incident"
				return m, nil
			}
			return m, func() tea.Msg { return errMsg{msg.err} }
		}

		if m.selectedIncident == nil {
			m.setStatus("unable to refresh incident - no selected incident")
			return m, nil
		}

		log.Info("added note to incident", "incident_id", m.selectedIncident.ID)

		// Flash notification for note addition
		cmds = append(cmds, m.flashNotification(fmt.Sprintf("Added note to %s", m.selectedIncident.ID)))

		cmds = append(cmds, func() tea.Msg { return getIncidentMsg(m.selectedIncident.ID) })

	case loginMsg:
		if m.selectedIncident == nil {
			m.setStatus("unable to login - no selected incident")
			return m, nil
		}

		if len(m.selectedIncidentAlerts) == 0 {
			log.Debug("tui.Update()", "msg_type", reflect.TypeOf(msg), "msg", "no alerts found for incident - requeuing", "incident", m.selectedIncident.ID)
			return m, func() tea.Msg { return loginMsg("sender: loginMsg; requeue") }
		}

		clusters := getUniqueClusters(m.selectedIncidentAlerts)

		var cluster string
		switch len(clusters) {
		case 0:
			return m, m.flashNotification("No cluster_id found in alerts — cannot login")
		case 1:
			cluster = clusters[0]
			m.setStatus(fmt.Sprintf("logging into cluster %s", cluster))
		default:
			// Multiple distinct clusters - open scrollable selection view
			m.clusterSelectMode = true
			m.clusterSelectOptions = clusters
			m.clusterSelectPrompt = "Select cluster to log into (Enter=select, Esc=cancel):"
			clusterServices := mapClusterServices(m.selectedIncidentAlerts)
			cols := []table.Column{
				{Title: "Cluster ID", Width: m.layout.ClusterSelectClusterIDWidth},
				{Title: "Service", Width: m.layout.ClusterSelectServiceWidth},
			}
			var rows []table.Row
			for _, c := range clusters {
				rows = append(rows, table.Row{c, clusterServices[c]})
			}
			m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))
			m.clusterSelectTable.SetStyles(m.styles.Table)
			return m, nil
		}

		// NOTE: It's important that **ALL** of these variables' values are NOT NIL.
		// They can be empty strings, but the must not be nil.
		var vars = map[string]string{
			"%%CLUSTER_ID%%":  cluster,
			"%%INCIDENT_ID%%": m.selectedIncident.ID,
		}

		log.Info("login initiated",
			"user_id", m.config.CurrentUser.ID,
			"cluster_id", cluster,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		cmds = append(cmds, login(vars, m.launcher, m.selectedIncident, m.selectedIncidentAlerts, m.selectedIncidentNotes))

	case clusterSelectedMsg:
		if m.selectedIncident == nil {
			m.setStatus("unable to login - no selected incident")
			return m, nil
		}

		cluster := string(msg)
		m.setStatus(fmt.Sprintf("logging into cluster %s", cluster))

		var vars = map[string]string{
			"%%CLUSTER_ID%%":  cluster,
			"%%INCIDENT_ID%%": m.selectedIncident.ID,
		}

		log.Info("login initiated",
			"user_id", m.config.CurrentUser.ID,
			"cluster_id", cluster,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		cmds = append(cmds, login(vars, m.launcher, m.selectedIncident, m.selectedIncidentAlerts, m.selectedIncidentNotes))

	case rosaBoundaryLoginMsg:
		if m.selectedIncident == nil {
			m.setStatus("unable to login via rosa-boundary - no selected incident")
			return m, nil
		}

		if len(m.selectedIncidentAlerts) == 0 {
			log.Debug("tui.Update()", "msg_type", reflect.TypeOf(msg), "msg", "no alerts found for incident - requeuing")
			return m, func() tea.Msg { return rosaBoundaryLoginMsg("requeue") }
		}

		clusters := getUniqueClusters(m.selectedIncidentAlerts)

		var cluster string
		switch len(clusters) {
		case 0:
			return m, m.flashNotification("No cluster_id found in alerts — cannot login via rosa-boundary")
		case 1:
			cluster = clusters[0]
			m.setStatus(fmt.Sprintf("rosa-boundary login to cluster %s", cluster))
		default:
			m.clusterSelectMode = true
			m.rosaBoundaryClusterSelect = true
			m.clusterSelectOptions = clusters
			m.clusterSelectPrompt = "Select cluster for rosa-boundary login (Enter=select, Esc=cancel):"
			clusterServices := mapClusterServices(m.selectedIncidentAlerts)
			cols := []table.Column{
				{Title: "Cluster ID", Width: m.layout.ClusterSelectClusterIDWidth},
				{Title: "Service", Width: m.layout.ClusterSelectServiceWidth},
			}
			var rows []table.Row
			for _, c := range clusters {
				rows = append(rows, table.Row{c, clusterServices[c]})
			}
			m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))
			m.clusterSelectTable.SetStyles(m.styles.Table)
			return m, nil
		}

		var vars = map[string]string{
			"%%CLUSTER_ID%%":  cluster,
			"%%INCIDENT_ID%%": m.selectedIncident.ID,
		}

		log.Info("rosa-boundary login initiated",
			"user_id", m.config.CurrentUser.ID,
			"cluster_id", cluster,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		cmds = append(cmds, rosaBoundaryLogin(vars, m.rosaBoundaryLauncher))

	case rosaBoundaryClusterSelectedMsg:
		if m.selectedIncident == nil {
			m.setStatus("unable to login via rosa-boundary - no selected incident")
			return m, nil
		}

		cluster := string(msg)
		m.setStatus(fmt.Sprintf("rosa-boundary login to cluster %s", cluster))

		var vars = map[string]string{
			"%%CLUSTER_ID%%":  cluster,
			"%%INCIDENT_ID%%": m.selectedIncident.ID,
		}

		log.Info("rosa-boundary login initiated",
			"user_id", m.config.CurrentUser.ID,
			"cluster_id", cluster,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		cmds = append(cmds, rosaBoundaryLogin(vars, m.rosaBoundaryLauncher))

	case loginFinishedMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("failed to login: %s", msg.err)
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		if m.selectedIncident != nil {
			log.Info("login completed", "incident_id", m.selectedIncident.ID)
		} else {
			log.Info("login completed")
		}

	case openBrowserMsg:
		if m.selectedIncident == nil {
			m.setStatus("no incident selected")
			return m, nil
		}
		if defaultBrowserOpenCommand == "" {
			return m, func() tea.Msg { return errMsg{fmt.Errorf("unsupported OS: no browser open command available")} }
		}

		log.Debug("openBrowserMsg", "incident", m.selectedIncident.ID, "title", m.selectedIncident.Title, "service", m.selectedIncident.Service.Summary)
		log.Info("opened incident in browser", "incident_id", m.selectedIncident.ID)

		c := []string{defaultBrowserOpenCommand}
		return m, tea.Batch(m.flashNotification(fmt.Sprintf("Opened %s in browser", m.selectedIncident.ID)), openBrowserCmd(c, m.selectedIncident.HTMLURL))

	case openSOPMsg:
		if m.selectedIncident == nil {
			m.setStatus("no incident selected")
			return m, nil
		}
		link, ok := getSOPLink(m.selectedIncidentAlerts)
		if !ok {
			m.setStatus("no SOP link found")
			return m, nil
		}
		if defaultBrowserOpenCommand == "" {
			return m, func() tea.Msg { return errMsg{fmt.Errorf("unsupported OS: no browser open command available")} }
		}

		log.Debug("openSOPMsg", "incident", m.selectedIncident.ID, "link", link)
		log.Info("opened SOP in browser", "incident_id", m.selectedIncident.ID, "link", link)

		c := []string{defaultBrowserOpenCommand}
		return m, tea.Batch(m.flashNotification(fmt.Sprintf("Opened SOP for %s", m.selectedIncident.ID)), openBrowserCmd(c, link))

	case browserFinishedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("failed to open browser: %s", msg.err))
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		if m.selectedIncident != nil {
			m.setStatus(fmt.Sprintf("opened incident %s in browser - check browser window", m.selectedIncident.ID))
		} else {
			m.setStatus("opened incident in browser - check browser window")
		}
		return m, nil

	// This is a catch all for any action that requires a selected incident
	case waitForSelectedIncidentThenDoMsg:
		if msg.action == nil {
			m.setStatus("failed to perform action: no action included in msg")
			return m, nil
		}
		if msg.msg == nil {
			m.setStatus("failed to perform action: no data included in msg")
			return m, nil
		}

		// If the user has closed the incident view (via ESC) AND there's no highlighted row in the table,
		// abort the action instead of waiting forever for an incident that will never be set
		if m.selectedIncident == nil && !m.viewingIncident && m.table.SelectedRow() == nil {
			log.Debug("Update", "waitForSelectedIncidentThenDoMsg", "aborting action - no incident selected or highlighted", "msg", msg.msg)
			m.setStatus("action cancelled - no incident selected")
			return m, nil
		}

		// Re-queue the message if the selected incident is not yet available
		if m.selectedIncident == nil {
			m.setStatus("waiting for incident info...")
			return m, func() tea.Msg { return msg }
		}

		// Perform the action once the selected incident is available
		log.Debug("Update", "waitForSelectedIncidentThenDoMsg", "performing action", "action", msg.action, "incident", m.selectedIncident.ID)
		return m, msg.action

	case logFileContentMsg:
		m.logViewer.SetContent(wrapLines(string(msg), m.logViewer.Width))
		m.logViewer.GotoBottom()
		m.viewingLog = true
		return m, nil

	case renderIncidentMsg:
		if m.selectedIncident == nil {
			m.setStatus("failed render incidents - no incidents provided")
			m.viewingIncident = false
			return m, nil
		}

		cmds = append(cmds, renderIncident(&m))

	case renderedIncidentMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		log.Debug("renderedIncidentMsg")

		// Only set viewing state if we still have a selected incident
		// This prevents late-arriving render messages from reopening the incident view
		// after the user has already closed it with ESC
		if m.selectedIncident != nil {
			wasViewingBefore := m.viewingIncident
			m.incidentViewer.SetContent(msg.content)
			// Only go to top on first render, not on progressive updates
			if !wasViewingBefore {
				m.incidentViewer.GotoTop()
			}
			m.viewingIncident = true
		} else {
			log.Debug("renderedIncidentMsg", "action", "discarding render - incident was closed")
		}

	case noAcknowledgeMsg:
		// Background auto-ack sweep found nothing to acknowledge (user not on-call,
		// on-call check failed, or no candidate matched). No-op — deliberately NOT an
		// acknowledgeIncidentsMsg, whose nil-incidents fallback would ack the selected
		// incident.
		return m, nil

	case acknowledgeIncidentsMsg:
		// If incidents are provided in the message, use those
		// Otherwise, use the selected incident (which is always synced to highlighted row)
		incidents := msg.incidents
		if incidents == nil {
			if m.selectedIncident == nil {
				m.setStatus("failed acknowledging incidents - no incident selected")
				return m, nil
			}
			incidents = []pagerduty.Incident{*m.selectedIncident}
		}

		m.apiInProgress = true
		return m, tea.Sequence(
			m.spinner.Tick,
			acknowledgeIncidents(m.config, incidents),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: acknowledgeIncidentsMsg") },
		)

	case unAcknowledgeIncidentsMsg:
		// If incidents are provided in the message, use those
		// Otherwise, use the selected incident (which is always synced to highlighted row)
		incidents := msg.incidents
		if incidents == nil {
			if m.selectedIncident == nil {
				m.setStatus("failed re-escalating incidents - no incident selected")
				return m, nil
			}
			incidents = []pagerduty.Incident{*m.selectedIncident}
		}

		// Skip un-acknowledge step - go directly to re-escalation
		// Re-escalation will reassign to the current on-call at the escalation level
		// Group incidents by their current escalation policy ID
		policyGroups := make(map[string][]pagerduty.Incident)
		for _, incident := range incidents {
			// Use the incident's actual escalation policy, not a service-based lookup
			if incident.EscalationPolicy.ID != "" {
				policyGroups[incident.EscalationPolicy.ID] = append(policyGroups[incident.EscalationPolicy.ID], incident)
			} else {
				log.Warn("tui.unAcknowledgeIncidentsMsg", "incident has no escalation policy", "incident_id", incident.ID)
			}
		}

		// Create re-escalate commands for each policy group
		var cmds []tea.Cmd
		reescalateLevel := m.reescalateLevel
		if reescalateLevel == 0 {
			reescalateLevel = reEscalateDefaultPolicyLevel
		}
		for policyID, incidents := range policyGroups {
			// Fetch the full escalation policy details for this policy ID
			cmd := fetchEscalationPolicyAndReEscalate(m.config, incidents, policyID, reescalateLevel)
			cmds = append(cmds, cmd)
		}

		// Add clear selected incidents after re-escalation
		cmds = append(cmds, func() tea.Msg { return clearSelectedIncidentsMsg("sender: unAcknowledgeIncidentsMsg") })

		if len(cmds) > 0 {
			m.apiInProgress = true
			cmds = append([]tea.Cmd{m.spinner.Tick}, cmds...)
			return m, tea.Sequence(cmds...)
		}

		return m, func() tea.Msg { return updateIncidentListMsg("sender: unAcknowledgeIncidentsMsg") }

	case acknowledgedIncidentsMsg:
		m.apiInProgress = false
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		incidentIDs := strings.Join(getIDsFromIncidents(msg.incidents), " ")
		log.Info("acknowledged incident", "incident_id", incidentIDs)

		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Acknowledged %s", incidentIDs)),
			func() tea.Msg { return updateIncidentListMsg("sender: acknowledgedIncidentsMsg") },
		)

	case reassignIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed reassigning incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			reassignIncidents(m.config, msg.incidents, msg.users),
			func() tea.Msg { return clearSelectedIncidentsMsg("reassign incidents") },
		)

	case reassignedIncidentsMsg:
		incidentIDs := getIDsFromIncidents(msg)
		log.Info("reassigned incidents", "incident_ids", incidentIDs)
		m.setStatus(fmt.Sprintf("reassigned incidents %v; refreshing Incident List ", incidentIDs))
		return m, func() tea.Msg { return updateIncidentListMsg("sender: reassignedIncidentsMsg") }

	case reEscalateIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed re-escalating incidents - no incidents provided")
			return m, nil
		}

		return m, tea.Sequence(
			reEscalateIncidents(m.config, msg.incidents, msg.policy, msg.level),
			func() tea.Msg { return clearSelectedIncidentsMsg("sender: reEscalatedIncidentsMsg") },
		)

	case reEscalatedIncidentsMsg:
		m.apiInProgress = false
		incidentIDs := strings.Join(getIDsFromIncidents(msg), " ")
		log.Info("re-escalated incident",
			"user_id", m.config.CurrentUser.ID,
			"reason", func() string {
				if m.selectedIncident != nil {
					return m.selectedIncident.HTMLURL
				}
				return ""
			}(),
			"alert", func() string {
				if m.selectedIncident != nil {
					return alert.ExtractAlertName(m.selectedIncident.Title)
				}
				return ""
			}())

		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Re-escalated %s", incidentIDs)),
			func() tea.Msg { return updateIncidentListMsg("sender: reEscalatedIncidentsMsg") },
		)

	case silenceSelectedIncidentMsg:
		if m.selectedIncident == nil {
			m.setStatus("failed silencing incident - no incident selected")
			return m, nil
		}

		incidentID := m.selectedIncident.ID
		log.Info("silenced incident",
			"user_id", m.config.CurrentUser.ID,
			"reason", m.selectedIncident.HTMLURL,
			"alert", alert.ExtractAlertName(m.selectedIncident.Title))
		policyKey := getEscalationPolicyKey(m.selectedIncident.Service.ID, m.config.EscalationPolicies)

		m.apiInProgress = true
		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Silenced %s", incidentID)),
			tea.Sequence(
				m.spinner.Tick,
				silenceIncidents([]pagerduty.Incident{*m.selectedIncident}, m.config.EscalationPolicies[policyKey], silentDefaultPolicyLevel),
				func() tea.Msg { return clearSelectedIncidentsMsg("sender: silenceSelectedIncidentMsg") },
			),
		)

	case enterBulkSilenceMsg:
		var options []huh.Option[string]
		for _, inc := range m.incidentList {
			label := fmt.Sprintf("%s — %s — %s", inc.ID, inc.Service.Summary, inc.Title)
			options = append(options, huh.NewOption(label, inc.ID))
		}
		if len(options) == 0 {
			m.setStatus("no incidents to silence")
			return m, nil
		}
		m.bulkSilenceIDs = nil

		theme := huh.ThemeCharm()
		theme.Focused.Title = theme.Focused.Title.Foreground(m.theme.Highlight)
		theme.Focused.Description = theme.Focused.Description.Foreground(m.theme.Muted)
		theme.Focused.SelectedOption = theme.Focused.SelectedOption.Foreground(m.theme.Highlight)
		theme.Focused.UnselectedOption = theme.Focused.UnselectedOption.Foreground(m.theme.Text)
		theme.Focused.MultiSelectSelector = theme.Focused.MultiSelectSelector.Foreground(m.theme.Text)
		theme.Focused.SelectedPrefix = theme.Focused.SelectedPrefix.Foreground(m.theme.Highlight)
		theme.Focused.UnselectedPrefix = theme.Focused.UnselectedPrefix.Foreground(m.theme.Muted)
		theme.Focused.Base = theme.Focused.Base.BorderForeground(m.theme.Border)

		m.bulkSilenceForm = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select incidents to silence").
					Description("Space to toggle, a to select all, enter to confirm, esc to cancel").
					Options(options...).
					Filterable(true).
					Value(&m.bulkSilenceIDs),
			),
		).WithTheme(theme).WithHeight(m.layout.TeamSelectFormHeight)
		m.bulkSilenceMode = true
		return m, m.bulkSilenceForm.Init()

	case bulkSilenceConfirmedMsg:
		if len(msg.incidents) == 0 {
			m.setStatus("no incidents to silence")
			return m, nil
		}

		var cmds []tea.Cmd
		for _, inc := range msg.incidents {
			policyKey := getEscalationPolicyKey(inc.Service.ID, m.config.EscalationPolicies)
			policy := m.config.EscalationPolicies[policyKey]
			cmds = append(cmds, silenceIncidents([]pagerduty.Incident{inc}, policy, silentDefaultPolicyLevel))
		}

		incidentIDs := strings.Join(getIDsFromIncidents(msg.incidents), " ")
		cmds = append(cmds, func() tea.Msg { return clearSelectedIncidentsMsg("sender: bulkSilenceConfirmedMsg") })

		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Silenced %s", incidentIDs)),
			tea.Sequence(cmds...),
		)

	case silenceIncidentsMsg:
		if msg.incidents == nil {
			m.setStatus("failed silencing incidents - no incidents provided")
			return m, nil
		}

		incidents := msg.incidents
		if m.selectedIncident != nil {
			incidents = append(msg.incidents, *m.selectedIncident)
		}

		var cmds []tea.Cmd
		for _, inc := range incidents {
			policyKey := getEscalationPolicyKey(inc.Service.ID, m.config.EscalationPolicies)
			policy := m.config.EscalationPolicies[policyKey]
			cmds = append(cmds, silenceIncidents([]pagerduty.Incident{inc}, policy, silentDefaultPolicyLevel))
		}

		incidentIDs := strings.Join(getIDsFromIncidents(incidents), " ")
		cmds = append(cmds, func() tea.Msg { return clearSelectedIncidentsMsg("sender: silenceIncidentsMsg") })

		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Silenced %s", incidentIDs)),
			tea.Sequence(cmds...),
		)

	case clearSelectedIncidentsMsg:
		m.clearSelectedIncident(msg)
		return m, nil

	case claudePromptMsg:
		return m.handleClaudePrompt(msg, defaultLookPath)

	case claudeResponseMsg:
		return m.handleClaudeResponse(msg)

	case mergeIncidentMsg:
		if m.mergeSourceIncident == nil || m.mergeTargetID == "" {
			m.setStatus("merge failed - missing source or target")
			return m, nil
		}
		sourceID := m.mergeSourceIncident.ID
		targetID := m.mergeTargetID
		m.mergeMode = false
		m.mergeSourceIncident = nil
		m.mergeTargetID = ""
		m.table.Focus()
		m.apiInProgress = true
		return m, tea.Batch(m.spinner.Tick, mergeIncidents(m.config, sourceID, targetID))

	case mergedIncidentMsg:
		m.apiInProgress = false
		if msg.err != nil {
			return m, func() tea.Msg { return errMsg{msg.err} }
		}
		log.Info("merged incident",
			"user_id", m.config.CurrentUser.ID,
			"source", msg.sourceID,
			"target", msg.targetID)
		return m, tea.Batch(
			m.flashNotification(fmt.Sprintf("Merged %s into %s", msg.sourceID, msg.targetID)),
			func() tea.Msg { return updateIncidentListMsg("sender: mergedIncidentMsg") },
		)

	case clusterInfoMsg:
		delete(m.clusterEnrichInFlight, msg.clusterID)
		if msg.err != nil {
			log.Debug("ocm.GetCluster failed", "cluster_id", msg.clusterID, "error", msg.err)
			if m.clusterEnrichFailed == nil {
				m.clusterEnrichFailed = make(map[string]int)
			}
			m.clusterEnrichFailed[msg.clusterID]++
			return m, m.flashNotification(fmt.Sprintf("OCM: cluster lookup failed for %s", msg.clusterID))
		}
		if m.clusterCache == nil {
			m.clusterCache = make(map[string]*ocm.ClusterInfo)
		}
		m.clusterCache[msg.clusterID] = msg.info
		log.Info("OCM enriched cluster", "cluster_id", msg.clusterID, "name", msg.info.DisplayName, "region", msg.info.Region)

		m.rebuildFlagMatchCache()

		// Phase 2: dispatch service logs and limited support using the
		// OCM internal ID for the API call but the PD cluster ID as cache key.
		// Skip if already cached (prevents duplicate calls from repeated alerts).
		internalID := msg.info.ID
		externalID := msg.info.ExternalID
		cacheKey := msg.clusterID
		var phase2Cmds []tea.Cmd
		if m.ocmClient != nil {
			if _, cached := m.serviceLogCache[cacheKey]; !cached {
				phase2Cmds = append(phase2Cmds,
					getOCMServiceLogs(m.ocmClient, internalID, externalID, cacheKey),
				)
			}
			if _, cached := m.limitedSupportCache[cacheKey]; !cached {
				phase2Cmds = append(phase2Cmds,
					getLimitedSupportHistory(m.ocmClient, internalID, cacheKey),
				)
			}
		}
		if m.backplaneClient != nil {
			if _, cached := m.clusterReportCache[cacheKey]; !cached {
				phase2Cmds = append(phase2Cmds,
					getClusterReports(m.backplaneClient, internalID, cacheKey),
				)
			}
		}

		// Rebuild table immediately so display names update
		incidents := m.incidentList
		clusterID := msg.clusterID
		flashMsg := fmt.Sprintf("OCM enriched cluster %s", clusterID)
		rebuildAndFlash := tea.Sequence(
			func() tea.Msg { return updatedIncidentListMsg{incidents, nil} },
			func() tea.Msg { return setStatusMsg{flashMsg} },
			tea.Tick(4*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{message: flashMsg} }),
		)

		allCmds := append(phase2Cmds, rebuildAndFlash)
		if m.viewingIncident {
			allCmds = append(allCmds, func() tea.Msg { return renderIncidentMsg("cluster info arrived") })
		}
		return m, tea.Batch(allCmds...)

	case ocmServiceLogsMsg:
		if msg.err != nil {
			log.Debug("ocm.GetServiceLogs failed", "cluster_id", msg.clusterID, "error", msg.err)
			return m, nil
		}
		if m.serviceLogCache == nil {
			m.serviceLogCache = make(map[string][]ocm.ServiceLog)
		}
		m.serviceLogCache[msg.clusterID] = msg.logs
		log.Info("service logs fetched", "cluster_id", msg.clusterID, "count", len(msg.logs))
		return m, nil

	case limitedSupportMsg:
		if msg.err != nil {
			log.Debug("ocm.GetLimitedSupportHistory failed", "cluster_id", msg.clusterID, "error", msg.err)
			return m, nil
		}
		if m.limitedSupportCache == nil {
			m.limitedSupportCache = make(map[string][]ocm.LimitedSupportReason)
		}
		m.limitedSupportCache[msg.clusterID] = msg.reasons
		log.Info("limited support fetched", "cluster_id", msg.clusterID, "count", len(msg.reasons))
		return m, nil

	case clusterReportsMsg:
		if msg.err != nil {
			log.Debug("backplane.ListReports failed", "cluster_id", msg.clusterID, "error", msg.err)
			return m, nil
		}
		if m.clusterReportCache == nil {
			m.clusterReportCache = make(map[string][]backplane.Report)
		}
		m.clusterReportCache[msg.clusterID] = msg.reports
		log.Info("cluster reports fetched", "cluster_id", msg.clusterID, "count", len(msg.reports))
		return m, nil

	case priorAlertsMsg:
		if m.priorAlertPending[msg.clusterID] > 0 {
			m.priorAlertPending[msg.clusterID]--
		}
		allWeeksDone := m.priorAlertPending[msg.clusterID] == 0
		if allWeeksDone {
			delete(m.priorAlertPending, msg.clusterID)
		}
		if m.priorAlertCache == nil {
			m.priorAlertCache = make(map[string]*PriorAlertData)
		}
		existing := m.priorAlertCache[msg.clusterID]
		if existing == nil {
			existing = &PriorAlertData{}
			m.priorAlertCache[msg.clusterID] = existing
		}
		if msg.err != nil {
			log.Debug("fetchPriorAlerts week failed", "cluster_id", msg.clusterID, "error", msg.err)
		} else if msg.data != nil {
			existing.SameAlert = append(existing.SameAlert, msg.data.SameAlert...)
			existing.OtherAlerts = append(existing.OtherAlerts, msg.data.OtherAlerts...)
			log.Debug("fetchPriorAlerts week merged", "cluster_id", msg.clusterID,
				"week_same", len(msg.data.SameAlert), "week_other", len(msg.data.OtherAlerts),
				"total_same", len(existing.SameAlert), "total_other", len(existing.OtherAlerts),
				"pending", m.priorAlertPending[msg.clusterID])
		}
		if allWeeksDone {
			log.Info("PD history complete", "cluster_id", msg.clusterID,
				"same_alert", len(existing.SameAlert), "other_alerts", len(existing.OtherAlerts))
		}
		var nextCmd tea.Cmd
		if len(msg.nextWeeks) > 0 {
			nextCmd = fetchPriorAlertsWeek(
				msg.client,
				msg.serviceIDs,
				msg.teamIDs,
				msg.clusterID,
				msg.currentAlertName,
				msg.incidentID,
				msg.nextWeeks[0],
				msg.nextWeeks[1:],
			)
		}
		if m.viewingIncident && m.activeTab == tabPDHistory {
			renderCmd := func() tea.Msg { return renderIncidentMsg("prior alerts arrived") }
			if nextCmd != nil {
				return m, tea.Batch(renderCmd, nextCmd)
			}
			return m, renderCmd
		}
		if nextCmd != nil {
			return m, nextCmd
		}
		return m, nil

	case updateAvailableMsg:
		m.updateAvailable = true
		m.updateVersion = msg.latest
		m.updateReleaseURL = msg.releaseURL
		return m, nil
	}

	if m.configMode && m.configForm != nil {
		form, cmd := m.configForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.configForm = f
		}
		if m.configForm.State == huh.StateCompleted || m.configForm.State == huh.StateAborted {
			result, resultCmd := switchConfigFocusMode(m, msg)
			return result, resultCmd
		}
		cmds = append(cmds, cmd)
	}

	if m.teamSelectMode && m.teamSelectForm != nil {
		form, cmd := m.teamSelectForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.teamSelectForm = f
		}
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)

}

func (m *model) buildConfigForm(msg configWizardReadyMsg, tokenDesc, keepTeamsDesc, keepSilentDesc, keepCustomDesc string, existingTeamSet map[string]bool) *huh.Form {
	var fetchedTeams []pagerduty.Team
	submitted := false

	theme := huh.ThemeCharm()
	theme.Focused.Title = theme.Focused.Title.Foreground(m.theme.Highlight)
	theme.Focused.Description = theme.Focused.Description.Foreground(m.theme.Muted)
	theme.Focused.SelectedOption = theme.Focused.SelectedOption.Foreground(m.theme.Highlight)
	theme.Focused.UnselectedOption = theme.Focused.UnselectedOption.Foreground(m.theme.Text)
	theme.Focused.MultiSelectSelector = theme.Focused.MultiSelectSelector.Foreground(m.theme.Text)
	theme.Focused.SelectedPrefix = theme.Focused.SelectedPrefix.Foreground(m.theme.Highlight)
	theme.Focused.UnselectedPrefix = theme.Focused.UnselectedPrefix.Foreground(m.theme.Muted)
	theme.Focused.Base = theme.Focused.Base.BorderForeground(m.theme.Border)

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "ctrl+q"), key.WithHelp("ctrl+q/ctrl+c", "quit"))
	km.Input.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back"))
	km.Input.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "next"))
	km.Select.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back"))
	km.MultiSelect.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back"))
	km.Note.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back"))
	km.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back"))

	clientFactory := m.pdClientFactory
	if clientFactory == nil {
		clientFactory = pd.NewClient
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("PagerDuty API token").
				Description(tokenDesc).
				EchoMode(huh.EchoModePassword).
				Value(&m.configState.TokenInput).
				Validate(func(s string) error {
					token := strings.TrimSpace(s)
					if token == "" {
						if msg.existing.Token != "" {
							return nil
						}
						return fmt.Errorf("a PagerDuty API token is required")
					}
					client := clientFactory(token)
					_, err := pd.GetCurrentUserTeams(client)
					if err != nil {
						return fmt.Errorf("invalid token: %v", err)
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Keep current teams?").
				Description(keepTeamsDesc).
				Value(&m.configState.KeepTeams),
		).WithHideFunc(func() bool { return !msg.kd.HasValidTeams }),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select your PagerDuty teams").
				Description("Select the team(s) whose incidents you want to monitor. Most users only need one.").
				OptionsFunc(func() []huh.Option[string] {
					token := strings.TrimSpace(m.configState.TokenInput)
					if token == "" {
						token = msg.existing.Token
					}
					if token == "" {
						return []huh.Option[string]{
							huh.NewOption("(enter a token first)", ""),
						}
					}
					client := clientFactory(token)
					teams, err := pd.GetCurrentUserTeams(client)
					if err != nil {
						return []huh.Option[string]{
							huh.NewOption(fmt.Sprintf("Error: %v", err), ""),
						}
					}
					fetchedTeams = teams
					var opts []huh.Option[string]
					for _, team := range teams {
						opt := huh.NewOption(
							fmt.Sprintf("%s — %s", team.Name, team.ID), team.ID,
						)
						if existingTeamSet[team.ID] {
							opt = opt.Selected(true)
						}
						opts = append(opts, opt)
					}
					return opts
				}, &m.configState.TokenInput).
				Value(&m.configState.SelectedTeams).
				Validate(func(s []string) error {
					if !submitted {
						submitted = true
						return nil
					}
					if len(s) == 0 {
						return fmt.Errorf("at least one team is required")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return m.configState.KeepTeams }),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Keep current silent escalation policy?").
				Description(keepSilentDesc).
				Value(&m.configState.KeepSilent),
		).WithHideFunc(func() bool { return !msg.kd.HasSilent }),
		huh.NewGroup(
			huh.NewInput().
				Title("Default silent escalation policy").
				Description(
					"When you silence an incident, it gets reassigned to this policy —\n"+
						"one that routes only to bot users, not on-call humans.\n"+
						"Find the ID at People → Escalation Policies (ID is in the URL,\n"+
						"e.g., PXXXXXX). Look for a policy like \"Silent Test\".\n"+
						"Leave blank to configure later.",
				).
				Value(&m.configState.SilentPolicy),
		).WithHideFunc(func() bool { return m.configState.KeepSilent }),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Keep current custom service-to-policy mappings?").
				Description(keepCustomDesc).
				Value(&m.configState.KeepCustom),
		).WithHideFunc(func() bool { return !msg.kd.HasCustom }),
		huh.NewGroup(
			huh.NewInput().
				Title("Custom service-to-policy mappings").
				Description(
					"Some services need a different silent policy than the default.\n"+
						"For example, Deadmanssnitch alerts might route to a separate\n"+
						"silent policy. Find service IDs in Services → Service Directory\n"+
						"(ID in URL). Enter as SERVICE_ID:POLICY_ID separated by commas.\n"+
						"Leave blank to skip.",
				).
				Value(&m.configState.CustomInput),
		).WithHideFunc(func() bool { return m.configState.KeepCustom }),
		huh.NewGroup(
			huh.NewNote().
				Title("Configuration summary").
				DescriptionFunc(func() string {
					tmpFinal, _ := pkgconfig.ResolveFinalValues(m.configExisting, pkgconfig.WizardInputs{
						TokenInput:          m.configState.TokenInput,
						SelectedTeams:       m.configState.SelectedTeams,
						SilentPolicyID:      m.configState.SilentPolicy,
						CustomMappingsInput: m.configState.CustomInput,
						KeepTeams:           m.configState.KeepTeams,
						KeepSilent:          m.configState.KeepSilent,
						KeepCustom:          m.configState.KeepCustom,
					})
					tmpNames := make(map[string]string)
					for k, v := range m.configTeamNames {
						tmpNames[k] = v
					}
					for _, team := range fetchedTeams {
						tmpNames[team.ID] = team.Name
					}
					var tmpChanges pkgconfig.ConfigChanges
					if m.configIsNewFile {
						tmpChanges = pkgconfig.DetectChangesForNewFile(tmpFinal)
					} else {
						tmpChanges = pkgconfig.DetectChanges(m.configExisting, tmpFinal, strings.TrimSpace(m.configState.TokenInput))
					}
					return pkgconfig.BuildSummary(m.configExisting, tmpFinal, tmpChanges, tmpNames, m.configPolicyNames)
				}, &m.configState.CustomInput),
			huh.NewConfirm().
				Title("Save changes?").
				Value(&m.configState.Confirm),
		),
	).WithTheme(theme).WithKeyMap(km).WithWidth(m.layout.FormWidth).WithHeight(m.layout.FormHeight)
}
