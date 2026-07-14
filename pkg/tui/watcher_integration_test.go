package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/stretchr/testify/assert"
)

func TestWatcherToggle(t *testing.T) {
	t.Run("w key toggles watcherExpanded", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}
		m.table.Focus()

		assert.False(t, m.watcherExpanded)

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
		result, _ := m.Update(msg)
		updated := result.(model)

		assert.True(t, updated.watcherExpanded)
	})
}

func TestWatcherPromptMsg_NoProvider(t *testing.T) {
	m := createTestModel()
	m.aiProvider = nil
	m.table.Focus()

	result, cmd := m.Update(watcherPromptMsg{prompt: "test"})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.NotNil(t, cmd, "should return flash notification")
}

func TestWatcherPromptMsg_ProviderOffline(t *testing.T) {
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("test")
	m.aiHealthy = false
	m.table.Focus()

	result, cmd := m.Update(watcherPromptMsg{prompt: "test"})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.NotNil(t, cmd, "should return flash notification")
}

func TestWatcherPromptMsg_Success(t *testing.T) {
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("test")
	m.aiHealthy = true
	m.table.Focus()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	result, cmd := m.Update(watcherPromptMsg{prompt: "what happened"})
	updated := result.(model)

	assert.True(t, updated.watcherAnalyzing)
	assert.True(t, updated.apiInProgress)
	assert.True(t, updated.watcherExpanded)
	assert.NotNil(t, cmd)
}

func TestWatcherResponseMsg_Success(t *testing.T) {
	m := createTestModel()
	m.watcherAnalyzing = true
	m.apiInProgress = true
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	result, cmd := m.Update(watcherResponseMsg{response: "analysis result"})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.False(t, updated.apiInProgress)
	assert.True(t, updated.watcherExpanded)
	assert.NotNil(t, cmd, "should return typewriter tick")
	assert.NotNil(t, updated.typewriter)
}

func TestWatcherResponseMsg_Error(t *testing.T) {
	m := createTestModel()
	m.watcherAnalyzing = true
	m.apiInProgress = true

	result, cmd := m.Update(watcherResponseMsg{err: assert.AnError})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.False(t, updated.apiInProgress)
	assert.NotNil(t, cmd, "should return flash notification")
}

func TestWatcherSynthesisMsg_Success(t *testing.T) {
	m := createTestModel()
	m.watcherAnalyzing = true
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	result, cmd := m.Update(watcherSynthesisMsg{
		observation: "service storm detected",
		response:    "Multiple incidents suggest a platform-wide issue.",
	})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.True(t, updated.watcherExpanded)
	assert.NotNil(t, cmd, "should return typewriter tick")
}

func TestWatcherSynthesisMsg_Error(t *testing.T) {
	m := createTestModel()
	m.watcherAnalyzing = true
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	result, _ := m.Update(watcherSynthesisMsg{
		observation: "service storm detected",
		err:         assert.AnError,
	})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.Equal(t, 1, updated.watcherBuffer.Len(), "raw observation should be appended on error")
}

func TestTypewriterTickMsg(t *testing.T) {
	m := createTestModel()
	m.typewriter = &typewriterState{
		words:  []string{"hello", "world", "foo"},
		marker: "🤖 ",
	}
	m.watcherBuffer.Append("")

	result, cmd := m.Update(typewriterTickMsg{})
	updated := result.(model)

	assert.Contains(t, updated.watcherBuffer.Content(), "hello")
	if updated.typewriter == nil {
		assert.Nil(t, cmd, "no more ticks when done")
	} else {
		assert.NotNil(t, cmd, "more ticks when words remain")
	}
}

func TestMouseMsg_WatcherExpanded(t *testing.T) {
	m := createTestModel()
	m.watcherExpanded = true
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}
	m.recomputeLayout()
	m.watcherViewport.Width = m.layout.WatcherWidth
	m.watcherViewport.Height = m.layout.WatcherHeight
	m.watcherViewport.SetContent("some content\nto scroll\nthrough")

	mouseMsg := tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	result, _ := m.Update(mouseMsg)
	_ = result.(model)
}

func TestMouseMsg_WatcherCollapsed(t *testing.T) {
	m := createTestModel()
	m.watcherExpanded = false

	mouseMsg := tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	_, cmd := m.Update(mouseMsg)

	assert.Nil(t, cmd)
}

func TestInputMode_WatcherCommand(t *testing.T) {
	m := createTestModel()
	m.input = newTextInput()
	m.input.Focus()
	m.input.SetValue(":watcher what happened")
	m.table.Focus()

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := switchInputFocusMode(m, enterMsg)
	updated := result.(model)

	assert.False(t, updated.input.Focused())
	assert.NotNil(t, cmd)

	msg := cmd()
	promptMsg, ok := msg.(watcherPromptMsg)
	assert.True(t, ok)
	assert.Equal(t, "what happened", promptMsg.prompt)
}

func TestInputMode_WatcherCommandEmpty(t *testing.T) {
	m := createTestModel()
	m.input = newTextInput()
	m.input.Focus()
	m.input.SetValue(":watcher")

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := switchInputFocusMode(m, enterMsg)
	updated := result.(model)

	assert.Contains(t, updated.status, "usage")
	assert.Nil(t, cmd)
}

func TestRenderWatcherPane_Collapsed(t *testing.T) {
	m := createTestModel()
	m.watcherExpanded = false

	result := m.renderWatcherPane()
	assert.Equal(t, "", result)
}

func TestRenderWatcherPane_Expanded(t *testing.T) {
	m := createTestModel()
	m.watcherExpanded = true
	m.watcherViewport.Width = 80
	m.watcherViewport.Height = 10

	result := m.renderWatcherPane()
	assert.NotEmpty(t, result)
}

func TestRenderWatcherStatus_NoProvider(t *testing.T) {
	m := createTestModel()
	m.aiProvider = nil

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "[AI Watcher]")
	assert.Contains(t, status, "idle")
}

func TestRenderWatcherStatus_Healthy(t *testing.T) {
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("ollama")
	m.aiHealthy = true

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "ollama")
	assert.Contains(t, status, "healthy")
}

func TestRenderWatcherStatus_Offline(t *testing.T) {
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("ollama")
	m.aiHealthy = false

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "offline")
}

func TestRenderFooter_WatcherCollapsed(t *testing.T) {
	m := createTestModel()
	m.watcherExpanded = false

	footer := m.renderFooter()
	assert.Contains(t, footer, "Watching")
	assert.NotContains(t, footer, "[AI Watcher]")
}

func TestRenderFooter_WatcherExpanded(t *testing.T) {
	m := createTestModel()
	m.watcherExpanded = true
	m.aiProvider = ai.NewMockProvider("ollama")
	m.aiHealthy = true
	windowSize = tea.WindowSizeMsg{Width: 120, Height: 60}

	footer := m.renderFooter()
	assert.Contains(t, footer, "Watching")
}

func TestBuildClusterContext(t *testing.T) {
	m := createTestModel()
	m.clusterCache = map[string]*ocm.ClusterInfo{
		"cluster-abc": {
			DisplayName:   "test-cluster.example.org",
			State:         "ready",
			Region:        "us-east-1",
			CloudProvider: "aws",
			Version:       "4.16.5",
		},
	}

	parts := buildClusterContext(&m, "cluster-abc")

	assert.NotEmpty(t, parts)
	found := false
	for _, p := range parts {
		if assert.ObjectsAreEqual("test-cluster.example.org", p) || len(p) > 0 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestAdvanceTypewriter(t *testing.T) {
	m := createTestModel()
	m.watcherBuffer.Append("")
	m.typewriter = &typewriterState{
		words:  []string{"one", "two", "three", "four", "five"},
		marker: "📡 ",
	}

	cmd := m.advanceTypewriter()
	assert.NotNil(t, cmd, "should return tick for remaining words")
	assert.Contains(t, m.watcherBuffer.Content(), "one")

	cmd = m.advanceTypewriter()
	if m.typewriter == nil {
		assert.Nil(t, cmd, "nil when done")
	}
}

func TestAdvanceTypewriter_Nil(t *testing.T) {
	m := createTestModel()
	m.typewriter = nil

	cmd := m.advanceTypewriter()
	assert.Nil(t, cmd)
}

func TestBuildWatcherContext_WithCachedAlerts(t *testing.T) {
	m := createTestModel()
	inc := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P123"},
		Title:     "Test Alert",
		Status:    "triggered",
		Urgency:   "high",
		Service:   pagerduty.APIObject{Summary: "test-service"},
	}
	m.selectedIncident = inc
	m.incidentCache["P123"] = &cachedIncidentData{
		alertsLoaded: true,
		alerts: []pagerduty.IncidentAlert{
			{
				Body: map[string]interface{}{
					"details": map[string]interface{}{
						"alert_name": "ClusterOperatorDown",
						"cluster_id": "cluster-abc",
					},
				},
			},
		},
		notesLoaded: true,
		notes: []pagerduty.IncidentNote{
			{Content: "Investigated — worker node OOM."},
		},
	}

	ctx := buildWatcherContext(&m)

	assert.Contains(t, ctx, "ClusterOperatorDown")
	assert.Contains(t, ctx, "cluster-abc")
	assert.Contains(t, ctx, "Investigated")
}

func TestWatcherPromptMsg_StreamingProvider_UsesStreamPath(t *testing.T) {
	m := createTestModel()
	provider := ai.NewMockProvider("test")
	provider.Streaming = true
	provider.StreamTokens = []string{"a", "b", "c"}
	m.aiProvider = provider
	m.aiHealthy = true
	m.streamResponses = true
	m.table.Focus()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	// Prompt should dispatch the streaming command (not the typewriter path).
	result, cmd := m.Update(watcherPromptMsg{prompt: "go"})
	m = result.(model)
	assert.True(t, m.watcherAnalyzing)
	assert.Nil(t, m.typewriter, "streaming path must not use the typewriter")

	// Drive the batched command tree to obtain watcherStreamStartedMsg, then feed
	// each subsequent message back through Update, accumulating chunks in the buffer.
	var started watcherStreamStartedMsg
	drainCmd(t, cmd, func(msg tea.Msg) {
		if s, ok := msg.(watcherStreamStartedMsg); ok {
			started = s
		}
	})
	assert.NotNil(t, started.ch, "should have started a stream")

	// Deliver started, then drain chunks/done through Update.
	result, next := m.Update(started)
	m = result.(model)
	done := false
	for i := 0; i < 10 && !done && next != nil; i++ {
		msg := next()
		result, next = m.Update(msg)
		m = result.(model)
		if _, ok := msg.(watcherStreamDoneMsg); ok {
			done = true
		}
	}

	assert.True(t, done, "stream should reach done")
	assert.False(t, m.watcherAnalyzing, "analyzing cleared on done")
	assert.Contains(t, m.watcherBuffer.Content(), "abc", "buffer should contain the accumulated stream")
}

func TestWatcherPromptMsg_StreamingDisabled_FallsBackToBlocking(t *testing.T) {
	m := createTestModel()
	provider := ai.NewMockProvider("test")
	provider.Streaming = true // provider supports it...
	m.aiProvider = provider
	m.aiHealthy = true
	m.streamResponses = false // ...but the user disabled streaming
	m.table.Focus()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	result, _ := m.Update(watcherPromptMsg{prompt: "go"})
	m = result.(model)

	// Blocking path: a watcherResponseMsg (not a stream) will drive the typewriter.
	// The stream partial must remain untouched.
	assert.Equal(t, "", m.watcherStreamPartial)
}

func TestWatcherStreamChunkMsg_AccumulatesInPlace(t *testing.T) {
	m := createTestModel()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}
	m.watcherBuffer.Append(prefixLines(m.watcherMarker, ""))

	ch := make(chan streamEvent)
	result, _ := m.Update(watcherStreamChunkMsg{text: "Hello", ch: ch})
	m = result.(model)
	result, _ = m.Update(watcherStreamChunkMsg{text: " world", ch: ch})
	m = result.(model)

	assert.Equal(t, "Hello world", m.watcherStreamPartial)
	assert.Contains(t, m.watcherBuffer.Content(), "Hello world")
}

// --- aiHealthCheckMsg handler tests ---

func TestAiHealthCheckMsg_Healthy(t *testing.T) {
	t.Run("sets aiHealthy to true when healthy", func(t *testing.T) {
		m := createTestModel()
		m.aiHealthy = false

		result, cmd := m.Update(aiHealthCheckMsg{healthy: true, err: nil})
		updated := result.(model)

		assert.True(t, updated.aiHealthy, "aiHealthy should be set to true")
		assert.Nil(t, cmd, "should return nil cmd")
	})
}

func TestAiHealthCheckMsg_Unhealthy(t *testing.T) {
	t.Run("sets aiHealthy to false when unhealthy", func(t *testing.T) {
		m := createTestModel()
		m.aiHealthy = true

		result, cmd := m.Update(aiHealthCheckMsg{healthy: false, err: assert.AnError})
		updated := result.(model)

		assert.False(t, updated.aiHealthy, "aiHealthy should be set to false")
		assert.Nil(t, cmd, "should return nil cmd")
	})
}

func TestAiHealthCheckMsg_ErrorNotInspected(t *testing.T) {
	t.Run("err field is not inspected; only healthy field matters", func(t *testing.T) {
		m := createTestModel()
		m.aiHealthy = false

		// healthy=true even though err is set: the handler only reads msg.healthy
		result, cmd := m.Update(aiHealthCheckMsg{healthy: true, err: assert.AnError})
		updated := result.(model)

		assert.True(t, updated.aiHealthy, "aiHealthy should follow msg.healthy, not msg.err")
		assert.Nil(t, cmd)
	})
}

func TestAiHealthCheckMsg_UnhealthyWithNilError(t *testing.T) {
	t.Run("unhealthy with nil error still sets false", func(t *testing.T) {
		m := createTestModel()
		m.aiHealthy = true

		result, cmd := m.Update(aiHealthCheckMsg{healthy: false, err: nil})
		updated := result.(model)

		assert.False(t, updated.aiHealthy)
		assert.Nil(t, cmd)
	})
}

// --- watcherResponseMsg handler additional edge case tests ---

func TestWatcherResponseMsg_ExpandsWatcherWhenCollapsed(t *testing.T) {
	t.Run("expands watcher pane if not already expanded", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.apiInProgress = true
		m.watcherExpanded = false
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, cmd := m.Update(watcherResponseMsg{response: "test response"})
		updated := result.(model)

		assert.True(t, updated.watcherExpanded, "watcher should be expanded on response")
		assert.False(t, updated.watcherAnalyzing, "watcherAnalyzing should be cleared")
		assert.False(t, updated.apiInProgress, "apiInProgress should be cleared")
		assert.NotNil(t, cmd, "should return typewriter cmd")
	})
}

func TestWatcherResponseMsg_KeepsExpandedWhenAlreadyExpanded(t *testing.T) {
	t.Run("keeps watcher expanded if already expanded", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.apiInProgress = true
		m.watcherExpanded = true
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, _ := m.Update(watcherResponseMsg{response: "test response"})
		updated := result.(model)

		assert.True(t, updated.watcherExpanded, "watcher should remain expanded")
	})
}

func TestWatcherResponseMsg_AppendsEmptyLineToBuffer(t *testing.T) {
	t.Run("appends empty line before typewriter on success", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.apiInProgress = true
		m.watcherBuffer.Append("previous content")
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, _ := m.Update(watcherResponseMsg{response: "new analysis"})
		updated := result.(model)

		// Buffer should have: "previous content", "" (appended empty line)
		assert.True(t, updated.watcherBuffer.Len() >= 2,
			"buffer should have at least 2 entries after response")
	})
}

func TestWatcherResponseMsg_SetsStatus(t *testing.T) {
	t.Run("sets status to watcher response received on success", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.apiInProgress = true
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, _ := m.Update(watcherResponseMsg{response: "result"})
		updated := result.(model)

		assert.Contains(t, updated.status, "watcher response received")
	})
}

func TestWatcherResponseMsg_ErrorFlashContainsMessage(t *testing.T) {
	t.Run("flash notification contains the error message", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.apiInProgress = true

		result, cmd := m.Update(watcherResponseMsg{err: assert.AnError})
		updated := result.(model)

		assert.Contains(t, updated.status, "watcher query failed")
		assert.NotNil(t, cmd, "should return flash notification cmd")
	})
}

func TestWatcherResponseMsg_StartsTypewriter(t *testing.T) {
	t.Run("starts typewriter with response on success", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.apiInProgress = true
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, cmd := m.Update(watcherResponseMsg{response: "multi word response text"})
		updated := result.(model)

		assert.NotNil(t, updated.typewriter, "typewriter should be started")
		assert.NotNil(t, cmd, "should return typewriter tick cmd")
	})
}

// --- watcherSynthesisMsg handler additional edge case tests ---

func TestWatcherSynthesisMsg_ExpandsWatcherWhenCollapsed(t *testing.T) {
	t.Run("expands watcher pane if not already expanded", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.watcherExpanded = false
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, _ := m.Update(watcherSynthesisMsg{
			observation: "pattern detected",
			response:    "analysis of pattern",
		})
		updated := result.(model)

		assert.True(t, updated.watcherExpanded, "watcher should be expanded")
		assert.False(t, updated.watcherAnalyzing, "watcherAnalyzing should be cleared")
	})
}

func TestWatcherSynthesisMsg_ErrorAppendsObservation(t *testing.T) {
	t.Run("appends raw observation to buffer on error", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.watcherExpanded = true
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, cmd := m.Update(watcherSynthesisMsg{
			observation: "something unusual observed",
			err:         assert.AnError,
		})
		updated := result.(model)

		assert.False(t, updated.watcherAnalyzing)
		bufContent := updated.watcherBuffer.Content()
		assert.Contains(t, bufContent, "something unusual observed",
			"raw observation should be in buffer on error")
		assert.Nil(t, cmd, "should return nil cmd on error (no typewriter)")
	})
}

func TestWatcherSynthesisMsg_SuccessStartsTypewriter(t *testing.T) {
	t.Run("starts typewriter with response on success", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.watcherExpanded = true
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, cmd := m.Update(watcherSynthesisMsg{
			observation: "pattern detected",
			response:    "this looks like a cascading failure",
		})
		updated := result.(model)

		assert.False(t, updated.watcherAnalyzing)
		assert.NotNil(t, cmd, "should return typewriter tick cmd")
		assert.NotNil(t, updated.typewriter, "typewriter should be started")
	})
}

func TestWatcherSynthesisMsg_SuccessAppendsEmptyLine(t *testing.T) {
	t.Run("appends empty line to buffer before typewriter on success", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.watcherExpanded = true
		m.watcherBuffer.Append("prior entry")
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

		result, _ := m.Update(watcherSynthesisMsg{
			observation: "obs",
			response:    "resp",
		})
		updated := result.(model)

		assert.True(t, updated.watcherBuffer.Len() >= 2,
			"buffer should have at least 2 entries (prior + empty line)")
	})
}
