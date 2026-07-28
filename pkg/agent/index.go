package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	charlog "github.com/charmbracelet/log"
	"github.com/google/uuid"
)

type sessionEntry struct {
	IncidentID string    `json:"incident_id"`
	SessionID  string    `json:"session_id"`
	Created    time.Time `json:"created"`
	LastUsed   time.Time `json:"last_used"`
}

type sessionIndex struct {
	mu          sync.Mutex
	established map[string]uuid.UUID // incidentID -> sessionID
	path        string               // path to index.jsonl
	warned      bool                 // suppress repeated I/O warnings
}

func newSessionIndex(sessionDir string) *sessionIndex {
	idx := &sessionIndex{
		established: make(map[string]uuid.UUID),
	}
	if sessionDir == "" {
		return idx
	}
	idx.path = filepath.Join(sessionDir, "index.jsonl")
	idx.load()
	return idx
}

func (idx *sessionIndex) load() {
	if idx.path == "" {
		return
	}

	f, err := os.Open(idx.path)
	if err != nil {
		if !os.IsNotExist(err) {
			charlog.Warn("agent.index.load", "error", err)
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lastLine []byte
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		var entry sessionEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			lastLine = append([]byte{}, line...)
			continue
		}
		lastLine = nil
		sid, parseErr := uuid.Parse(entry.SessionID)
		if parseErr != nil {
			charlog.Warn("agent.index.load", "line", lineNum, "error", parseErr)
			continue
		}
		idx.established[entry.IncidentID] = sid
	}

	if lastLine != nil {
		charlog.Warn("agent.index.load",
			"msg", "corrupt trailing line truncated",
			"line", lineNum,
			"content", string(lastLine))
	}
}

func (idx *sessionIndex) has(incidentID string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	_, ok := idx.established[incidentID]
	return ok
}

func (idx *sessionIndex) record(incidentID string, sessionID uuid.UUID) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.established[incidentID] = sessionID

	if idx.path == "" {
		return
	}

	dir := filepath.Dir(idx.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		if !idx.warned {
			charlog.Warn("agent.index.record", "msg", "cannot create session dir", "error", err)
			idx.warned = true
		}
		return
	}

	entry := sessionEntry{
		IncidentID: incidentID,
		SessionID:  sessionID.String(),
		Created:    time.Now(),
		LastUsed:   time.Now(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(idx.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		if !idx.warned {
			charlog.Warn("agent.index.record", "msg", "cannot write index", "error", err)
			idx.warned = true
		}
		return
	}
	defer f.Close()

	_, _ = f.Write(data)
	_, _ = f.Write([]byte("\n"))
}

// SessionIndexDir computes the default session index directory from
// XDG_CONFIG_HOME (or ~/.config) + srepd/sessions.
func SessionIndexDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "srepd", "sessions")
}

// SessionIndexPath returns the full path to the session index file.
func SessionIndexPath() string {
	d := SessionIndexDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "index.jsonl")
}

// IndexEntryCount returns the number of established sessions in the
// index. Exposed for testing only — the decision logic is internal.
func (m *SessionManager) IndexEntryCount() int {
	if m.index == nil {
		return 0
	}
	m.index.mu.Lock()
	defer m.index.mu.Unlock()
	return len(m.index.established)
}

// ForTestStubIndexWrite disables index file writes by clearing the path.
// This is used only for the 410a §2c revert check.
func (m *SessionManager) ForTestStubIndexWrite() {
	if m.index != nil {
		m.index.path = fmt.Sprintf("/dev/null/nonexistent/%d", time.Now().UnixNano())
	}
}
