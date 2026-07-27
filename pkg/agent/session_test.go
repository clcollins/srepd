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
	s.Close()

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
		maxLive:  2,
		cfg:      cfg,
		exec: &delegatingExecutor{
			create: func() *mockStreamExecutor {
				return newMockStreamExecutor(`{"type":"system","subtype":"init","session_id":"test"}` + "\n")
			},
			sessions: sessions,
		},
	}

	mgr.GetOrCreate("INC-001", nil)
	mgr.GetOrCreate("INC-002", nil)

	// Creating a third should evict INC-001
	mgr.GetOrCreate("INC-003", nil)

	mgr.mu.Lock()
	assert.Len(t, mgr.order, 2, "should only have 2 sessions in order")
	mgr.mu.Unlock()
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
