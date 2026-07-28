package agent

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStdin implements io.WriteCloser for tests.
type mockStdin struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
}

func (m *mockStdin) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.buf.Write(p)
}

func (m *mockStdin) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockStdin) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

// mockStreamExecutor implements StreamCommandExecutor for tests.
type mockStreamExecutor struct {
	mu      sync.Mutex
	started bool
	stdin   *mockStdin
	stdout  io.ReadCloser
	wait    func() error
}

func newMockStreamExecutor(output string) *mockStreamExecutor {
	stdin := &mockStdin{}
	stdout := io.NopCloser(strings.NewReader(output))
	return &mockStreamExecutor{
		stdin:  stdin,
		stdout: stdout,
		wait:   func() error { return nil },
	}
}

func (m *mockStreamExecutor) Start(_ context.Context, _ string, _ []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return m.stdin, m.stdout, m.wait, nil
}

func TestSession_SpawnOnFirstSend(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"00000000-0000-0000-0000-000000000000"}
{"type":"result","subtype":"success","session_id":"00000000-0000-0000-0000-000000000000","result":"done","is_error":false}
`
	exec := newMockStreamExecutor(output)

	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hello")
	require.NoError(t, err)

	assert.True(t, exec.started, "process should be started on first Send")
	assert.Contains(t, exec.stdin.String(), "hello", "user turn should be written to stdin")

	// Wait for events to be read
	timeout := time.After(2 * time.Second)
	var events []Event
loop:
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				break loop
			}
			events = append(events, ev)
			if ev.Kind == Result {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	require.True(t, len(events) >= 1, "should receive at least one event")
	assert.Equal(t, Init, events[0].Kind)
}

func TestSession_DoubleRenderPrevention(t *testing.T) {
	// Integration test: a fixture stream that contains BOTH stream_event
	// deltas and a consolidated assistant text block. With double-render
	// prevention, only the stream_event deltas should surface as TextDelta;
	// the consolidated assistant text should be suppressed.
	output := `{"type":"system","subtype":"init","session_id":"00000000-0000-0000-0000-000000000000"}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello world"}]}}
{"type":"result","subtype":"success","session_id":"00000000-0000-0000-0000-000000000000","result":"Hello world","is_error":false}
`
	exec := newMockStreamExecutor(output)
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	timeout := time.After(2 * time.Second)
	var textEvents []Event
	var allEvents []Event
done:
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				break done
			}
			allEvents = append(allEvents, ev)
			if ev.Kind == TextDelta {
				textEvents = append(textEvents, ev)
			}
			if ev.Kind == Result {
				break done
			}
		case <-timeout:
			break done
		}
	}

	// Only the two stream_event deltas should surface, not the consolidated one
	require.Len(t, textEvents, 2, "text should surface exactly twice (two stream_event deltas)")
	assert.Equal(t, "Hello", textEvents[0].Text)
	assert.Equal(t, " world", textEvents[1].Text)

	// Revert check: the consolidated event was present in the stream but suppressed
	assert.False(t, textEvents[0].Consolidated)
	assert.False(t, textEvents[1].Consolidated)
}

func TestSession_SlowConsumerReceivesResult(t *testing.T) {
	// D3: Result must not be dropped even if the consumer is slow.
	// We produce many TextDelta events followed by a Result.
	var lines strings.Builder
	lines.WriteString(`{"type":"system","subtype":"init","session_id":"00000000-0000-0000-0000-000000000000"}` + "\n")
	for i := 0; i < 200; i++ {
		lines.WriteString(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"x"}}}` + "\n")
	}
	lines.WriteString(`{"type":"result","subtype":"success","session_id":"00000000-0000-0000-0000-000000000000","result":"done","is_error":false}` + "\n")

	exec := newMockStreamExecutor(lines.String())
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	// Slow consumer: drain events with a small delay
	timeout := time.After(5 * time.Second)
	var sawResult bool
done:
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				break done
			}
			if ev.Kind == Result {
				sawResult = true
				break done
			}
			time.Sleep(time.Millisecond) // simulate slow consumer
		case <-timeout:
			break done
		}
	}

	assert.True(t, sawResult, "Result event must not be dropped under backpressure")
}

func TestSession_CLICommandArgsPreserved(t *testing.T) {
	// D4: user-supplied args from agent_cli_command must be passed through
	output := `{"type":"system","subtype":"init","session_id":"00000000-0000-0000-0000-000000000000"}
{"type":"result","subtype":"success","session_id":"00000000-0000-0000-0000-000000000000","result":"done","is_error":false}
`
	var capturedArgs []string
	exec := &argCapturingExecutor{
		output:   output,
		captured: &capturedArgs,
	}

	cfg := Config{CLICommand: "claude --foo bar", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	// Wait for spawn
	time.Sleep(100 * time.Millisecond)

	require.NotEmpty(t, capturedArgs, "args should be captured")
	// --foo and bar should appear before the stream-json flags
	fooIdx := -1
	barIdx := -1
	for i, a := range capturedArgs {
		if a == "--foo" {
			fooIdx = i
		}
		if a == "bar" {
			barIdx = i
		}
	}
	assert.True(t, fooIdx >= 0, "--foo must be in args")
	assert.True(t, barIdx >= 0, "bar must be in args")
	assert.True(t, barIdx == fooIdx+1, "bar must follow --foo")
}

type argCapturingExecutor struct {
	output   string
	captured *[]string
}

func (e *argCapturingExecutor) Start(_ context.Context, _ string, args []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	*e.captured = append([]string{}, args...)
	stdin := &mockStdin{}
	stdout := io.NopCloser(strings.NewReader(e.output))
	return stdin, stdout, func() error { return nil }, nil
}

func TestSession_Close(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"00000000-0000-0000-0000-000000000000"}
`
	exec := newMockStreamExecutor(output)

	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hello")
	require.NoError(t, err)

	err = s.Close()
	require.NoError(t, err)

	// Double close should be safe
	err = s.Close()
	require.NoError(t, err)
}

func TestSession_SendAfterClose(t *testing.T) {
	exec := newMockStreamExecutor("")
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)
	require.NoError(t, s.Close())

	err := s.Send(context.Background(), "hello")
	assert.Error(t, err, "Send after Close should return error")
}

func TestSessionManager_GetOrCreate(t *testing.T) {
	exec := newMockStreamExecutor("")
	cfg := Config{CLICommand: "claude", SessionEnabled: true, MaxSessions: 3}
	mgr := NewSessionManager(cfg, exec)

	s1 := mgr.GetOrCreate("INC-001", nil)
	s2 := mgr.GetOrCreate("INC-001", nil)
	assert.Equal(t, s1, s2, "same incident should return same session")

	s3 := mgr.GetOrCreate("INC-002", nil)
	assert.NotEqual(t, s1, s3, "different incident should return different session")
}

func TestSessionManager_LRUEviction(t *testing.T) {
	cfg := Config{CLICommand: "claude", SessionEnabled: true, MaxSessions: 2}

	sessions := make(map[string]*mockStreamExecutor)
	mgr := &SessionManager{
		sessions: make(map[string]*Session),
		evicted:  make(map[string]bool),
		maxLive:  2,
		cfg:      cfg,
		exec: &delegatingExecutor{
			create: func() *mockStreamExecutor {
				return newMockStreamExecutor(`{"type":"system","subtype":"init","session_id":"test"}` + "\n")
			},
			sessions: sessions,
		},
	}

	s1 := mgr.GetOrCreate("INC-001", nil)
	mgr.GetOrCreate("INC-002", nil)

	// Creating a third should evict INC-001
	mgr.GetOrCreate("INC-003", nil)

	mgr.mu.Lock()
	assert.Len(t, mgr.order, 2, "should only have 2 sessions in order")
	mgr.mu.Unlock()

	// Re-accessing INC-001 should create a new Session (not the old one)
	s1Again := mgr.GetOrCreate("INC-001", nil)
	assert.True(t, s1 != s1Again, "evicted incident must get a new Session pointer")
	assert.True(t, s1Again.resumed, "re-created session must use --resume")
}

// delegatingExecutor creates a fresh mock for each Start call
type delegatingExecutor struct {
	mu       sync.Mutex
	create   func() *mockStreamExecutor
	sessions map[string]*mockStreamExecutor
}

func (d *delegatingExecutor) Start(ctx context.Context, name string, args []string, env []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	m := d.create()
	return m.Start(ctx, name, args, env)
}

func TestSessionManager_EvictionNoRace(t *testing.T) {
	// D2: new Session per spawn prevents race between old readLoop and new session.
	// Run with -race to verify.
	cfg := Config{CLICommand: "claude", SessionEnabled: true, MaxSessions: 1}

	mgr := &SessionManager{
		sessions: make(map[string]*Session),
		evicted:  make(map[string]bool),
		maxLive:  1,
		cfg:      cfg,
		exec: &delegatingExecutor{
			create: func() *mockStreamExecutor {
				return newMockStreamExecutor(
					`{"type":"system","subtype":"init","session_id":"test"}` + "\n" +
						`{"type":"result","subtype":"success","session_id":"test","result":"ok","is_error":false}` + "\n",
				)
			},
			sessions: make(map[string]*mockStreamExecutor),
		},
	}

	// Create and send on INC-001
	s1 := mgr.GetOrCreate("INC-001", nil)
	err := s1.Send(context.Background(), "hi")
	require.NoError(t, err)

	// Wait for s1's readLoop to finish
	<-s1.Done()

	// Evict INC-001 by creating INC-002
	s2 := mgr.GetOrCreate("INC-002", nil)
	err = s2.Send(context.Background(), "hi")
	require.NoError(t, err)

	// Wait for s2's readLoop to finish
	<-s2.Done()

	// Re-access INC-001: should get a new Session (different pointer)
	s1Again := mgr.GetOrCreate("INC-001", nil)
	assert.True(t, s1 != s1Again, "must be a new Session pointer, not the old one")
	assert.True(t, s1Again.resumed, "re-created session must use --resume")

	// The old s1's readLoop is finished; it cannot affect s1Again
	s1Again.mu.Lock()
	closed := s1Again.closed
	s1Again.mu.Unlock()
	assert.False(t, closed, "new session must not be closed by old readLoop")
}

func TestSessionManager_CloseAll(t *testing.T) {
	exec := newMockStreamExecutor("")
	cfg := Config{CLICommand: "claude", SessionEnabled: true, MaxSessions: 3}
	mgr := NewSessionManager(cfg, exec)

	mgr.GetOrCreate("INC-001", nil)
	mgr.GetOrCreate("INC-002", nil)

	mgr.CloseAll()
	// Should not panic on double close
	mgr.CloseAll()
}
