package tui

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/clcollins/srepd/pkg/delta"
)

const (
	emojiWatcherMarker   = "📡 "
	emojiAgentMarker     = "🤖 "
	noEmojiWatcherMarker = "☺ "
	noEmojiAgentMarker   = "☻ "
)

type markers struct {
	flag    string
	watcher string
	agent   string
}

func resolveMarkers(useEmoji bool) markers {
	if useEmoji {
		return markers{
			flag:    emojiFlagMarker,
			watcher: emojiWatcherMarker,
			agent:   emojiAgentMarker,
		}
	}
	return markers{
		flag:    noEmojiFlagMarker,
		watcher: noEmojiWatcherMarker,
		agent:   noEmojiAgentMarker,
	}
}

type watcherBuffer struct {
	entries  []string
	capacity int
}

func newWatcherBuffer(capacity int) *watcherBuffer {
	return &watcherBuffer{
		entries:  make([]string, 0, capacity),
		capacity: capacity,
	}
}

func (b *watcherBuffer) Append(entry string) {
	entry = stripControl(entry)
	if len(b.entries) >= b.capacity {
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, entry)
}

func (b *watcherBuffer) SetLast(entry string) {
	entry = stripControl(entry)
	if len(b.entries) == 0 {
		b.Append(entry)
		return
	}
	b.entries[len(b.entries)-1] = entry
}

func (b *watcherBuffer) Content() string {
	return strings.Join(b.entries, "\n---\n")
}

func (b *watcherBuffer) Len() int {
	return len(b.entries)
}

func (b *watcherBuffer) Clear() {
	b.entries = b.entries[:0]
}

func (m *model) updateWatcherViewport() {
	content := m.watcherBuffer.Content()
	if m.watcherViewport.Width > 0 {
		content = lipgloss.NewStyle().Width(m.watcherViewport.Width).Render(content)
	}
	m.watcherViewport.SetContent(content)
	m.watcherViewport.GotoBottom()

	if m.chatMode {
		m.updateChatViewport()
	}
}

func (m *model) updateChatViewport() {
	content := m.watcherBuffer.Content()
	if m.chatViewport.Width > 0 {
		content = lipgloss.NewStyle().Width(m.chatViewport.Width).Render(content)
	}
	wasAtBottom := m.chatViewport.AtBottom()
	m.chatViewport.SetContent(content)
	if wasAtBottom {
		m.chatViewport.GotoBottom()
	}
}

func (m *model) chatViewportGotoBottom() {
	m.chatViewport.GotoBottom()
}

const (
	typewriterWordsPerTick = 3
	typewriterTickInterval = 30 * time.Millisecond
)

type typewriterState struct {
	words   []string
	index   int
	marker  string
	partial string
}

type typewriterTickMsg struct{}

func splitKeepingNewlines(text string) []string {
	var tokens []string
	for _, line := range strings.Split(text, "\n") {
		words := strings.Fields(line)
		tokens = append(tokens, words...)
		tokens = append(tokens, "\n")
	}
	if len(tokens) > 0 {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

func (m *model) startTypewriter(marker string, text string) tea.Cmd {
	words := splitKeepingNewlines(text)
	if len(words) == 0 {
		return nil
	}
	m.typewriter = &typewriterState{
		words:  words,
		marker: marker,
	}
	return tea.Tick(typewriterTickInterval, func(time.Time) tea.Msg {
		return typewriterTickMsg{}
	})
}

func (m *model) advanceTypewriter() tea.Cmd {
	tw := m.typewriter
	if tw == nil {
		return nil
	}

	end := tw.index + typewriterWordsPerTick
	if end > len(tw.words) {
		end = len(tw.words)
	}

	for i := tw.index; i < end; i++ {
		word := tw.words[i]
		if word == "\n" {
			tw.partial += "\n"
		} else if tw.partial == "" || strings.HasSuffix(tw.partial, "\n") {
			tw.partial += word
		} else {
			tw.partial += " " + word
		}
	}
	tw.index = end

	m.watcherBuffer.SetLast(prefixMessage(tw.marker, tw.partial))
	m.updateWatcherViewport()

	if tw.index >= len(tw.words) {
		m.typewriter = nil
		return nil
	}

	return tea.Tick(typewriterTickInterval, func(time.Time) tea.Msg {
		return typewriterTickMsg{}
	})
}

type watcherObservation struct {
	Summary     string
	IncidentIDs []string // triggering incident IDs for scoped context
}

type watcherDedup struct {
	seen     map[string]time.Time
	cooldown time.Duration
}

func newWatcherDedup(cooldown time.Duration) *watcherDedup {
	return &watcherDedup{
		seen:     make(map[string]time.Time),
		cooldown: cooldown,
	}
}

const watcherDedupEvictThreshold = 100

func (d *watcherDedup) IsNew(observation string) bool {
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(observation)))
	if last, ok := d.seen[h]; ok && time.Since(last) < d.cooldown {
		return false
	}
	d.seen[h] = time.Now()
	if len(d.seen) > watcherDedupEvictThreshold {
		d.evictExpired()
	}
	return true
}

func (d *watcherDedup) evictExpired() {
	for k, ts := range d.seen {
		if time.Since(ts) >= d.cooldown {
			delete(d.seen, k)
		}
	}
}

func (m *model) runDetectors(changes []delta.Change) []tea.Cmd {
	if len(m.incidentList) < 2 {
		return nil
	}

	// D1 gate: only investigate when the incident state actually changed.
	// On first poll (no previous state), every incident is a first-sighting
	// and produces IncidentNew changes — that is correct.
	if len(changes) == 0 {
		return nil
	}

	observations := detectAll(m.incidentList, m.incidentClusterMap)

	var cmds []tea.Cmd
	added := false
	for _, obs := range observations {
		// Secondary rate limit: suppress re-investigation of the same
		// observation text within the cooldown window, even if the delta
		// gate fires (e.g. an unrelated field changed).
		if !m.watcherDedup.IsNew(obs.Summary) {
			continue
		}

		log.Debug("watcher.runDetectors", "observation", obs.Summary)

		if m.aiProvider != nil && m.aiHealth != aiHealthError && !m.watcherAnalyzing {
			m.watcherAnalyzing = true
			m.watcherQueryStart = time.Now()
			m.watcherQueryTimeout = watcherSynthesisTimeout

			if m.toolRunnerFactory != nil && m.toolRegistry != nil && isAnthropicFamily(m.aiProvider.Name()) {
				m.watcherQueryTimeout = m.investigationCfg.timeout
				contextStr := buildObservationContext(m, obs)
				changesNarrative := delta.Narrate(changes, time.Now())
				if changesNarrative != "" {
					contextStr = changesNarrative + "\n\n" + contextStr
				}
				cmds = append(cmds, watcherInvestigateCmd(
					m.toolRunnerFactory,
					m.toolRegistry,
					m.investigationCfg,
					m.watcherSystemPrompt,
					obs.Summary,
					contextStr,
					ai.ResolvedModel(m.aiProvider),
					nil,
					obs.IncidentIDs,
				))
			} else {
				summary := buildIncidentSummary(m.incidentList)
				cmds = append(cmds, watcherSynthesizeCmd(m.aiProvider, m.watcherSystemPrompt, obs.Summary, summary))
			}
		} else {
			m.watcherBuffer.Append(prefixMessage(m.watcherMarker, obs.Summary))
			added = true
		}
	}

	if added {
		if !m.watcherExpanded {
			m.watcherExpanded = true
			m.recomputeLayout()
		}
		m.updateWatcherViewport()
	}

	return cmds
}

const maxRecentChanges = 200

func toSnapshots(incidents []pagerduty.Incident, cache map[string]*cachedIncidentData) []delta.Snapshot {
	snaps := make([]delta.Snapshot, 0, len(incidents))
	for _, inc := range incidents {
		var noteCount, alertCount *int
		if c, ok := cache[inc.ID]; ok {
			if c.notesLoaded {
				n := len(c.notes)
				noteCount = &n
			}
			if c.alertsLoaded {
				a := len(c.alerts)
				alertCount = &a
			}
		}
		snaps = append(snaps, delta.SnapshotFromFields(
			inc.ID, inc.Title, inc.Service.Summary,
			inc.Status, inc.Urgency,
			noteCount, alertCount,
		))
	}
	return snaps
}

func (m *model) computeAndStoreDeltas() []delta.Change {
	curr := toSnapshots(m.incidentList, m.incidentCache)
	changes := delta.Diff(m.prevSnapshots, curr)
	m.prevSnapshots = curr

	if len(changes) > 0 {
		m.recentChanges = append(m.recentChanges, changes...)
		if len(m.recentChanges) > maxRecentChanges {
			m.recentChanges = m.recentChanges[len(m.recentChanges)-maxRecentChanges:]
		}
	}

	return changes
}

func buildIncidentSummary(incidents []pagerduty.Incident) string {
	var lines []string
	for _, inc := range incidents {
		lines = append(lines, fmt.Sprintf("- [%s] %s (%s, %s)", inc.ID, inc.Title, inc.Service.Summary, inc.Urgency))
	}
	return strings.Join(lines, "\n")
}

func detectAll(incidents []pagerduty.Incident, clusterMap map[string][]string) []watcherObservation {
	var observations []watcherObservation
	observations = append(observations, detectServiceStorm(incidents)...)
	observations = append(observations, detectClusterStorm(incidents, clusterMap)...)
	observations = append(observations, detectUrgencyShift(incidents)...)
	return observations
}

func detectServiceStorm(incidents []pagerduty.Incident) []watcherObservation {
	serviceIncidents := make(map[string][]string)
	serviceCounts := make(map[string]int)
	for _, inc := range incidents {
		serviceCounts[inc.Service.Summary]++
		serviceIncidents[inc.Service.Summary] = append(serviceIncidents[inc.Service.Summary], inc.ID)
	}

	var observations []watcherObservation
	for svc, count := range serviceCounts {
		if count >= 3 {
			observations = append(observations, watcherObservation{
				Summary:     fmt.Sprintf("Service storm: %d incidents on %s", count, svc),
				IncidentIDs: serviceIncidents[svc],
			})
		}
	}
	return observations
}

func detectClusterStorm(incidents []pagerduty.Incident, clusterMap map[string][]string) []watcherObservation {
	if clusterMap == nil {
		return nil
	}

	clusterCounts := make(map[string]int)
	clusterIncidents := make(map[string][]string)
	for _, inc := range incidents {
		for _, clusterID := range clusterMap[inc.ID] {
			clusterCounts[clusterID]++
			clusterIncidents[clusterID] = append(clusterIncidents[clusterID], inc.ID)
		}
	}

	var observations []watcherObservation
	for cluster, count := range clusterCounts {
		if count >= 2 {
			observations = append(observations, watcherObservation{
				Summary:     fmt.Sprintf("Cluster storm: %d incidents on cluster %s", count, cluster),
				IncidentIDs: clusterIncidents[cluster],
			})
		}
	}
	return observations
}

func detectUrgencyShift(incidents []pagerduty.Incident) []watcherObservation {
	highCount := 0
	var highIDs []string
	for _, inc := range incidents {
		if inc.Urgency == "high" {
			highCount++
			highIDs = append(highIDs, inc.ID)
		}
	}

	if highCount >= 3 {
		return []watcherObservation{{
			Summary:     fmt.Sprintf("High urgency cluster: %d/%d incidents are high urgency", highCount, len(incidents)),
			IncidentIDs: highIDs,
		}}
	}
	return nil
}

type watcherPromptMsg struct {
	prompt string
}

func isWatcherCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, ":watcher ") || trimmed == ":watcher"
}

func parseWatcherQuery(input string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), ":watcher"))
}

// buildObservationContext derives context from the observation's triggering
// incidents, never from m.selectedIncident. Implements design choice (c):
// triggering incidents are foregrounded with sibling alerts labelled as background.
func buildObservationContext(m *model, obs watcherObservation) string {
	var parts []string

	for i, incID := range obs.IncidentIDs {
		inc := findIncidentByID(m.incidentList, incID)
		if inc == nil {
			continue
		}

		label := "Triggering incident"
		if i > 0 {
			label = "Related incident"
		}
		parts = append(parts, fmt.Sprintf("%s: %s (%s)", label, inc.Title, inc.ID))
		parts = append(parts, fmt.Sprintf("Service: %s", inc.Service.Summary))
		parts = append(parts, fmt.Sprintf("Status: %s, Urgency: %s", inc.Status, inc.Urgency))

		var alerts []pagerduty.IncidentAlert
		if cached, ok := m.incidentCache[inc.ID]; ok && cached.alertsLoaded {
			alerts = cached.alerts
		}

		for _, alert := range alerts {
			if details, ok := alert.Body["details"].(map[string]interface{}); ok {
				if name, ok := details["alert_name"].(string); ok {
					parts = append(parts, fmt.Sprintf("Alert: %s", name))
				}
				if sopURL, ok := details["firing"].(string); ok && sopURL != "" {
					parts = append(parts, fmt.Sprintf("SOP: %s", sopURL))
				}
				if cluster, ok := details["cluster_id"].(string); ok {
					parts = append(parts, fmt.Sprintf("Cluster: %s", cluster))
					parts = append(parts, buildClusterContext(m, cluster)...)
				}
			}
		}

		var notes []pagerduty.IncidentNote
		if cached, ok := m.incidentCache[inc.ID]; ok && cached.notesLoaded {
			notes = cached.notes
		}

		if len(notes) > 0 {
			parts = append(parts, fmt.Sprintf("Notes: %d", len(notes)))
			for j, n := range notes {
				if j >= 5 {
					break
				}
				content := n.Content
				if r := []rune(content); len(r) > 300 {
					content = string(r[:300]) + "..."
				}
				parts = append(parts, fmt.Sprintf("  - %s", content))
			}
		}
	}

	if len(m.incidentList) > 0 {
		parts = append(parts, fmt.Sprintf("\nFull incident queue (%d incidents):", len(m.incidentList)))
		parts = append(parts, buildIncidentSummary(m.incidentList))
	}

	return strings.Join(parts, "\n")
}

func findIncidentByID(incidents []pagerduty.Incident, id string) *pagerduty.Incident {
	for i := range incidents {
		if incidents[i].ID == id {
			return &incidents[i]
		}
	}
	return nil
}

func buildWatcherContext(m *model) string {
	var parts []string

	if m.selectedIncident != nil {
		inc := m.selectedIncident
		parts = append(parts, fmt.Sprintf("Selected incident: %s (%s)", inc.Title, inc.ID))
		parts = append(parts, fmt.Sprintf("Service: %s", inc.Service.Summary))
		parts = append(parts, fmt.Sprintf("Status: %s, Urgency: %s", inc.Status, inc.Urgency))

		// Pull alerts from cache (populated by OCM enrichment pipeline)
		var alerts []pagerduty.IncidentAlert
		if cached, ok := m.incidentCache[inc.ID]; ok && cached.alertsLoaded {
			alerts = cached.alerts
		} else if len(m.selectedIncidentAlerts) > 0 {
			alerts = m.selectedIncidentAlerts
		}

		for _, alert := range alerts {
			if details, ok := alert.Body["details"].(map[string]interface{}); ok {
				if name, ok := details["alert_name"].(string); ok {
					parts = append(parts, fmt.Sprintf("Alert: %s", name))
				}
				if sopURL, ok := details["firing"].(string); ok && sopURL != "" {
					parts = append(parts, fmt.Sprintf("SOP: %s", sopURL))
				}
				if cluster, ok := details["cluster_id"].(string); ok {
					parts = append(parts, fmt.Sprintf("Cluster: %s", cluster))
					parts = append(parts, buildClusterContext(m, cluster)...)
				}
			}
		}

		// Pull notes from cache
		var notes []pagerduty.IncidentNote
		if cached, ok := m.incidentCache[inc.ID]; ok && cached.notesLoaded {
			notes = cached.notes
		} else if len(m.selectedIncidentNotes) > 0 {
			notes = m.selectedIncidentNotes
		}

		if len(notes) > 0 {
			parts = append(parts, fmt.Sprintf("Notes: %d", len(notes)))
			for i, n := range notes {
				if i >= 5 {
					break
				}
				content := n.Content
				if r := []rune(content); len(r) > 300 {
					content = string(r[:300]) + "..."
				}
				parts = append(parts, fmt.Sprintf("  - %s", content))
			}
		}
	}

	if len(m.incidentList) > 0 {
		parts = append(parts, fmt.Sprintf("\nFull incident queue (%d incidents):", len(m.incidentList)))
		parts = append(parts, buildIncidentSummary(m.incidentList))
	}

	return strings.Join(parts, "\n")
}

func buildClusterContext(m *model, clusterID string) []string {
	var parts []string

	if info, ok := m.clusterCache[clusterID]; ok {
		parts = append(parts, fmt.Sprintf("Cluster name: %s", info.DisplayName))
		parts = append(parts, fmt.Sprintf("State: %s, Region: %s, Provider: %s, Version: %s",
			info.State, info.Region, info.CloudProvider, info.Version))
	}

	if logs, ok := m.serviceLogCache[clusterID]; ok && len(logs) > 0 {
		parts = append(parts, fmt.Sprintf("Recent service logs: %d", len(logs)))
		for i, sl := range logs {
			if i >= 5 {
				break
			}
			parts = append(parts, fmt.Sprintf("  - [%s] %s: %s", sl.Severity, sl.ServiceName, sl.Summary))
		}
	}

	if reasons, ok := m.limitedSupportCache[clusterID]; ok && len(reasons) > 0 {
		parts = append(parts, fmt.Sprintf("Limited support reasons: %d", len(reasons)))
		for _, r := range reasons {
			parts = append(parts, fmt.Sprintf("  - %s", r.Summary))
		}
	}

	return parts
}

// stripControl removes ANSI escape sequences (CSI, OSC, ESC) and C0
// control characters from s, preserving only \n and \t. Applied at
// the buffer-append boundary so all AI output is sanitized in one
// place, preventing terminal injection from attacker-influenced data.
func stripControl(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch == 0x1b {
			i++
			if i >= len(s) {
				break
			}
			switch s[i] {
			case '[':
				// CSI sequence: ESC [ ... final byte (0x40-0x7E)
				i++
				for i < len(s) && (s[i] < 0x40 || (s[i] > 0x7E && s[i] < 0x80)) {
					i++
				}
				if i < len(s) {
					i++ // skip final byte
				}
			case ']':
				// OSC sequence: ESC ] ... (terminated by BEL or ST)
				i++
				for i < len(s) {
					if s[i] == 0x07 {
						i++
						break
					}
					if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
			default:
				// Other ESC sequences (e.g., ESC c, ESC D): skip one char after ESC
				i++
			}
			continue
		}
		// Preserve \n and \t, strip all other C0 controls and DEL
		if ch == '\n' || ch == '\t' {
			b.WriteByte(ch)
			i++
			continue
		}
		if ch < 0x20 || ch == 0x7f {
			i++
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func prefixLines(marker string, text string) string {
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result = append(result, "")
		} else {
			result = append(result, marker+line)
		}
	}
	return strings.Join(result, "\n")
}

// prefixMessage prepends the marker exactly once at the start of the block.
// Unlike prefixLines, continuation lines within the same --- block get NO
// marker — the user sees one identifier per watcher/agent response.
func prefixMessage(marker string, text string) string {
	if text == "" {
		return ""
	}
	return marker + text
}
