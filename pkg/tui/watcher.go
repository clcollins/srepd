package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
