package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
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
	output := `{"type":"system","subtype":"init","session_id":"00000000-fake-0000-0000-000000000000"}
{"type":"result","subtype":"success","session_id":"00000000-fake-0000-0000-000000000000","result":"done","is_error":false}
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
	output := `{"type":"system","subtype":"init","session_id":"00000000-fake-0000-0000-000000000000"}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}
{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello world"}]}}
{"type":"result","subtype":"success","session_id":"00000000-fake-0000-0000-000000000000","result":"Hello world","is_error":false}
`
	exec := newMockStreamExecutor(output)
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	timeout := time.After(2 * time.Second)
	var textEvents []Event
done:
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				break done
			}
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

// M3: When no stream_event deltas arrive, consolidated assistant text must
// not be suppressed. This guards against a CLI version change or flag
// behavior shift where partial deltas stop arriving.
func TestSession_ConsolidatedTextSurfacedWithoutDeltas(t *testing.T) {
	// Init → consolidated assistant text → result, NO stream_event deltas
	output := `{"type":"system","subtype":"init","session_id":"00000000-fake-0000-0000-000000000000"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello from consolidated"}]}}
{"type":"result","subtype":"success","session_id":"00000000-fake-0000-0000-000000000000","result":"Hello from consolidated","is_error":false}
`
	exec := newMockStreamExecutor(output)
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	timeout := time.After(2 * time.Second)
	var textEvents []Event
done:
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				break done
			}
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

	require.Len(t, textEvents, 1, "consolidated text must surface when no deltas arrive")
	assert.Equal(t, "Hello from consolidated", textEvents[0].Text)
	assert.True(t, textEvents[0].Consolidated, "event should be marked consolidated")
}

func TestSession_SlowConsumerReceivesResult(t *testing.T) {
	// D3: Result must not be dropped even if the consumer is slow.
	// We produce many TextDelta events followed by a Result.
	var lines strings.Builder
	lines.WriteString(`{"type":"system","subtype":"init","session_id":"00000000-fake-0000-0000-000000000000"}` + "\n")
	for i := 0; i < 200; i++ {
		lines.WriteString(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"x"}}}` + "\n")
	}
	lines.WriteString(`{"type":"result","subtype":"success","session_id":"00000000-fake-0000-0000-000000000000","result":"done","is_error":false}` + "\n")

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
	output := `{"type":"system","subtype":"init","session_id":"00000000-fake-0000-0000-000000000000"}
{"type":"result","subtype":"success","session_id":"00000000-fake-0000-0000-000000000000","result":"done","is_error":false}
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
	output := `{"type":"system","subtype":"init","session_id":"00000000-fake-0000-0000-000000000000"}
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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
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
		index:     newSessionIndex(""),
		ctx:       ctx,
		ctxCancel: cancel,
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

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
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
		index:     newSessionIndex(""),
		ctx:       ctx,
		ctxCancel: cancel,
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

// B1: Test that spawn() rejects denied flags injected via agent_cli_command.
// This exercises the REAL argv that spawn() assembles, not just BuildSpawnArgs.
func TestSpawn_RejectsDeniedFlags(t *testing.T) {
	tests := []struct {
		name       string
		cliCommand string
		wantErr    string
	}{
		{"--bare flag", "claude --bare", "--bare"},
		{"--bare=true flag", "claude --bare=true", "--bare"},
		{"--dangerously-skip-permissions", "claude --dangerously-skip-permissions", "--dangerously-skip-permissions"},
		{"--permission-mode override", "claude --permission-mode bypassPermissions", "--permission-mode"},
		{"--allowedTools override", "claude --allowedTools Bash", "--allowedTools"},
		{"--disallowedTools override", "claude --disallowedTools Read", "--disallowedTools"},
		{"--session-id override", "claude --session-id abc", "--session-id"},
		{"--session-id=value form", "claude --session-id=abc", "--session-id"},
		{"--resume override", "claude --resume abc", "--resume"},
		{"--resume=value form", "claude --resume=abc", "--resume"},
		{"-r short alias", "claude -r abc", "-r"},
		{"-r=value form", "claude -r=abc", "-r"},
		{"--continue override", "claude --continue", "--continue"},
		{"--continue=value form", "claude --continue=abc", "--continue"},
		{"-c short alias", "claude -c", "-c"},
		{"-c=value form", "claude -c=abc", "-c"},
		{"--fork-session override", "claude --fork-session", "--fork-session"},
		{"--fork-session=value form", "claude --fork-session=true", "--fork-session"},
		{"--input-format override", "claude --input-format text", "--input-format"},
		{"--output-format override", "claude --output-format text", "--output-format"},
		{"--bare=value form", "claude --bare=1", "--bare"},
		{"denied flag mid-args", "claude --model opus --bare --verbose", "--bare"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			exec := &argCapturingExecutor{
				output:   `{"type":"system","subtype":"init","session_id":"test"}` + "\n",
				captured: &capturedArgs,
			}
			cfg := Config{CLICommand: tt.cliCommand, SessionEnabled: true}
			s := NewSession(cfg, "INC-001", exec, nil)

			err := s.Send(context.Background(), "hi")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, err.Error(), "denied flag")

			// Verify the process was never started
			assert.Empty(t, capturedArgs, "denied flag must prevent process spawn")
		})
	}
}

// B1: Test that spawn() allows legitimate user flags.
func TestSpawn_AllowsLegitimateFlags(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"test"}
{"type":"result","subtype":"success","session_id":"test","result":"ok","is_error":false}
`
	var capturedArgs []string
	exec := &argCapturingExecutor{
		output:   output,
		captured: &capturedArgs,
	}
	cfg := Config{CLICommand: "claude --model opus --verbose", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)
	assert.NotEmpty(t, capturedArgs, "legitimate flags should allow spawn")

	// Verify user args appear in the final argv
	found := false
	for _, a := range capturedArgs {
		if a == "--model" {
			found = true
			break
		}
	}
	assert.True(t, found, "--model should be in argv")
}

// B7: Test that scanner errors are surfaced as Error events.
func TestSession_ScannerErrorSurfaced(t *testing.T) {
	// Create a reader that emits one valid line then an oversized line
	// that exceeds the 256KB scanner buffer, causing a scanner error.
	var lines strings.Builder
	lines.WriteString(`{"type":"system","subtype":"init","session_id":"test"}` + "\n")
	// Write a line longer than 256KB without a newline terminator to force
	// bufio.Scanner to hit ErrTooLong
	lines.WriteString(strings.Repeat("x", 300*1024))
	lines.WriteString("\n")

	exec := newMockStreamExecutor(lines.String())
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	timeout := time.After(3 * time.Second)
	var sawError bool
loop:
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				break loop
			}
			if ev.Kind == Error {
				sawError = true
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	assert.True(t, sawError, "scanner error must surface as an Error event")
}

// B8: Test that Done() closes for never-spawned sessions.
func TestSession_DoneClosesForNeverSpawned(t *testing.T) {
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	exec := newMockStreamExecutor("")
	s := NewSession(cfg, "INC-001", exec, nil)

	// Close without ever calling Send
	err := s.Close()
	require.NoError(t, err)

	// Done() should be closed
	select {
	case <-s.Done():
		// success
	case <-time.After(time.Second):
		t.Fatal("Done() should be closed after Close() on never-spawned session")
	}
}

// B5: Test that Close() after natural child exit still cleans up.
func TestSession_CloseAfterNaturalExit(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"test"}
{"type":"result","subtype":"success","session_id":"test","result":"ok","is_error":false}
`
	exec := newMockStreamExecutor(output)
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	// Wait for readLoop to finish (natural child exit)
	<-s.Done()

	// Close() after natural exit should not error
	err = s.Close()
	require.NoError(t, err)

	// Double Close should also be safe
	err = s.Close()
	require.NoError(t, err)
}

// M5: Deliberate close must not emit a spurious Error event from wait().
// When Close() or eviction kills the child, the exec.ExitError ("signal:
// killed") must be suppressed because the kill was intentional.
func TestSession_CloseNoSpuriousError(t *testing.T) {
	// Use a reader that blocks until context is cancelled, simulating a
	// long-lived child process.
	pr, pw := io.Pipe()
	waitErr := fmt.Errorf("signal: killed")
	executor := &callbackExecutor{
		startFn: func(ctx context.Context, _ string, _ []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
			go func() {
				<-ctx.Done()
				_ = pw.Close()
			}()
			return &mockStdin{}, pr, func() error { return waitErr }, nil
		},
	}

	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", executor, nil)

	// Write init so readLoop starts scanning
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = pw.Write([]byte(`{"type":"system","subtype":"init","session_id":"test"}` + "\n"))
	}()

	err := s.Send(context.Background(), "hi")
	require.NoError(t, err)

	// Wait for Init event to confirm readLoop is running
	timeout := time.After(2 * time.Second)
	select {
	case ev := <-s.Events():
		assert.Equal(t, Init, ev.Kind)
	case <-timeout:
		t.Fatal("timed out waiting for Init")
	}

	// Close deliberately
	require.NoError(t, s.Close())

	// Drain remaining events — there must be no Error event
	drainTimeout := time.After(time.Second)
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				return // channel closed, no error — correct
			}
			assert.NotEqual(t, Error, ev.Kind, "deliberate close must not emit Error event, got: %v", ev.Err)
		case <-s.Done():
			return
		case <-drainTimeout:
			return
		}
	}
}

// M1: A crashed session (closed=true from readLoop) must be replaced by
// GetOrCreate with a fresh Session using --resume. The dead session must
// not brick the incident's chat.
func TestSessionManager_CrashedSessionReplaced(t *testing.T) {
	cfg := Config{CLICommand: "claude", SessionEnabled: true, MaxSessions: 3}

	var spawnCount int
	var lastArgs []string
	exec := &callbackExecutor{
		startFn: func(_ context.Context, _ string, args []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
			spawnCount++
			lastArgs = append([]string{}, args...)
			stdin := &mockStdin{}
			output := `{"type":"system","subtype":"init","session_id":"test"}` + "\n" +
				`{"type":"result","subtype":"success","session_id":"test","result":"ok","is_error":false}` + "\n"
			stdout := io.NopCloser(strings.NewReader(output))
			return stdin, stdout, func() error { return nil }, nil
		},
	}

	mgr := NewSessionManager(cfg, exec)

	// Get session, send to spawn it, wait for readLoop to finish (child dies)
	s1 := mgr.GetOrCreate("INC-001", nil)
	err := s1.Send(context.Background(), "hello")
	require.NoError(t, err)
	<-s1.Done() // readLoop sets closed=true

	s1.mu.Lock()
	assert.True(t, s1.closed, "session should be closed after readLoop exits")
	s1.mu.Unlock()

	// GetOrCreate must detect the dead session and create a new one
	s2 := mgr.GetOrCreate("INC-001", nil)
	assert.True(t, s1 != s2, "must get a different Session pointer after crash")
	assert.True(t, s2.resumed, "replacement session must use --resume")

	// Send on the new session should succeed
	err = s2.Send(context.Background(), "world")
	require.NoError(t, err)

	// Verify --resume is in the spawn args
	hasResume := false
	for _, a := range lastArgs {
		if a == "--resume" {
			hasResume = true
			break
		}
	}
	assert.True(t, hasResume, "replacement session must be spawned with --resume")
}

// callbackExecutor delegates Start to a function for flexible test setups.
type callbackExecutor struct {
	startFn func(ctx context.Context, name string, args []string, env []string) (io.WriteCloser, io.ReadCloser, func() error, error)
}

func (e *callbackExecutor) Start(ctx context.Context, name string, args []string, env []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	return e.startFn(ctx, name, args, env)
}

// hungWriter accepts the first write, then blocks all subsequent writes
// until Close is called.
type hungWriter struct {
	mu      sync.Mutex
	first   bool
	closeCh chan struct{}
}

func newHungWriter() *hungWriter {
	return &hungWriter{first: true, closeCh: make(chan struct{})}
}

func (w *hungWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	if w.first {
		w.first = false
		w.mu.Unlock()
		return len(p), nil
	}
	w.mu.Unlock()
	<-w.closeCh
	return 0, io.ErrClosedPipe
}

func (w *hungWriter) Close() error {
	select {
	case <-w.closeCh:
	default:
		close(w.closeCh)
	}
	return nil
}

// FIX 2: Write goroutine must not leak per timed-out Send.
// A single writer goroutine should be reused across Send calls.
func TestSession_WriteGoroutineDoesNotLeak(t *testing.T) {
	hw := newHungWriter()
	executor := &callbackExecutor{
		startFn: func(ctx context.Context, _ string, _ []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
			stdoutR, stdoutW := io.Pipe()
			go func() {
				_, _ = stdoutW.Write([]byte(`{"type":"system","subtype":"init","session_id":"test"}` + "\n"))
				<-ctx.Done()
				_ = stdoutW.Close()
			}()
			return hw, stdoutR, func() error {
				<-ctx.Done()
				return fmt.Errorf("signal: killed")
			}, nil
		},
	}

	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", executor, nil)
	t.Cleanup(func() { _ = s.Close() })

	// First send spawns the process (first write succeeds)
	err := s.Send(context.Background(), "hello")
	require.NoError(t, err)

	// Wait for init
	select {
	case ev := <-s.Events():
		require.Equal(t, Init, ev.Kind)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Init")
	}

	// Settle goroutines
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Issue several sends that will time out (child not draining stdin)
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		_ = s.Send(ctx, fmt.Sprintf("message %d", i))
		cancel()
	}

	// Wait for goroutines to settle
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()

	// With the fix (single writer goroutine), count should not grow per send.
	assert.LessOrEqual(t, after, baseline+2,
		"goroutine count grew from %d to %d — write goroutines are leaking", baseline, after)

	// Close must release everything
	require.NoError(t, s.Close())
	time.Sleep(300 * time.Millisecond)
	final := runtime.NumGoroutine()
	assert.Less(t, final, baseline+2,
		"goroutines not released after Close: %d (baseline was %d)", final, baseline)
}

// S1: Test that Send() honors context cancellation.
func TestSession_SendHonorsContext(t *testing.T) {
	output := `{"type":"system","subtype":"init","session_id":"test"}
`
	// Use a writer that blocks forever to simulate a hung pipe
	blockingStdin := &blockingWriter{}
	exec := &customStdinExecutor{
		stdin:  blockingStdin,
		output: output,
	}
	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)

	// First send spawns the process
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = s.Send(ctx, "hello")
	// The first Send may succeed (spawn + write to non-blocking mock).
	// But a second send with an already-cancelled context should fail.
	cancel()
	err := s.Send(ctx, "this should fail")
	assert.Error(t, err, "Send with cancelled context should fail")
}

type blockingWriter struct {
	mu     sync.Mutex
	closed bool
}

func (b *blockingWriter) Write(p []byte) (int, error) {
	// Block until closed
	for {
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return 0, io.ErrClosedPipe
		}
		b.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
}

func (b *blockingWriter) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

type customStdinExecutor struct {
	stdin  io.WriteCloser
	output string
}

func (e *customStdinExecutor) Start(_ context.Context, _ string, _ []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	stdout := io.NopCloser(strings.NewReader(e.output))
	return e.stdin, stdout, func() error { return nil }, nil
}

// contextAwareExecutor implements StreamCommandExecutor and RESPECTS
// context cancellation, mimicking exec.CommandContext. When the context
// passed to Start is cancelled, stdout closes and wait() returns with
// an error — just like a real killed subprocess.
//
// Unlike mockStreamExecutor and argCapturingExecutor, this executor
// does NOT discard the context. It exists specifically to test the
// bug where spawn derived the subprocess lifetime from the caller's
// Send context: a mock that ignores context cannot detect that bug.
type contextAwareExecutor struct {
	initOutput string
}

func (e *contextAwareExecutor) Start(ctx context.Context, _ string, _ []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	stdin := &mockStdin{}
	pr, pw := io.Pipe()

	go func() {
		_, _ = pw.Write([]byte(e.initOutput))
		<-ctx.Done()
		_ = pw.Close()
	}()

	return stdin, pr, func() error {
		<-ctx.Done()
		return fmt.Errorf("signal: killed")
	}, nil
}

// C1 headline regression test: the caller's Send context must not
// kill the subprocess. Before the fix, spawn derived spawnCtx from
// the caller's context; cancelling it cascaded into the child.
func TestSession_CallerCancelDoesNotKillProcess(t *testing.T) {
	initOutput := `{"type":"system","subtype":"init","session_id":"test"}` + "\n"
	exec := &contextAwareExecutor{initOutput: initOutput}

	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", exec, nil)
	t.Cleanup(func() { _ = s.Close() })

	callerCtx, callerCancel := context.WithCancel(context.Background())
	err := s.Send(callerCtx, "hello")
	require.NoError(t, err)

	// Wait for Init event to confirm process is running
	select {
	case ev := <-s.Events():
		require.Equal(t, Init, ev.Kind)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Init")
	}

	// Cancel the caller context — simulates defer cancel() in startAgentSession
	callerCancel()
	time.Sleep(200 * time.Millisecond)

	// Session must still be alive
	select {
	case <-s.Done():
		t.Fatal("session must still be alive after caller context cancel")
	default:
	}

	s.mu.Lock()
	assert.False(t, s.closed, "session must not be closed by caller cancel")
	s.mu.Unlock()

	// A second Send with a fresh context must succeed
	err = s.Send(context.Background(), "second message")
	assert.NoError(t, err, "second Send must succeed after caller cancel")
}

// N4: Send(context.Background()) against a non-draining child must not
// prevent Close() from returning promptly. Before the fix, Send held
// s.mu across a blocking select, so Close() would block on the lock.
func TestSession_SendDoesNotBlockClose(t *testing.T) {
	pr, pw := io.Pipe()
	executor := &callbackExecutor{
		startFn: func(ctx context.Context, _ string, _ []string, _ []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
			go func() {
				_, _ = pw.Write([]byte(`{"type":"system","subtype":"init","session_id":"test"}` + "\n"))
				<-ctx.Done()
				_ = pw.Close()
			}()
			// stdin that never drains: writes block forever
			neverDrain := &neverDrainWriter{ctx: ctx}
			return neverDrain, pr, func() error {
				<-ctx.Done()
				return fmt.Errorf("signal: killed")
			}, nil
		},
	}

	cfg := Config{CLICommand: "claude", SessionEnabled: true}
	s := NewSession(cfg, "INC-001", executor, nil)

	// First send spawns the process
	require.NoError(t, s.Send(context.Background(), "hello"))

	// Wait for Init
	select {
	case ev := <-s.Events():
		require.Equal(t, Init, ev.Kind)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Init")
	}

	// Launch a Send(context.Background()) that will block on the non-draining stdin
	sendDone := make(chan struct{})
	go func() {
		_ = s.Send(context.Background(), "this will block")
		close(sendDone)
	}()
	// Give the goroutine time to enter the blocking select
	time.Sleep(100 * time.Millisecond)

	// Close must return promptly, not be blocked by Send holding the lock
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- s.Close()
	}()

	select {
	case err := <-closeDone:
		assert.NoError(t, err, "Close must succeed")
	case <-time.After(2 * time.Second):
		t.Fatal("Close() blocked — Send holds s.mu across blocking select")
	}

	// The blocked Send should also unblock (context cancelled by Close)
	select {
	case <-sendDone:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked Send did not unblock after Close")
	}
}

// neverDrainWriter blocks writes until context is cancelled.
type neverDrainWriter struct {
	ctx context.Context
}

func (w *neverDrainWriter) Write(_ []byte) (int, error) {
	<-w.ctx.Done()
	return 0, io.ErrClosedPipe
}

func (w *neverDrainWriter) Close() error {
	return nil
}

// C1 reaping test: CloseAll must still terminate children after the
// fix. This proves the process is not made immortal by decoupling
// from the caller context.
func TestSessionManager_CloseAllReapsChildren(t *testing.T) {
	initOutput := `{"type":"system","subtype":"init","session_id":"test"}` + "\n"
	exec := &contextAwareExecutor{initOutput: initOutput}

	cfg := Config{CLICommand: "claude", SessionEnabled: true, MaxSessions: 3}
	mgr := NewSessionManager(cfg, exec)

	s1 := mgr.GetOrCreate("INC-001", nil)
	require.NoError(t, s1.Send(context.Background(), "hello"))

	s2 := mgr.GetOrCreate("INC-002", nil)
	require.NoError(t, s2.Send(context.Background(), "world"))

	// Wait for both to receive Init
	for i, s := range []*Session{s1, s2} {
		select {
		case ev := <-s.Events():
			require.Equal(t, Init, ev.Kind, "session %d", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("session %d: timed out waiting for Init", i)
		}
	}

	// CloseAll must terminate all children
	mgr.CloseAll()

	// Both sessions' Done() must close
	for i, s := range []*Session{s1, s2} {
		select {
		case <-s.Done():
		case <-time.After(3 * time.Second):
			t.Fatalf("session %d: Done() must close after CloseAll", i)
		}
	}
}
