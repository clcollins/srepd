package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// EventKind classifies the type of parsed stream event.
type EventKind int

const (
	Init EventKind = iota
	TextDelta
	ToolUse
	ToolResult
	PermissionAsk
	Result
	Error
)

// Event is the parsed representation of one semantic unit from the
// Claude Code stream-json NDJSON output. One input line can yield
// multiple Events (e.g. an assistant message with both text and
// tool_use content blocks).
type Event struct {
	Kind      EventKind
	Text      string
	Tool      string
	ToolInput string
	SessionID string
	IsError   bool
	Err       error
}

// sessionNamespace is a fixed UUID used as the namespace for
// deterministic UUIDv5 session IDs: uuid.NewSHA1(sessionNamespace, []byte("srepd/"+incidentID)).
var sessionNamespace = uuid.MustParse("a3b2c1d0-e4f5-6789-abcd-ef0123456789")

// SessionIDFor returns a deterministic UUIDv5 session ID for the given
// incident ID. The same incidentID always produces the same UUID.
func SessionIDFor(incidentID string) uuid.UUID {
	return uuid.NewSHA1(sessionNamespace, []byte("srepd/"+incidentID))
}

// ParseStreamEvent parses a single NDJSON line from Claude Code's
// stream-json output and returns zero or more Events.
// One input line can yield multiple Events when the message contains
// multiple content blocks (e.g. text + tool_use).
func ParseStreamEvent(line []byte) ([]Event, error) {
	// stub — tests will fail
	return nil, fmt.Errorf("not implemented")
}

// EncodeUserTurn produces the NDJSON line (newline-terminated) that
// sends a user text message to a Claude Code stream-json process.
func EncodeUserTurn(text string) ([]byte, error) {
	// stub — tests will fail
	return nil, fmt.Errorf("not implemented")
}

// Config holds the configuration for spawning a Claude Code session.
type Config struct {
	CLICommand     string
	SessionEnabled bool
	MaxSessions    int
	AllowedTools   []string
	PermissionMode string
}

// BuildSpawnArgs produces the argument list for exec.CommandContext
// to spawn or resume a Claude Code session.
// Never includes --bare (srepd needs the host's skills/MCPs to load).
// Tested version: claude 2.1.220.
// TODO: when --bare becomes the default for -p, pass the future
// opt-in-to-full-loading flag. Detect via claude --version.
func BuildSpawnArgs(cfg Config, sessionID uuid.UUID, resume bool) []string {
	// stub — tests will fail
	return nil
}

// summarizeToolInput returns a compact one-line summary of tool input JSON.
func summarizeToolInput(input json.RawMessage) string {
	var m map[string]interface{}
	if err := json.Unmarshal(input, &m); err != nil {
		return string(input)
	}
	if desc, ok := m["description"].(string); ok && desc != "" {
		return desc
	}
	if cmd, ok := m["command"].(string); ok && cmd != "" {
		if len(cmd) > 80 {
			return cmd[:80] + "..."
		}
		return cmd
	}
	if fp, ok := m["file_path"].(string); ok && fp != "" {
		return fp
	}
	s := string(input)
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}

// SessionEntry is one record in the JSONL session index.
type SessionEntry struct {
	IncidentID string `json:"incident_id"`
	SessionID  string `json:"session_id"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at"`
}

// EncodeSessionEntry marshals a SessionEntry to a newline-terminated JSON line.
func EncodeSessionEntry(e SessionEntry) ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeSessionIndex parses a JSONL session index, tolerating a
// corrupt trailing line (truncate-and-warn pattern).
func DecodeSessionIndex(data []byte) ([]SessionEntry, error) {
	var entries []SessionEntry
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e SessionEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			if i == len(lines)-1 || (i == len(lines)-2 && strings.TrimSpace(lines[len(lines)-1]) == "") {
				// Corrupt trailing line — tolerate it
				break
			}
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// SessionIndexPath returns the path to the session index JSONL file,
// respecting XDG_CONFIG_HOME.
func SessionIndexPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "srepd", "sessions", "index.jsonl")
}
