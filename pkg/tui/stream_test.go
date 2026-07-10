package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drainStream runs streamWatcherCmd end-to-end: it executes the start command,
// then repeatedly runs readStreamCmd, collecting chunk text in order until done.
// Returns the concatenated text and the terminal error.
func drainStream(t *testing.T, provider ai.Provider) (string, error) {
	t.Helper()

	startMsg := streamWatcherCmd(provider, "sys", "user", "")()
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

func TestStreamWatcherCmd_EmitsChunksInOrderThenDone(t *testing.T) {
	provider := ai.NewMockProvider("mock")
	provider.Streaming = true
	provider.StreamTokens = []string{"Hello", ", ", "world"}

	got, err := drainStream(t, provider)

	assert.NoError(t, err)
	assert.Equal(t, "Hello, world", got, "chunks must accumulate in arrival order")
	assert.Equal(t, 1, provider.CallCounts["StreamQuery"])
}

func TestStreamWatcherCmd_PropagatesStreamError(t *testing.T) {
	provider := ai.NewMockProvider("mock")
	provider.Streaming = true
	provider.StreamTokens = []string{"partial"}
	provider.StreamErr = assert.AnError

	got, err := drainStream(t, provider)

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
