package agent

import (
	"bufio"
	"encoding/json"
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
	defer func() { _ = f.Close() }()

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

	if err := scanner.Err(); err != nil {
		charlog.Warn("agent.index.load", "msg", "scanner error", "error", err)
	}

	if lastLine != nil {
		charlog.Warn("agent.index.load",
			"msg", "corrupt trailing line ignored",
			"line", lineNum,
			"len", len(lastLine))
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
	if _, exists := idx.established[incidentID]; exists {
		idx.mu.Unlock()
		return
	}
	idx.established[incidentID] = sessionID
	path := idx.path
	warned := idx.warned
	idx.mu.Unlock()

	if path == "" {
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		idx.mu.Lock()
		if !idx.warned {
			charlog.Warn("agent.index.record", "msg", "cannot create session dir", "error", err)
			idx.warned = true
		}
		idx.mu.Unlock()
		return
	}

	entry := sessionEntry{
		IncidentID: incidentID,
		SessionID:  sessionID.String(),
		Created:    time.Now(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		if !warned {
			idx.mu.Lock()
			if !idx.warned {
				charlog.Warn("agent.index.record", "msg", "cannot write index", "error", err)
				idx.warned = true
			}
			idx.mu.Unlock()
		}
		return
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(data, '\n')); err != nil {
		charlog.Warn("agent.index.record", "msg", "write failed", "error", err)
	}
}
