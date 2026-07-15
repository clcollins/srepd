package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drainStream runs streamWatcherCmd end-to-end: it executes the start command,
// then repeatedly runs readStreamCmd, collecting chunk text in order until done.
// Returns the concatenated text and the terminal error.
func drainStream(t *testing.T, provider ai.Provider, timeout time.Duration) (string, error) {
	t.Helper()

	startMsg := streamWatcherCmd(provider, "sys", "user", "", timeout)()
	started, ok := startMsg.(watcherStreamStartedMsg)
	require.True(t, ok, "expected watcherStreamStartedMsg, got %T", startMsg)
	require.NotNil(t, started.ch)

	var got string
	ch := started.ch
	for {
		msg := readStreamCmd(ch)()
		switch m := msg.(type) {
		case watcherStreamChunkMsg:
			got += m.text
			ch = m.ch
		case watcherStreamDoneMsg:
			return got, m.err
		default:
			t.Fatalf("unexpected msg type %T", msg)
		}
	}
}

// pacedStreamProvider emits tokens with a configurable delay before each one,
// simulating a provider that thinks for a while mid-stream.
type pacedStreamProvider struct {
	tokens []string
	delays []time.Duration // delay before emitting tokens[i]
}

func (p *pacedStreamProvider) Query(context.Context, string, string) (string, error) {
	return "", nil
}
func (p *pacedStreamProvider) Name() string            { return "paced" }
func (p *pacedStreamProvider) SupportsStreaming() bool { return true }
func (p *pacedStreamProvider) StreamQuery(ctx context.Context, _, _ string, ch chan<- string) error {
	defer close(ch)
	for i, tok := range p.tokens {
		if d := p.delays[i]; d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		select {
		case ch <- tok:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestStreamWatcherCmd_TimeoutClearedAfterFirstToken(t *testing.T) {
	// The startup timeout guards only the wait for the FIRST token. Once the
	// provider starts responding, the stream must run to completion no matter
	// how long it takes (mirrors the agent CLI streaming behavior).
	provider := &pacedStreamProvider{
		tokens: []string{"first", " second"},
		delays: []time.Duration{0, 150 * time.Millisecond}, // 3x the timeout below
	}

	got, err := drainStream(t, provider, 50*time.Millisecond)

	assert.NoError(t, err, "a stream that started responding must never be truncated by the startup timeout")
	assert.Equal(t, "first second", got)
}

func TestStreamWatcherCmd_StartupTimeoutSurfacesError(t *testing.T) {
	// A provider that never produces a token must be killed by the startup
	// watchdog, and the error must be identifiable as a timeout — NOT
	// context.Canceled, which the Update loop deliberately swallows as the
	// superseded-stream case.
	provider := &pacedStreamProvider{
		tokens: []string{"never delivered"},
		delays: []time.Duration{time.Second},
	}

	got, err := drainStream(t, provider, 50*time.Millisecond)

	assert.Empty(t, got)
	require.Error(t, err, "startup timeout must surface as an error")
	assert.False(t, context.Canceled == err, "timeout must not be reported as bare context.Canceled")
	assert.Contains(t, err.Error(), "no response", "error should say the provider never responded")
}

func TestStreamWatcherCmd_EmitsChunksInOrderThenDone(t *testing.T) {
	provider := ai.NewMockProvider("mock")
	provider.Streaming = true
	provider.StreamTokens = []string{"Hello", ", ", "world"}

	got, err := drainStream(t, provider, 5*time.Second)

	assert.NoError(t, err)
	assert.Equal(t, "Hello, world", got, "chunks must accumulate in arrival order")
	assert.Equal(t, 1, provider.CallCounts["StreamQuery"])
}

func TestStreamWatcherCmd_PropagatesStreamError(t *testing.T) {
	provider := ai.NewMockProvider("mock")
	provider.Streaming = true
	provider.StreamTokens = []string{"partial"}
	provider.StreamErr = assert.AnError

	got, err := drainStream(t, provider, 5*time.Second)

	// The partial token is still delivered before the terminal error.
	assert.Equal(t, "partial", got)
	assert.Error(t, err)
}

func TestReadStreamCmd_ClosedChannelYieldsDone(t *testing.T) {
	ch := make(chan streamEvent)
	close(ch)

	msg := readStreamCmd(ch)()

	_, ok := msg.(watcherStreamDoneMsg)
	assert.True(t, ok, "a closed channel should yield watcherStreamDoneMsg")
}

// Ensure the messages satisfy tea.Msg (compile-time guard).
var (
	_ tea.Msg = watcherStreamStartedMsg{}
	_ tea.Msg = watcherStreamChunkMsg{}
	_ tea.Msg = watcherStreamDoneMsg{}
)
