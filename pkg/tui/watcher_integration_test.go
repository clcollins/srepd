package tui

import (
	"context"
	"testing"
	"time"

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
	// MockProvider implements HealthChecker, so a failed check blocks queries
	// (no point hammering a probe-verified-down local daemon).
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("test")
	m.aiHealth = aiHealthError
	m.table.Focus()

	result, cmd := m.Update(watcherPromptMsg{prompt: "test"})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.NotNil(t, cmd, "should return flash notification")
}

// unverifiableProvider is a Provider without HealthChecker support, standing in
// for the anthropic/vertex/bedrock providers whose APIs have no probe endpoint.
type unverifiableProvider struct{}

func (p *unverifiableProvider) Query(_ context.Context, _ string, _ string) (string, error) {
	return "ok", nil
}
func (p *unverifiableProvider) StreamQuery(_ context.Context, _ string, _ string, ch chan<- string) error {
	close(ch)
	return nil
}
func (p *unverifiableProvider) Name() string { return "unverifiable" }

func TestWatcherPromptMsg_UnverifiedProviderAllowed(t *testing.T) {
	// A provider with no health-check support starts unverified; queries must
	// be allowed — the first query outcome is what establishes health.
	m := createTestModel()
	m.aiProvider = &unverifiableProvider{}
	m.aiHealth = aiHealthUnverified
	m.table.Focus()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	result, _ := m.Update(watcherPromptMsg{prompt: "test"})
	updated := result.(model)

	assert.True(t, updated.watcherAnalyzing, "unverified provider must not block queries")
}

func TestWatcherPromptMsg_ReactiveErrorAllowsRetry(t *testing.T) {
	// A probe-less provider in error state must still allow queries: only a
	// successful query can ever flip it back to healthy, so blocking here
	// would brick the watcher until restart.
	m := createTestModel()
	m.aiProvider = &unverifiableProvider{}
	m.aiHealth = aiHealthError
	m.table.Focus()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}

	result, _ := m.Update(watcherPromptMsg{prompt: "retry"})
	updated := result.(model)

	assert.True(t, updated.watcherAnalyzing, "probe-less provider in error state must allow retries")
}

func TestWatcherPromptMsg_Success(t *testing.T) {
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("test")
	m.aiHealth = aiHealthOK
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
	assert.NotNil(t, cmd, "should return a command carrying the error")

	// The error must route through errMsg so the full-screen error view is
	// shown, not just a transient status flash.
	msg := cmd()
	errM, ok := msg.(errMsg)
	assert.True(t, ok, "expected errMsg, got %T", msg)
	assert.Contains(t, errM.Error(), "watcher query failed")
}

func TestWatcherStreamDoneMsg_ErrorShowsErrorView(t *testing.T) {
	m := createTestModel()
	m.watcherAnalyzing = true
	m.apiInProgress = true
	m.watcherStreamCancel = func() {}

	result, cmd := m.Update(watcherStreamDoneMsg{err: assert.AnError})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.Nil(t, updated.watcherStreamCancel)
	assert.Equal(t, aiHealthError, updated.aiHealth, "a failed query must mark the provider errored")
	assert.NotNil(t, cmd, "should return a command carrying the error")

	msg := cmd()
	errM, ok := msg.(errMsg)
	assert.True(t, ok, "expected errMsg, got %T", msg)
	assert.Contains(t, errM.Error(), "watcher query failed")
}

func TestWatcherStreamDoneMsg_CanceledIsSilent(t *testing.T) {
	// A superseded stream ends with context.Canceled; that must not surface
	// as an error to the user, and must not change observed health.
	m := createTestModel()
	m.watcherAnalyzing = true
	m.aiHealth = aiHealthOK
	m.watcherStreamCancel = func() {}

	result, cmd := m.Update(watcherStreamDoneMsg{err: context.Canceled})
	updated := result.(model)

	assert.False(t, updated.watcherAnalyzing)
	assert.Nil(t, cmd, "canceled stream must not produce an error command")
	assert.Nil(t, updated.err, "canceled stream must not set the error view")
	assert.Equal(t, aiHealthOK, updated.aiHealth, "cancellation is not a health signal")
}

func TestWatcherStreamDoneMsg_SuccessSetsHealthOK(t *testing.T) {
	// A completed query is the strongest health evidence there is — it must
	// flip an unverified or errored provider to healthy.
	m := createTestModel()
	m.watcherAnalyzing = true
	m.aiHealth = aiHealthUnverified
	m.watcherStreamCancel = func() {}

	result, _ := m.Update(watcherStreamDoneMsg{})
	updated := result.(model)

	assert.Equal(t, aiHealthOK, updated.aiHealth, "a successful query must mark the provider healthy")
}

func TestWatcherResponseMsg_SuccessSetsHealthOK(t *testing.T) {
	m := createTestModel()
	m.watcherAnalyzing = true
	m.aiHealth = aiHealthError

	result, _ := m.Update(watcherResponseMsg{response: "all good"})
	updated := result.(model)

	assert.Equal(t, aiHealthOK, updated.aiHealth, "a successful query must recover an errored provider")
}

func TestWatcherResponseMsg_ErrorSetsHealthError(t *testing.T) {
	m := createTestModel()
	m.watcherAnalyzing = true
	m.aiHealth = aiHealthOK

	result, _ := m.Update(watcherResponseMsg{err: assert.AnError})
	updated := result.(model)

	assert.Equal(t, aiHealthError, updated.aiHealth, "a failed query must mark the provider errored")
}

func TestAIHealthCheckCmd_UnsupportedProviderIsUnverified(t *testing.T) {
	// A provider without HealthChecker support must never be reported
	// healthy by the periodic check — nothing was verified.
	cmd := aiHealthCheckCmd(&unverifiableProvider{})
	msg := cmd()

	checkMsg, ok := msg.(aiHealthCheckMsg)
	assert.True(t, ok, "expected aiHealthCheckMsg, got %T", msg)
	assert.Equal(t, aiHealthUnverified, checkMsg.state,
		"an unprobeable provider is unverified, not healthy")
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
	m.aiHealth = aiHealthOK

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "ollama")
	assert.Contains(t, status, "healthy")
}

func TestRenderWatcherStatus_Error(t *testing.T) {
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("ollama")
	m.aiHealth = aiHealthError

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "error")
	assert.NotContains(t, status, "healthy", "a failing provider must never read healthy")
}

func TestRenderWatcherStatus_StreamingHidesCountdown(t *testing.T) {
	// The countdown reflects the startup watchdog, which is disarmed by the
	// first token — once tokens are flowing the countdown is meaningless and
	// must be replaced by a streaming indicator immediately, not at zero.
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("test")
	m.aiHealth = aiHealthOK
	m.watcherAnalyzing = true
	m.watcherQueryStart = time.Now()
	m.watcherQueryTimeout = time.Minute
	m.watcherStreamPartial = "tokens are flowing"

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "streaming")
	assert.NotContains(t, status, "analyzing", "countdown must be replaced once the stream is responding")
}

func TestRenderWatcherStatus_AgentStreamingHidesCountdown(t *testing.T) {
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("test")
	m.aiHealth = aiHealthOK
	m.claudeQuerying = true
	m.watcherQueryStart = time.Now()
	m.watcherQueryTimeout = time.Minute
	m.agentStreamPartial = "agent tokens flowing"

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "streaming")
	assert.NotContains(t, status, "analyzing", "countdown must be replaced once the agent stream is responding")
}

func TestRenderWatcherStatus_LongStreamHidesDeadCountdown(t *testing.T) {
	// Streams have no whole-request deadline once responding; when the
	// nominal window has elapsed, showing "analyzing... 0s" reads like a
	// hung timer. Drop the countdown at zero.
	m := createTestModel()
	m.aiProvider = ai.NewMockProvider("test")
	m.aiHealth = aiHealthOK
	m.watcherAnalyzing = true
	m.watcherQueryStart = time.Now().Add(-2 * time.Minute)
	m.watcherQueryTimeout = time.Minute

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "analyzing")
	assert.NotContains(t, status, "analyzing... 0s", "elapsed countdown must be hidden, not shown as 0s")
}

func TestRenderWatcherStatus_Unverified(t *testing.T) {
	// A provider that cannot be probed and has not completed a query yet must
	// not claim to be healthy — it is unverified.
	m := createTestModel()
	m.aiProvider = &unverifiableProvider{}
	m.aiHealth = aiHealthUnverified

	status := m.renderWatcherStatus()
	assert.Contains(t, status, "unverified")
	assert.NotContains(t, status, "healthy", "an unprobed provider must never read healthy")
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
	m.aiHealth = aiHealthOK
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
	m.aiHealth = aiHealthOK
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
	m.aiHealth = aiHealthOK
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
	t.Run("sets aiHealth to OK when the probe passes", func(t *testing.T) {
		m := createTestModel()
		m.aiHealth = aiHealthError

		result, cmd := m.Update(aiHealthCheckMsg{state: aiHealthOK, err: nil})
		updated := result.(model)

		assert.Equal(t, aiHealthOK, updated.aiHealth, "aiHealth should be set to OK")
		assert.Nil(t, cmd, "should return nil cmd")
	})
}

func TestAiHealthCheckMsg_Unhealthy(t *testing.T) {
	t.Run("sets aiHealth to error when the probe fails", func(t *testing.T) {
		m := createTestModel()
		m.aiHealth = aiHealthOK

		result, cmd := m.Update(aiHealthCheckMsg{state: aiHealthError, err: assert.AnError})
		updated := result.(model)

		assert.Equal(t, aiHealthError, updated.aiHealth, "aiHealth should be set to error")
		assert.Nil(t, cmd, "should return nil cmd")
	})
}

func TestAiHealthCheckMsg_ErrorNotInspected(t *testing.T) {
	t.Run("err field is not inspected; only the state field matters", func(t *testing.T) {
		m := createTestModel()
		m.aiHealth = aiHealthError

		// state=OK even though err is set: the handler only reads msg.state
		result, cmd := m.Update(aiHealthCheckMsg{state: aiHealthOK, err: assert.AnError})
		updated := result.(model)

		assert.Equal(t, aiHealthOK, updated.aiHealth, "aiHealth should follow msg.state, not msg.err")
		assert.Nil(t, cmd)
	})
}

func TestAiHealthCheckMsg_UnhealthyWithNilError(t *testing.T) {
	t.Run("error state with nil error still sets error", func(t *testing.T) {
		m := createTestModel()
		m.aiHealth = aiHealthOK

		result, cmd := m.Update(aiHealthCheckMsg{state: aiHealthError, err: nil})
		updated := result.(model)

		assert.Equal(t, aiHealthError, updated.aiHealth)
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

func TestWatcherResponseMsg_ErrorCarriesMessage(t *testing.T) {
	t.Run("the error view message contains the underlying error", func(t *testing.T) {
		m := createTestModel()
		m.watcherAnalyzing = true
		m.apiInProgress = true

		_, cmd := m.Update(watcherResponseMsg{err: assert.AnError})

		assert.NotNil(t, cmd, "should return a command carrying the error")
		produced := cmd()
		errM, ok := produced.(errMsg)
		assert.True(t, ok, "expected errMsg, got %T", produced)
		assert.Contains(t, errM.Error(), "watcher query failed")
		assert.Contains(t, errM.Error(), assert.AnError.Error(),
			"the classified message must preserve the underlying error text")
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

func TestWatcherStreamChunkMsg_FirstTokenSetsHealthOK(t *testing.T) {
	// The first streamed token is proof the provider answered — health flips
	// to OK immediately, not at end of stream (mirrors the streaming timeout,
	// which also stops once the provider starts responding).
	m := createTestModel()
	windowSize = tea.WindowSizeMsg{Width: 80, Height: 60}
	m.aiHealth = aiHealthUnverified
	m.watcherBuffer.Append(prefixLines(m.watcherMarker, ""))

	ch := make(chan streamEvent)
	result, _ := m.Update(watcherStreamChunkMsg{text: "Hello", ch: ch})
	updated := result.(model)

	assert.Equal(t, aiHealthOK, updated.aiHealth,
		"first streamed token must mark the provider healthy")
}
