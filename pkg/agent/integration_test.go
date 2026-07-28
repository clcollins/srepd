package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fakeBinaryPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fakeclaude-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	fakeBinaryPath = filepath.Join(dir, "claude")
	cmd := exec.Command("go", "build", "-o", fakeBinaryPath, "./testdata/fakeclaude")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build fake claude: %s\n%v\n", out, err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

type fakeLog struct {
	Type string   `json:"type"`
	Args []string `json:"args,omitempty"`
	Line string   `json:"line,omitempty"`
}

func readFakeLog(t *testing.T, path string) []fakeLog {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []fakeLog
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry fakeLog
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

func filterArgvEntries(entries []fakeLog) []fakeLog {
	var result []fakeLog
	for _, e := range entries {
		if e.Type == "argv" {
			result = append(result, e)
		}
	}
	return result
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func drainUntilResultOrDone(t *testing.T, s *Session) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				return
			}
			if ev.Kind == Result {
				return
			}
			if ev.Kind == Error {
				t.Fatalf("unexpected Error event: %v", ev.Err)
			}
		case <-s.Done():
			return
		case <-timeout:
			t.Fatal("timed out waiting for Result")
		}
	}
}

// TestIntegration_RestartResume is the headline test (H1a): a new
// SessionManager over the same config dir must use --resume (not
// --session-id) when the index shows an established session.
//
// This test is committed FAILING before the index implementation exists.
// It asserts on the spawn-flag decision, never UUID equality (410a §2).
func TestIntegration_RestartResume(t *testing.T) {
	if fakeBinaryPath == "" {
		t.Skip("fake claude binary not built")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "log.jsonl")
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	sessionDir := filepath.Join(tmpDir, "config", "sessions")

	cfg := Config{
		CLICommand:     fakeBinaryPath,
		SessionEnabled: true,
		MaxSessions:    3,
		SessionDir:     sessionDir,
	}

	env := []string{
		"FAKECLAUDE_LOG=" + logFile,
		"FAKECLAUDE_STATE=" + stateDir,
	}

	// Phase 1: first manager — send a turn, wait for result
	mgr1 := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr1.CloseAll() })
	s1 := mgr1.GetOrCreate("INC-001", env)
	require.NoError(t, s1.Send(context.Background(), "hello"))
	drainUntilResultOrDone(t, s1)
	_ = s1.Close()

	// The index must exist on disk with the entry
	indexPath := filepath.Join(sessionDir, "index.jsonl")
	assert.FileExists(t, indexPath, "index.jsonl must exist after established session")

	// Phase 2: new manager instance, same config dir — demonstrates
	// restart-resume: in-memory state is empty, only the on-disk index
	// tells it to use --resume.
	mgr2 := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr2.CloseAll() })
	s2 := mgr2.GetOrCreate("INC-001", env)
	_ = s2.Send(context.Background(), "world")

	// Give the second process time to start and log its argv
	time.Sleep(500 * time.Millisecond)

	entries := readFakeLog(t, logFile)
	argv := filterArgvEntries(entries)
	require.GreaterOrEqual(t, len(argv), 2,
		"need at least 2 argv entries, got %d", len(argv))

	assert.True(t, hasFlag(argv[0].Args, "--session-id"),
		"first spawn must use --session-id, got: %v", argv[0].Args)
	assert.True(t, hasFlag(argv[1].Args, "--resume"),
		"second spawn (after restart) must use --resume, got: %v", argv[1].Args)
}

// TestIntegration_AbsentIndex_SessionID verifies that a fresh manager
// with no index file uses --session-id (not --resume). This is the
// H1b / L1 regression test.
func TestIntegration_AbsentIndex_SessionID(t *testing.T) {
	if fakeBinaryPath == "" {
		t.Skip("fake claude binary not built")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "log.jsonl")
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	sessionDir := filepath.Join(tmpDir, "config", "sessions")

	cfg := Config{
		CLICommand:     fakeBinaryPath,
		SessionEnabled: true,
		MaxSessions:    3,
		SessionDir:     sessionDir,
	}

	env := []string{
		"FAKECLAUDE_LOG=" + logFile,
		"FAKECLAUDE_STATE=" + stateDir,
	}

	mgr := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr.CloseAll() })
	s := mgr.GetOrCreate("INC-001", env)
	require.NoError(t, s.Send(context.Background(), "hello"))
	drainUntilResultOrDone(t, s)

	entries := readFakeLog(t, logFile)
	argv := filterArgvEntries(entries)
	require.Len(t, argv, 1, "should have exactly 1 argv entry")

	assert.True(t, hasFlag(argv[0].Args, "--session-id"),
		"fresh manager with no index must use --session-id, got: %v", argv[0].Args)
	assert.False(t, hasFlag(argv[0].Args, "--resume"),
		"fresh manager with no index must NOT use --resume, got: %v", argv[0].Args)
}

// TestIntegration_PerIncidentIsolation verifies that two incidents
// get distinct session IDs and do not cross-write.
func TestIntegration_PerIncidentIsolation(t *testing.T) {
	if fakeBinaryPath == "" {
		t.Skip("fake claude binary not built")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "log.jsonl")
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	sessionDir := filepath.Join(tmpDir, "config", "sessions")

	cfg := Config{
		CLICommand:     fakeBinaryPath,
		SessionEnabled: true,
		MaxSessions:    3,
		SessionDir:     sessionDir,
	}

	env := []string{
		"FAKECLAUDE_LOG=" + logFile,
		"FAKECLAUDE_STATE=" + stateDir,
	}

	mgr := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr.CloseAll() })

	s1 := mgr.GetOrCreate("INC-001", env)
	require.NoError(t, s1.Send(context.Background(), "hello from 001"))
	drainUntilResultOrDone(t, s1)

	s2 := mgr.GetOrCreate("INC-002", env)
	require.NoError(t, s2.Send(context.Background(), "hello from 002"))
	drainUntilResultOrDone(t, s2)

	entries := readFakeLog(t, logFile)
	argv := filterArgvEntries(entries)
	require.Len(t, argv, 2, "should have 2 argv entries")

	sid1 := flagValue(argv[0].Args, "--session-id")
	sid2 := flagValue(argv[1].Args, "--session-id")
	require.NotEmpty(t, sid1, "INC-001 should have --session-id")
	require.NotEmpty(t, sid2, "INC-002 should have --session-id")
	assert.NotEqual(t, sid1, sid2,
		"two incidents must produce distinct session IDs")
}

// TestIntegration_CrashBeforeInit verifies L1: a crash before system/init
// leaves no index entry, so the next spawn uses --session-id.
func TestIntegration_CrashBeforeInit(t *testing.T) {
	if fakeBinaryPath == "" {
		t.Skip("fake claude binary not built")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "log.jsonl")
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	sessionDir := filepath.Join(tmpDir, "config", "sessions")

	// Write a script that crashes before init
	scriptFile := filepath.Join(tmpDir, "script.json")
	require.NoError(t, os.WriteFile(scriptFile,
		[]byte(`{"crash_before_init":true}`), 0644))

	cfg := Config{
		CLICommand:     fakeBinaryPath,
		SessionEnabled: true,
		MaxSessions:    3,
		SessionDir:     sessionDir,
	}

	envCrash := []string{
		"FAKECLAUDE_LOG=" + logFile,
		"FAKECLAUDE_STATE=" + stateDir,
		"FAKECLAUDE_SCRIPT=" + scriptFile,
	}

	// First attempt crashes before init
	mgr1 := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr1.CloseAll() })
	s1 := mgr1.GetOrCreate("INC-001", envCrash)
	_ = s1.Send(context.Background(), "hello")
	// Wait for process to exit
	select {
	case <-s1.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for crashed session")
	}

	// No index entry should exist
	indexPath := filepath.Join(sessionDir, "index.jsonl")
	_, err := os.Stat(indexPath)
	noIndex := os.IsNotExist(err)
	if !noIndex {
		data, _ := os.ReadFile(indexPath)
		assert.Empty(t, strings.TrimSpace(string(data)),
			"index.jsonl must be empty after crash-before-init")
	}

	// Second attempt without crash script — should use --session-id
	logFile2 := filepath.Join(tmpDir, "log2.jsonl")
	envNormal := []string{
		"FAKECLAUDE_LOG=" + logFile2,
		"FAKECLAUDE_STATE=" + stateDir,
	}
	mgr2 := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr2.CloseAll() })
	s2 := mgr2.GetOrCreate("INC-001", envNormal)
	require.NoError(t, s2.Send(context.Background(), "hello again"))
	drainUntilResultOrDone(t, s2)

	entries := readFakeLog(t, logFile2)
	argv := filterArgvEntries(entries)
	require.Len(t, argv, 1)
	assert.True(t, hasFlag(argv[0].Args, "--session-id"),
		"after crash-before-init, next spawn must use --session-id, got: %v",
		argv[0].Args)
}

// TestIntegration_DuplicateSessionID verifies V1: when the fake exits
// with code 1 and no result line (duplicate session-id), an Error event
// is surfaced and no hang occurs.
func TestIntegration_DuplicateSessionID(t *testing.T) {
	if fakeBinaryPath == "" {
		t.Skip("fake claude binary not built")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "log.jsonl")
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))

	cfg := Config{
		CLICommand:     fakeBinaryPath,
		SessionEnabled: true,
		MaxSessions:    3,
	}

	sid := SessionIDFor("INC-001").String()

	// Pre-create the state file to simulate an already-used session ID
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, sid), []byte("used"), 0644))

	env := []string{
		"FAKECLAUDE_LOG=" + logFile,
		"FAKECLAUDE_STATE=" + stateDir,
	}

	mgr := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr.CloseAll() })
	s := mgr.GetOrCreate("INC-001", env)

	// Send will spawn, the fake will exit 1 with no result
	_ = s.Send(context.Background(), "hello")

	// Must receive an Error event (not hang)
	timeout := time.After(5 * time.Second)
	var sawError bool
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				goto done
			}
			if ev.Kind == Error {
				sawError = true
				goto done
			}
		case <-s.Done():
			goto done
		case <-timeout:
			t.Fatal("timed out — code waiting for result line would hang here")
		}
	}
done:
	assert.True(t, sawError, "duplicate session-id must surface an Error event")
}

// TestIntegration_LRUEvictionResume verifies that an evicted incident's
// next send uses --resume.
func TestIntegration_LRUEvictionResume(t *testing.T) {
	if fakeBinaryPath == "" {
		t.Skip("fake claude binary not built")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "log.jsonl")
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))
	sessionDir := filepath.Join(tmpDir, "config", "sessions")

	cfg := Config{
		CLICommand:     fakeBinaryPath,
		SessionEnabled: true,
		MaxSessions:    2,
		SessionDir:     sessionDir,
	}

	env := []string{
		"FAKECLAUDE_LOG=" + logFile,
		"FAKECLAUDE_STATE=" + stateDir,
	}

	mgr := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr.CloseAll() })

	// Fill to capacity
	s1 := mgr.GetOrCreate("INC-001", env)
	require.NoError(t, s1.Send(context.Background(), "hi 001"))
	drainUntilResultOrDone(t, s1)

	s2 := mgr.GetOrCreate("INC-002", env)
	require.NoError(t, s2.Send(context.Background(), "hi 002"))
	drainUntilResultOrDone(t, s2)

	// Third evicts INC-001
	s3 := mgr.GetOrCreate("INC-003", env)
	require.NoError(t, s3.Send(context.Background(), "hi 003"))
	drainUntilResultOrDone(t, s3)

	// Re-access INC-001 — should use --resume
	s1Again := mgr.GetOrCreate("INC-001", env)
	require.NoError(t, s1Again.Send(context.Background(), "back to 001"))
	drainUntilResultOrDone(t, s1Again)

	entries := readFakeLog(t, logFile)
	argv := filterArgvEntries(entries)
	require.GreaterOrEqual(t, len(argv), 4)

	// Fourth spawn (INC-001 re-access) must use --resume
	assert.True(t, hasFlag(argv[3].Args, "--resume"),
		"evicted INC-001 must use --resume on re-access, got: %v", argv[3].Args)
}

// TestIntegration_SendHonorsTimeout verifies L2: Send returns on
// timeout when the child process is hung.
func TestIntegration_SendHonorsTimeout(t *testing.T) {
	if fakeBinaryPath == "" {
		t.Skip("fake claude binary not built")
	}

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")
	require.NoError(t, os.MkdirAll(stateDir, 0755))

	// The fake binary will emit init then wait for stdin. We send with a
	// very short timeout so the second send times out.
	cfg := Config{
		CLICommand:     fakeBinaryPath,
		SessionEnabled: true,
		MaxSessions:    3,
	}

	env := []string{
		"FAKECLAUDE_STATE=" + stateDir,
	}

	mgr := NewSessionManager(cfg, nil)
	t.Cleanup(func() { mgr.CloseAll() })
	s := mgr.GetOrCreate("INC-001", env)

	// First send succeeds (spawns process)
	require.NoError(t, s.Send(context.Background(), "hello"))

	// Drain the init event
	timeout := time.After(5 * time.Second)
	select {
	case <-s.Events():
	case <-timeout:
		t.Fatal("timed out waiting for init")
	}

	// Second send with an already-cancelled context must return promptly
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Send(ctx, "should fail")
	assert.Error(t, err, "Send with cancelled context must return error")
}

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
