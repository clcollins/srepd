package agent

import (
	"encoding/json"

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
	Kind         EventKind
	Text         string
	Tool         string
	ToolInput    string
	SessionID    string
	IsError      bool
	Err          error
	Consolidated bool // true for TextDelta from a consolidated assistant message (not stream_event)
}

// sessionNamespace is a fixed UUID used as the namespace for
// deterministic UUIDv5 session IDs: uuid.NewSHA1(sessionNamespace, []byte("srepd/"+incidentID)).
var sessionNamespace = uuid.MustParse("a3b2c1d0-e4f5-6789-abcd-ef0123456789")

// SessionIDFor returns a deterministic UUIDv5 session ID for the given
// incident ID. The same incidentID always produces the same UUID.
func SessionIDFor(incidentID string) uuid.UUID {
	return uuid.NewSHA1(sessionNamespace, []byte("srepd/"+incidentID))
}

// streamLine is the top-level NDJSON object from Claude Code stream-json.
type streamLine struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Message   *streamMessage  `json:"message,omitempty"`
	Event     *streamEventObj `json:"event,omitempty"`
	ResultStr string          `json:"result,omitempty"`
	IsErr     bool            `json:"is_error,omitempty"`
}

type streamMessage struct {
	Role    string               `json:"role"`
	Content []streamContentBlock `json:"content"`
}

type streamContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
}

type streamEventObj struct {
	Type  string          `json:"type"`
	Delta *streamDeltaObj `json:"delta,omitempty"`
}

type streamDeltaObj struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ParseStreamEvent parses a single NDJSON line from Claude Code's
// stream-json output and returns zero or more Events.
// One input line can yield multiple Events when the message contains
// multiple content blocks (e.g. text + tool_use).
//
// Deviation from plan 411: the plan specified a single-Event return
// signature. The verified wire format shows that one input line
// (type:"assistant") can contain multiple content blocks, so a
// []Event return is the honest signature.
func ParseStreamEvent(line []byte) ([]Event, error) {
	var sl streamLine
	if err := json.Unmarshal(line, &sl); err != nil {
		return nil, err
	}

	switch sl.Type {
	case "system":
		if sl.Subtype == "init" {
			return []Event{{Kind: Init, SessionID: sl.SessionID}}, nil
		}
		// hook_started, hook_response, status — ignore
		return nil, nil

	case "assistant":
		return parseMessageBlocks(sl.Message, false, true), nil

	case "user":
		return parseMessageBlocks(sl.Message, true, false), nil

	case "result":
		return []Event{{
			Kind:      Result,
			Text:      sl.ResultStr,
			SessionID: sl.SessionID,
			IsError:   sl.IsErr,
		}}, nil

	case "stream_event":
		if sl.Event != nil && sl.Event.Type == "content_block_delta" &&
			sl.Event.Delta != nil && sl.Event.Delta.Type == "text_delta" &&
			sl.Event.Delta.Text != "" {
			return []Event{{Kind: TextDelta, Text: sl.Event.Delta.Text}}, nil
		}
		return nil, nil

	case "rate_limit_event":
		return nil, nil

	default:
		// Unknown type — silently ignore for forward compatibility
		return nil, nil
	}
}

func parseMessageBlocks(msg *streamMessage, isUser bool, consolidated bool) []Event {
	if msg == nil {
		return nil
	}
	var events []Event
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			events = append(events, Event{Kind: TextDelta, Text: block.Text, Consolidated: consolidated})
		case "tool_use":
			events = append(events, Event{
				Kind:      ToolUse,
				Tool:      block.Name,
				ToolInput: summarizeToolInput(block.Input),
			})
		case "tool_result":
			events = append(events, Event{
				Kind:    ToolResult,
				Text:    block.Content,
				IsError: block.IsError,
			})
		}
	}
	return events
}

// userTurn is the NDJSON schema for sending a user message to a
// Claude Code stream-json process. Verified against claude 2.1.220.
type userTurn struct {
	Type    string          `json:"type"`
	Message userTurnMessage `json:"message"`
}

type userTurnMessage struct {
	Role    string            `json:"role"`
	Content []userTurnContent `json:"content"`
}

type userTurnContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// EncodeUserTurn produces the NDJSON line (newline-terminated) that
// sends a user text message to a Claude Code stream-json process.
func EncodeUserTurn(text string) ([]byte, error) {
	turn := userTurn{
		Type: "user",
		Message: userTurnMessage{
			Role: "user",
			Content: []userTurnContent{
				{Type: "text", Text: text},
			},
		},
	}
	data, err := json.Marshal(turn)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
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
	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
	}

	if resume {
		args = append(args, "--resume", sessionID.String())
	} else {
		args = append(args, "--session-id", sessionID.String())
	}

	for _, tool := range cfg.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	if cfg.PermissionMode != "" {
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}

	return args
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
