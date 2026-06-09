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
	if len(b.entries) >= b.capacity {
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, entry)
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
}

type watcherObservation struct {
	Summary string
}

type watcherDedup struct {
	seen    map[string]time.Time
	cooldown time.Duration
}

func newWatcherDedup(cooldown time.Duration) *watcherDedup {
	return &watcherDedup{
		seen:    make(map[string]time.Time),
		cooldown: cooldown,
	}
}

func (d *watcherDedup) IsNew(observation string) bool {
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(observation)))
	if last, ok := d.seen[h]; ok && time.Since(last) < d.cooldown {
		return false
	}
	d.seen[h] = time.Now()
	return true
}

func (m *model) runDetectors() []tea.Cmd {
	if len(m.incidentList) < 2 {
		return nil
	}

	observations := detectAll(m.incidentList, m.incidentClusterMap)

	var cmds []tea.Cmd
	added := false
	for _, obs := range observations {
		if !m.watcherDedup.IsNew(obs.Summary) {
			continue
		}

		log.Debug("watcher.runDetectors", "observation", obs.Summary)

		if m.aiProvider != nil && m.aiHealthy && !m.watcherAnalyzing {
			m.watcherAnalyzing = true
			summary := buildIncidentSummary(m.incidentList)
			cmds = append(cmds, watcherSynthesizeCmd(m.aiProvider, obs.Summary, summary))
		} else {
			m.watcherBuffer.Append(prefixLines(m.watcherMarker, obs.Summary))
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
	serviceCounts := make(map[string]int)
	for _, inc := range incidents {
		serviceCounts[inc.Service.Summary]++
	}

	var observations []watcherObservation
	for svc, count := range serviceCounts {
		if count >= 3 {
			observations = append(observations, watcherObservation{
				Summary: fmt.Sprintf("Service storm: %d incidents on %s", count, svc),
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
	for _, inc := range incidents {
		for _, clusterID := range clusterMap[inc.ID] {
			clusterCounts[clusterID]++
		}
	}

	var observations []watcherObservation
	for cluster, count := range clusterCounts {
		if count >= 2 {
			observations = append(observations, watcherObservation{
				Summary: fmt.Sprintf("Cluster storm: %d incidents on cluster %s", count, cluster),
			})
		}
	}
	return observations
}

func detectUrgencyShift(incidents []pagerduty.Incident) []watcherObservation {
	highCount := 0
	for _, inc := range incidents {
		if inc.Urgency == "high" {
			highCount++
		}
	}

	if highCount >= 3 {
		return []watcherObservation{{
			Summary: fmt.Sprintf("High urgency cluster: %d/%d incidents are high urgency", highCount, len(incidents)),
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
				if len(content) > 300 {
					content = content[:300] + "..."
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
