package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWatcherBuffer_Append(t *testing.T) {
	buf := newWatcherBuffer(5)
	buf.Append("first")
	buf.Append("second")

	assert.Equal(t, 2, buf.Len())
	assert.Contains(t, buf.Content(), "first")
	assert.Contains(t, buf.Content(), "second")
}

func TestWatcherBuffer_ContentJoinsWithSeparator(t *testing.T) {
	buf := newWatcherBuffer(5)
	buf.Append("one")
	buf.Append("two")
	buf.Append("three")

	assert.Equal(t, "one\n---\ntwo\n---\nthree", buf.Content())
}

func TestWatcherBuffer_CapacityOverflow(t *testing.T) {
	buf := newWatcherBuffer(3)
	buf.Append("a")
	buf.Append("b")
	buf.Append("c")
	buf.Append("d")

	assert.Equal(t, 3, buf.Len())
	assert.NotContains(t, buf.Content(), "a")
	assert.Contains(t, buf.Content(), "b")
	assert.Contains(t, buf.Content(), "d")
}

func TestWatcherBuffer_Clear(t *testing.T) {
	buf := newWatcherBuffer(5)
	buf.Append("something")
	buf.Clear()

	assert.Equal(t, 0, buf.Len())
	assert.Equal(t, "", buf.Content())
}

func TestWatcherBuffer_SingleEntry(t *testing.T) {
	buf := newWatcherBuffer(5)
	buf.Append("only entry")

	assert.Equal(t, "only entry", buf.Content())
}

func TestWatcherBuffer_EmptyContent(t *testing.T) {
	buf := newWatcherBuffer(5)
	assert.Equal(t, "", buf.Content())
	assert.Equal(t, 0, buf.Len())
}

func TestPrefixLines(t *testing.T) {
	tests := []struct {
		name     string
		marker   string
		text     string
		expected string
	}{
		{
			name:     "single line",
			marker:   "🤖 ",
			text:     "hello",
			expected: "🤖 hello",
		},
		{
			name:     "multi line",
			marker:   "🤖 ",
			text:     "line one\nline two\nline three",
			expected: "🤖 line one\n🤖 line two\n🤖 line three",
		},
		{
			name:     "blank lines preserved without marker",
			marker:   "📡 ",
			text:     "first\n\nsecond\n\nthird",
			expected: "📡 first\n\n📡 second\n\n📡 third",
		},
		{
			name:     "whitespace-only lines treated as blank",
			marker:   "☻ ",
			text:     "hello\n   \nworld",
			expected: "☻ hello\n\n☻ world",
		},
		{
			name:     "empty string",
			marker:   "🤖 ",
			text:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prefixLines(tt.marker, tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveMarkers_Emoji(t *testing.T) {
	mk := resolveMarkers(true)
	assert.Equal(t, emojiFlagMarker, mk.flag)
	assert.Equal(t, emojiWatcherMarker, mk.watcher)
	assert.Equal(t, emojiAgentMarker, mk.agent)
}

func TestResolveMarkers_NoEmoji(t *testing.T) {
	mk := resolveMarkers(false)
	assert.Equal(t, noEmojiFlagMarker, mk.flag)
	assert.Equal(t, noEmojiWatcherMarker, mk.watcher)
	assert.Equal(t, noEmojiAgentMarker, mk.agent)
}
