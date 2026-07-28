package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
)

// StreamCommandExecutor abstracts spawning a long-lived process with
// stdin/stdout pipes. The real implementation uses os/exec; tests inject
// a mock.
type StreamCommandExecutor interface {
	Start(ctx context.Context, name string, args []string, env []string) (stdin io.WriteCloser, stdout io.ReadCloser, wait func() error, err error)
}

// execStreamExecutor is the default implementation using os/exec.
type execStreamExecutor struct{}

func (e *execStreamExecutor) Start(ctx context.Context, name string, args []string, env []string) (io.WriteCloser, io.ReadCloser, func() error, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = 2 * time.Second

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, nil, nil, fmt.Errorf("start: %w", err)
	}
	return stdin, stdout, cmd.Wait, nil
}

// Session manages one long-lived Claude Code process. It spawns the
// process on the first Send, reads NDJSON events from stdout, and
// exposes them via Events(). Close kills the process gracefully.
type Session struct {
	cfg        Config
	id         uuid.UUID
	incidentID string
	exec       StreamCommandExecutor
	env        []string

	mu      sync.Mutex
	spawned bool
	resumed bool
	ctx     context.Context
	cancel  context.CancelFunc
	stdin   io.WriteCloser
	events  chan Event
	done    chan struct{}
	closed  bool
	err     error

	useStreamEvents bool
}

// NewSession creates a Session for the given incident. The process is
// not spawned until the first Send call (lazy-spawn).
func NewSession(cfg Config, incidentID string, executor StreamCommandExecutor, env []string) *Session {
	sid := SessionIDFor(incidentID)
	return &Session{
		cfg:        cfg,
		id:         sid,
		incidentID: incidentID,
		exec:       executor,
		env:        env,
		events:     make(chan Event, 128),
		done:       make(chan struct{}),
	}
}

// ID returns the session's UUID.
func (s *Session) ID() uuid.UUID {
	return s.id
}

// IncidentID returns the incident this session is bound to.
func (s *Session) IncidentID() string {
	return s.incidentID
}

// Events returns the channel from which the TUI reads parsed events.
func (s *Session) Events() <-chan Event {
	return s.events
}

// Done returns a channel that closes when the session's reader goroutine exits.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Send writes a user turn to the session's stdin. On the first call it
// spawns (or resumes) the Claude Code process.
func (s *Session) Send(ctx context.Context, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("session closed")
	}

	if !s.spawned {
		if err := s.spawn(ctx); err != nil {
			return err
		}
	}

	data, err := EncodeUserTurn(text)
	if err != nil {
		return fmt.Errorf("encode user turn: %w", err)
	}

	if _, err := s.stdin.Write(data); err != nil {
		return fmt.Errorf("write to stdin: %w", err)
	}
	return nil
}

func (s *Session) spawn(ctx context.Context) error {
	binary := s.cfg.CLICommand
	fields := strings.Fields(binary)
	if len(fields) == 0 {
		return fmt.Errorf("agent_cli_command is empty")
	}
	binPath := fields[0]

	args := BuildSpawnArgs(s.cfg, s.id, s.resumed)
	if len(fields) > 1 {
		args = append(fields[1:], args...)
	}

	spawnCtx, cancel := context.WithCancel(ctx)
	s.ctx = spawnCtx
	s.cancel = cancel

	stdin, stdout, wait, err := s.exec.Start(spawnCtx, binPath, args, s.env)
	if err != nil {
		cancel()
		return fmt.Errorf("spawn session: %w", err)
	}

	s.stdin = stdin
	s.spawned = true

	go s.readLoop(stdout, wait)
	return nil
}

func (s *Session) readLoop(stdout io.ReadCloser, wait func() error) {
	defer close(s.done)
	defer func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		events, err := ParseStreamEvent(scanner.Bytes())
		if err != nil {
			log.Debug("agent.session.readLoop", "parse_error", err)
			continue
		}

		for _, ev := range events {
			if ev.Kind == Init {
				s.useStreamEvents = true
			}

			if s.useStreamEvents && ev.Kind == TextDelta && ev.Consolidated {
				continue
			}

			select {
			case s.events <- ev:
			case <-s.ctx.Done():
				return
			}
		}
	}

	if err := wait(); err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		select {
		case s.events <- Event{Kind: Error, Err: err}:
		case <-s.ctx.Done():
		}
	}
}

// Close gracefully shuts down the session process.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.cancel != nil {
		s.cancel()
	}
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	return nil
}

// SessionManager manages per-incident sessions with LRU eviction.
// When a session is evicted, a new Session value is created on next
// access with resumed=true, avoiding races with the old readLoop.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	evicted  map[string]bool // incidents that were evicted and need --resume
	order    []string        // LRU order: oldest first
	maxLive  int
	cfg      Config
	exec     StreamCommandExecutor
}

// NewSessionManager creates a SessionManager with the given config.
func NewSessionManager(cfg Config, executor StreamCommandExecutor) *SessionManager {
	if executor == nil {
		executor = &execStreamExecutor{}
	}
	max := cfg.MaxSessions
	if max <= 0 {
		max = 3
	}
	return &SessionManager{
		sessions: make(map[string]*Session),
		evicted:  make(map[string]bool),
		maxLive:  max,
		cfg:      cfg,
		exec:     executor,
	}
}

// GetOrCreate returns the session for the given incident, creating one
// if needed. If the live session count exceeds MaxSessions, the oldest
// session is closed and a fresh Session is created on next access with
// resumed=true, avoiding races with the old readLoop goroutine.
func (m *SessionManager) GetOrCreate(incidentID string, env []string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[incidentID]; ok {
		m.touch(incidentID)
		return s
	}

	// Evict oldest if at capacity
	for len(m.order) >= m.maxLive {
		oldest := m.order[0]
		m.order = m.order[1:]
		if s, ok := m.sessions[oldest]; ok {
			_ = s.Close()
			delete(m.sessions, oldest)
			m.evicted[oldest] = true
		}
	}

	s := NewSession(m.cfg, incidentID, m.exec, env)
	if m.evicted[incidentID] {
		s.resumed = true
	}
	m.sessions[incidentID] = s
	m.order = append(m.order, incidentID)
	return s
}

func (m *SessionManager) touch(incidentID string) {
	for i, id := range m.order {
		if id == incidentID {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.order = append(m.order, incidentID)
}

// CloseAll kills all live sessions. Called on srepd exit.
func (m *SessionManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.sessions {
		_ = s.Close()
	}
}
