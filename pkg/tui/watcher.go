package tui

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
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

func (m *model) runDetectors() {
	if len(m.incidentList) < 2 {
		return
	}

	observations := detectAll(m.incidentList, m.incidentClusterMap)

	for _, obs := range observations {
		if m.watcherDedup.IsNew(obs.Summary) {
			log.Debug("watcher.runDetectors", "observation", obs.Summary)
			m.watcherBuffer.Append(prefixLines(m.watcherMarker, obs.Summary))
			m.updateWatcherViewport()
		}
	}
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
