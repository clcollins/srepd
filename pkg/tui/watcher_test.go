package tui

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/ai/tools"
	"github.com/clcollins/srepd/pkg/delta"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestStripControl(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "hello world", "hello world"},
		{"preserves newline", "line1\nline2", "line1\nline2"},
		{"preserves tab", "col1\tcol2", "col1\tcol2"},
		{"strips ANSI color", "hello \x1b[31mred\x1b[0m world", "hello red world"},
		{"strips ANSI cursor move", "text\x1b[2Amoved", "textmoved"},
		{"strips OSC-52 clipboard", "text\x1b]52;c;SGVsbG8=\x07done", "textdone"},
		{"strips OSC with ST terminator", "text\x1b]0;title\x1b\\done", "textdone"},
		{"strips BEL", "hello\x07world", "helloworld"},
		{"strips C0 NUL", "hello\x00world", "helloworld"},
		{"strips C0 BS", "hello\x08world", "helloworld"},
		{"strips C0 CR", "hello\rworld", "helloworld"},
		{"strips mixed controls", "\x1b[31m\x07alert\x1b[0m\x00", "alert"},
		{"empty string", "", ""},
		{"only controls", "\x1b[31m\x07\x00", ""},
		{"unicode preserved", "hello 世界 🤖", "hello 世界 🤖"},
		{"CSI with params", "\x1b[38;5;196mcolored\x1b[0m", "colored"},
		// Truncated/malformed sequences — must not panic (R1)
		{"truncated CSI params", "\x1b[999", ""},
		{"truncated CSI bare", "\x1b[", ""},
		{"lone ESC at end", "\x1b", ""},
		{"truncated OSC bare", "\x1b]", ""},
		{"truncated OSC-8 link", "\x1b]8;;", ""},
		{"lone ESC mid-string", "hello\x1bworld", "helloorld"},
		{"truncated CSI after text", "ok\x1b[999", "ok"},
		{"truncated OSC after text", "ok\x1b]title", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, stripControl(tt.input))
		})
	}
}

func TestWatcherBuffer_StripsControlOnAppend(t *testing.T) {
	buf := newWatcherBuffer(5)
	buf.Append("safe \x1b[31mred\x1b[0m text")
	assert.Equal(t, "safe red text", buf.Content())
}

func TestWatcherBuffer_StripsControlOnSetLast(t *testing.T) {
	buf := newWatcherBuffer(5)
	buf.Append("initial")
	buf.SetLast("updated \x1b]52;c;SGVsbG8=\x07 content")
	assert.Equal(t, "updated  content", buf.Content())
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

// D2 test: an observation for alert A, with a DIFFERENT incident selected,
// must produce context naming A — and would FAIL if it reverted to m.selectedIncident.
func TestBuildObservationContext_ScopedToTriggeringIncident(t *testing.T) {
	m := createTestModel()

	incidentA := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-A"},
		Title:     "Alert on cluster-xyz",
		Status:    "triggered",
		Urgency:   "high",
		Service:   pagerduty.APIObject{Summary: "svc-alpha"},
	}
	incidentB := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-B"},
		Title:     "Unrelated alert",
		Status:    "acknowledged",
		Urgency:   "low",
		Service:   pagerduty.APIObject{Summary: "svc-beta"},
	}
	m.incidentList = []pagerduty.Incident{incidentA, incidentB}

	// User has selected incident B, but the observation is about A
	m.selectedIncident = &incidentB

	obs := watcherObservation{
		Summary:     "Service storm on svc-alpha",
		IncidentIDs: []string{"INC-A"},
	}

	ctx := buildObservationContext(&m, obs)

	// Must contain triggering incident A's details
	assert.Contains(t, ctx, "INC-A")
	assert.Contains(t, ctx, "Alert on cluster-xyz")
	assert.Contains(t, ctx, "svc-alpha")
	assert.Contains(t, ctx, "triggered")

	// The triggering section must not label incident B as "Triggering" or "Related"
	assert.NotContains(t, ctx, "Triggering incident: Unrelated alert")
	assert.NotContains(t, ctx, "Related incident: Unrelated alert")
	// Queue summary includes all incidents (expected — design choice c)
	assert.Contains(t, ctx, "Full incident queue")
}

func TestBuildObservationContext_MultipleTriggering(t *testing.T) {
	m := createTestModel()
	m.incidentList = []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "P1"}, Title: "First", Service: pagerduty.APIObject{Summary: "svc-a"}, Status: "triggered", Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "P2"}, Title: "Second", Service: pagerduty.APIObject{Summary: "svc-a"}, Status: "triggered", Urgency: "high"},
	}

	obs := watcherObservation{
		Summary:     "Service storm",
		IncidentIDs: []string{"P1", "P2"},
	}

	ctx := buildObservationContext(&m, obs)
	assert.Contains(t, ctx, "P1")
	assert.Contains(t, ctx, "P2")
	assert.Contains(t, ctx, "Triggering incident")
	assert.Contains(t, ctx, "Related incident")
}

func TestBuildObservationContext_EmptyIncidentIDs(t *testing.T) {
	m := createTestModel()
	m.incidentList = []pagerduty.Incident{
		makeIncident("P1", "svc-a", "high"),
	}
	obs := watcherObservation{Summary: "Something happened"}

	ctx := buildObservationContext(&m, obs)
	assert.Contains(t, ctx, "P1", "queue summary still included")
}

// M1: Headline integration test exercising BOTH directions of the delta gate
// in runDetectors. FAILS if the suppression gate is stubbed with `if false &&`.
func TestRunDetectors_DeltaGateBothDirections(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.incidentClusterMap = make(map[string][]string)
	m.watcherDedup = newWatcherDedup(0) // disable cooldown to isolate delta gate

	// 3 incidents on the same service → triggers service storm detector.
	// Use low urgency to avoid triggering urgency-shift detector.
	m.incidentList = []pagerduty.Incident{
		makeIncident("P1", "svc-x", "low"),
		makeIncident("P2", "svc-x", "low"),
		makeIncident("P3", "svc-x", "low"),
	}

	// --- Direction 1: changes present → detectors MUST fire ---
	changes1 := m.computeAndStoreDeltas()
	require.NotEmpty(t, changes1, "first poll must produce IncidentNew changes")

	beforeLen := m.watcherBuffer.Len()
	cmds1 := m.runDetectors(changes1)
	// With no AI provider, observations go to the buffer
	assert.True(t, m.watcherBuffer.Len() > beforeLen || len(cmds1) > 0,
		"non-empty changes must trigger detector observations")
	firstPollBufLen := m.watcherBuffer.Len()

	// --- Direction 2: no changes → detectors MUST NOT fire ---
	changes2 := m.computeAndStoreDeltas()
	assert.Empty(t, changes2, "identical second poll must produce zero changes")

	cmds2 := m.runDetectors(changes2)
	assert.Empty(t, cmds2, "runDetectors must return nil when changes are empty")
	assert.Equal(t, firstPollBufLen, m.watcherBuffer.Len(),
		"buffer must not grow when delta gate suppresses")

	// --- Changed data re-enables detectors ---
	m.incidentList[0] = makeIncident("P1", "svc-x", "high") // urgency change
	changes3 := m.computeAndStoreDeltas()
	require.NotEmpty(t, changes3, "changed data must produce changes")

	cmds3 := m.runDetectors(changes3)
	assert.True(t, m.watcherBuffer.Len() > firstPollBufLen || len(cmds3) > 0,
		"changed data must re-enable detector observations")
}

// M3: the incidentList < 2 guard must suppress detectors for a single incident.
// FAILS if the guard is changed to `< 0`.
func TestRunDetectors_SingleIncidentNoInvestigation(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.incidentClusterMap = make(map[string][]string)

	m.incidentList = []pagerduty.Incident{
		makeIncident("P1", "svc-a", "high"),
	}

	changes := m.computeAndStoreDeltas()
	require.NotEmpty(t, changes, "first-sighting must produce changes")

	cmds := m.runDetectors(changes)
	assert.Empty(t, cmds, "single incident must not trigger investigation")
	assert.Equal(t, 0, m.watcherBuffer.Len(),
		"single incident must not produce buffer entries")
}

// M2: cache loading between polls must not produce false NoteAdded/AlertAdded.
// Poll 1 with unloaded cache → Poll 2 with loaded cache (same data) → zero
// note/alert changes. This test FAILS if toSnapshots treats "not loaded" as 0.
func TestToSnapshots_UnloadedCacheSuppressesFalseChanges(t *testing.T) {
	incidents := []pagerduty.Incident{
		makeIncident("P1", "svc-a", "high"),
	}

	// Poll 1: cache entry exists but notes/alerts not yet loaded
	cache1 := map[string]*cachedIncidentData{
		"P1": {notesLoaded: false, alertsLoaded: false},
	}
	snap1 := toSnapshots(incidents, cache1)

	// Between polls: lazy enrichment loads 5 notes and 3 alerts
	cache2 := map[string]*cachedIncidentData{
		"P1": {
			notesLoaded:  true,
			notes:        make([]pagerduty.IncidentNote, 5),
			alertsLoaded: true,
			alerts:       make([]pagerduty.IncidentAlert, 3),
		},
	}
	snap2 := toSnapshots(incidents, cache2)

	changes := delta.Diff(snap1, snap2)
	for _, c := range changes {
		assert.NotEqual(t, delta.NoteAdded, c.Kind,
			"cache loading must not produce false NoteAdded")
		assert.NotEqual(t, delta.AlertAdded, c.Kind,
			"cache loading must not produce false AlertAdded")
	}
}

// M2 counterpart: a genuine note addition AFTER cache load must still be detected.
func TestToSnapshots_GenuineNoteAdditionAfterCacheLoad(t *testing.T) {
	incidents := []pagerduty.Incident{
		makeIncident("P1", "svc-a", "high"),
	}

	cache1 := map[string]*cachedIncidentData{
		"P1": {notesLoaded: true, notes: make([]pagerduty.IncidentNote, 2)},
	}
	snap1 := toSnapshots(incidents, cache1)

	cache2 := map[string]*cachedIncidentData{
		"P1": {notesLoaded: true, notes: make([]pagerduty.IncidentNote, 3)},
	}
	snap2 := toSnapshots(incidents, cache2)

	changes := delta.Diff(snap1, snap2)
	found := false
	for _, c := range changes {
		if c.Kind == delta.NoteAdded {
			found = true
		}
	}
	assert.True(t, found, "genuine note addition must be detected")
}

func TestBuildAskFromVerdict_UsesOriginatingIncident(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	m := createTestModel()
	m.config = &pd.Config{Client: mock}

	incidentA := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-ORIGIN"},
		Title:     "Originating Alert",
	}
	incidentB := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC-SELECTED"},
		Title:     "UI Selected Alert",
	}
	m.incidentList = []pagerduty.Incident{incidentA, incidentB}
	m.selectedIncident = &incidentB

	verdict := tools.Verdict{
		Tier:    tools.TierActionable,
		Summary: "Post note",
		Action:  "Investigation note content",
	}

	ask := m.buildAskFromVerdict(verdict, []string{"INC-ORIGIN"})

	assert.Equal(t, "INC-ORIGIN", ask.IncidentID,
		"must use originating incident, not m.selectedIncident")
	assert.Equal(t, "Originating Alert", ask.IncidentTitle)
}
