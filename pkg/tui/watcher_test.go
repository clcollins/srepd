package tui

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
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

func makeIncident(id, service, urgency string) pagerduty.Incident {
	return pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: id},
		Service:   pagerduty.APIObject{Summary: service},
		Urgency:   urgency,
		Status:    "triggered",
	}
}

func TestDetectServiceStorm(t *testing.T) {
	t.Run("detects 3+ incidents on same service", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "osd-cluster-a", "high"),
			makeIncident("P2", "osd-cluster-a", "high"),
			makeIncident("P3", "osd-cluster-a", "low"),
		}
		obs := detectServiceStorm(incidents)
		assert.Len(t, obs, 1)
		assert.Contains(t, obs[0].Summary, "osd-cluster-a")
		assert.Contains(t, obs[0].Summary, "3")
	})

	t.Run("no storm with 2 incidents", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "svc-a", "high"),
			makeIncident("P2", "svc-a", "high"),
		}
		obs := detectServiceStorm(incidents)
		assert.Empty(t, obs)
	})

	t.Run("multiple services each below threshold", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "svc-a", "high"),
			makeIncident("P2", "svc-b", "high"),
			makeIncident("P3", "svc-c", "high"),
		}
		obs := detectServiceStorm(incidents)
		assert.Empty(t, obs)
	})
}

func TestDetectClusterStorm(t *testing.T) {
	t.Run("detects 2+ incidents on same cluster", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "svc-a", "high"),
			makeIncident("P2", "svc-b", "high"),
		}
		clusterMap := map[string][]string{
			"P1": {"cluster-abc"},
			"P2": {"cluster-abc"},
		}
		obs := detectClusterStorm(incidents, clusterMap)
		assert.Len(t, obs, 1)
		assert.Contains(t, obs[0].Summary, "cluster-abc")
	})

	t.Run("no storm with different clusters", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "svc-a", "high"),
			makeIncident("P2", "svc-b", "high"),
		}
		clusterMap := map[string][]string{
			"P1": {"cluster-abc"},
			"P2": {"cluster-def"},
		}
		obs := detectClusterStorm(incidents, clusterMap)
		assert.Empty(t, obs)
	})

	t.Run("nil cluster map returns nil", func(t *testing.T) {
		incidents := []pagerduty.Incident{makeIncident("P1", "svc", "high")}
		obs := detectClusterStorm(incidents, nil)
		assert.Nil(t, obs)
	})
}

func TestDetectUrgencyShift(t *testing.T) {
	t.Run("detects 3+ high urgency incidents", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "svc-a", "high"),
			makeIncident("P2", "svc-b", "high"),
			makeIncident("P3", "svc-c", "high"),
		}
		obs := detectUrgencyShift(incidents)
		assert.Len(t, obs, 1)
		assert.Contains(t, obs[0].Summary, "3/3")
	})

	t.Run("no alert with fewer than 3 high", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "svc-a", "high"),
			makeIncident("P2", "svc-b", "low"),
		}
		obs := detectUrgencyShift(incidents)
		assert.Nil(t, obs)
	})
}

func TestWatcherDedup(t *testing.T) {
	t.Run("first observation is new", func(t *testing.T) {
		d := newWatcherDedup(5 * time.Minute)
		assert.True(t, d.IsNew("something happened"))
	})

	t.Run("duplicate within cooldown is not new", func(t *testing.T) {
		d := newWatcherDedup(5 * time.Minute)
		d.IsNew("something happened")
		assert.False(t, d.IsNew("something happened"))
	})

	t.Run("different observation is new", func(t *testing.T) {
		d := newWatcherDedup(5 * time.Minute)
		d.IsNew("first thing")
		assert.True(t, d.IsNew("second thing"))
	})
}

func TestDetectAll(t *testing.T) {
	t.Run("returns combined observations from all detectors", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			makeIncident("P1", "svc-a", "high"),
			makeIncident("P2", "svc-a", "high"),
			makeIncident("P3", "svc-a", "high"),
		}
		obs := detectAll(incidents, nil)
		assert.GreaterOrEqual(t, len(obs), 2)
	})
}

func TestWatcherBuffer_SetLast(t *testing.T) {
	t.Run("replaces last entry", func(t *testing.T) {
		buf := newWatcherBuffer(5)
		buf.Append("first")
		buf.Append("second")
		buf.SetLast("replaced")

		assert.Equal(t, 2, buf.Len())
		assert.Contains(t, buf.Content(), "replaced")
		assert.NotContains(t, buf.Content(), "second")
	})

	t.Run("appends when empty", func(t *testing.T) {
		buf := newWatcherBuffer(5)
		buf.SetLast("only")

		assert.Equal(t, 1, buf.Len())
		assert.Equal(t, "only", buf.Content())
	})
}

func TestBuildIncidentSummary(t *testing.T) {
	incidents := []pagerduty.Incident{
		makeIncident("P1", "svc-a", "high"),
		makeIncident("P2", "svc-b", "low"),
	}

	summary := buildIncidentSummary(incidents)

	assert.Contains(t, summary, "P1")
	assert.Contains(t, summary, "svc-a")
	assert.Contains(t, summary, "high")
	assert.Contains(t, summary, "P2")
	assert.Contains(t, summary, "svc-b")
	assert.Contains(t, summary, "low")
}

func TestIsWatcherCommand(t *testing.T) {
	assert.True(t, isWatcherCommand(":watcher what happened"))
	assert.True(t, isWatcherCommand(":watcher"))
	assert.True(t, isWatcherCommand("  :watcher query"))
	assert.False(t, isWatcherCommand("watcher"))
	assert.False(t, isWatcherCommand(":agent query"))
	assert.False(t, isWatcherCommand(""))
}

func TestParseWatcherQuery(t *testing.T) {
	assert.Equal(t, "what happened", parseWatcherQuery(":watcher what happened"))
	assert.Equal(t, "", parseWatcherQuery(":watcher"))
	assert.Equal(t, "multi word query", parseWatcherQuery(":watcher multi word query"))
}

func TestSplitKeepingNewlines(t *testing.T) {
	t.Run("preserves newlines as tokens", func(t *testing.T) {
		tokens := splitKeepingNewlines("hello world\nfoo bar")
		assert.Equal(t, []string{"hello", "world", "\n", "foo", "bar"}, tokens)
	})

	t.Run("handles blank lines", func(t *testing.T) {
		tokens := splitKeepingNewlines("hello\n\nworld")
		assert.Equal(t, []string{"hello", "\n", "\n", "world"}, tokens)
	})

	t.Run("single line", func(t *testing.T) {
		tokens := splitKeepingNewlines("hello world")
		assert.Equal(t, []string{"hello", "world"}, tokens)
	})

	t.Run("empty string", func(t *testing.T) {
		tokens := splitKeepingNewlines("")
		assert.Empty(t, tokens)
	})
}

func TestBuildWatcherContext_NoIncident(t *testing.T) {
	m := createTestModel()
	m.incidentList = []pagerduty.Incident{
		makeIncident("P1", "svc-a", "high"),
	}

	ctx := buildWatcherContext(&m)

	assert.Contains(t, ctx, "P1")
	assert.Contains(t, ctx, "svc-a")
}

func TestBuildWatcherContext_WithIncident(t *testing.T) {
	m := createTestModel()
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P123"},
		Title:     "Test Alert",
		Status:    "triggered",
		Urgency:   "high",
		Service:   pagerduty.APIObject{Summary: "test-service"},
	}
	m.incidentList = []pagerduty.Incident{*m.selectedIncident}

	ctx := buildWatcherContext(&m)

	assert.Contains(t, ctx, "Test Alert")
	assert.Contains(t, ctx, "P123")
	assert.Contains(t, ctx, "test-service")
	assert.Contains(t, ctx, "triggered")
	assert.Contains(t, ctx, "high")
}
