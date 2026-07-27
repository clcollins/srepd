package agent

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- ParseStreamEvent ----------

func loadFixture(t *testing.T, name string) [][]byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	var lines [][]byte
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, []byte(line))
		}
	}
	return lines
}

func TestParseStreamEvent_InitLine(t *testing.T) {
	lines := loadFixture(t, "raw-capture.ndjson")
	require.True(t, len(lines) >= 1, "fixture must have at least 1 line")

	events, err := ParseStreamEvent(lines[0])
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, Init, events[0].Kind)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", events[0].SessionID)
}

func TestParseStreamEvent_TextDelta(t *testing.T) {
	// Line 2 (index 1) in raw-capture: assistant with text "PONG."
	lines := loadFixture(t, "raw-capture.ndjson")
	require.True(t, len(lines) >= 2)

	events, err := ParseStreamEvent(lines[1])
	require.NoError(t, err)
	require.True(t, len(events) >= 1, "should produce at least one event")
	assert.Equal(t, TextDelta, events[0].Kind)
	assert.Equal(t, "PONG.", events[0].Text)
}

func TestParseStreamEvent_ToolUse(t *testing.T) {
	// Line 3 (index 2) in raw-capture: assistant with tool_use block
	lines := loadFixture(t, "raw-capture.ndjson")
	require.True(t, len(lines) >= 3)

	events, err := ParseStreamEvent(lines[2])
	require.NoError(t, err)
	require.True(t, len(events) >= 1, "should produce at least one ToolUse event")

	var toolEvent *Event
	for i := range events {
		if events[i].Kind == ToolUse {
			toolEvent = &events[i]
			break
		}
	}
	require.NotNil(t, toolEvent, "must find a ToolUse event")
	assert.Equal(t, "Bash", toolEvent.Tool)
	assert.NotEmpty(t, toolEvent.ToolInput, "tool input summary should not be empty")
}

func TestParseStreamEvent_ToolResult(t *testing.T) {
	// Line 4 (index 3) in raw-capture: user with tool_result
	lines := loadFixture(t, "raw-capture.ndjson")
	require.True(t, len(lines) >= 4)

	events, err := ParseStreamEvent(lines[3])
	require.NoError(t, err)
	require.True(t, len(events) >= 1, "should produce at least one ToolResult event")

	var resultEvent *Event
	for i := range events {
		if events[i].Kind == ToolResult {
			resultEvent = &events[i]
			break
		}
	}
	require.NotNil(t, resultEvent, "must find a ToolResult event")
	assert.False(t, resultEvent.IsError)
}

func TestParseStreamEvent_ResultSuccess(t *testing.T) {
	// Last line in raw-capture: result/success
	lines := loadFixture(t, "raw-capture.ndjson")
	lastLine := lines[len(lines)-1]

	events, err := ParseStreamEvent(lastLine)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, Result, events[0].Kind)
	assert.False(t, events[0].IsError)
	assert.Contains(t, events[0].Text, "hello")
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", events[0].SessionID)
}

// Hand-authored fixture: result with error subtype
func TestParseStreamEvent_ResultError(t *testing.T) {
	line := `{"type":"result","subtype":"error","is_error":true,"session_id":"00000000-0000-0000-0000-000000000000","result":"something went wrong"}`
	events, err := ParseStreamEvent([]byte(line))
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, Result, events[0].Kind)
	assert.True(t, events[0].IsError)
}

func TestParseStreamEvent_StreamEventTextDelta(t *testing.T) {
	// From partial-capture: content_block_delta with text_delta
	lines := loadFixture(t, "partial-capture.ndjson")

	var found bool
	for _, line := range lines {
		events, err := ParseStreamEvent(line)
		if err != nil {
			continue
		}
		for _, ev := range events {
			if ev.Kind == TextDelta && ev.Text == "P" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	assert.True(t, found, "should find text_delta 'P' from stream_event")
}

func TestParseStreamEvent_MalformedJSON(t *testing.T) {
	_, err := ParseStreamEvent([]byte(`{this is not json}`))
	assert.Error(t, err)
}

func TestParseStreamEvent_OversizedLine(t *testing.T) {
	// >256KiB line — should still parse (or error gracefully)
	big := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"` +
		strings.Repeat("x", 300*1024) + `"}]}}`
	events, err := ParseStreamEvent([]byte(big))
	require.NoError(t, err)
	require.True(t, len(events) >= 1)
	assert.Equal(t, TextDelta, events[0].Kind)
}

func TestParseStreamEvent_UnknownTypeIgnored(t *testing.T) {
	line := `{"type":"unknown_future_type","data":"stuff"}`
	events, err := ParseStreamEvent([]byte(line))
	require.NoError(t, err)
	assert.Empty(t, events, "unknown types should be silently ignored")
}

func TestParseStreamEvent_IgnoredSystemSubtypes(t *testing.T) {
	for _, subtype := range []string{"hook_started", "hook_response", "status"} {
		line := `{"type":"system","subtype":"` + subtype + `"}`
		events, err := ParseStreamEvent([]byte(line))
		require.NoError(t, err, "subtype %s", subtype)
		assert.Empty(t, events, "system/%s should be ignored", subtype)
	}
}

func TestParseStreamEvent_RateLimitIgnored(t *testing.T) {
	line := `{"type":"rate_limit_event","data":"throttled"}`
	events, err := ParseStreamEvent([]byte(line))
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestParseStreamEvent_DoubleRenderPrevention(t *testing.T) {
	// With --include-partial-messages, you get BOTH stream_event deltas AND
	// a consolidated assistant message. We use stream_event for live streaming
	// and skip the consolidated assistant text blocks when partial messages
	// are in use (detected by the presence of stream_event lines).
	//
	// The consolidated assistant line should still be parseable (returns
	// TextDelta) — the caller decides which path to use.
	lines := loadFixture(t, "partial-capture.ndjson")

	var streamDeltas []Event
	var consolidatedTexts []Event
	for _, line := range lines {
		events, _ := ParseStreamEvent(line)
		for _, ev := range events {
			if ev.Kind == TextDelta {
				// Determine source: check if raw line has "stream_event"
				if strings.Contains(string(line), `"type":"stream_event"`) {
					streamDeltas = append(streamDeltas, ev)
				} else {
					consolidatedTexts = append(consolidatedTexts, ev)
				}
			}
		}
	}
	assert.NotEmpty(t, streamDeltas, "should have stream_event deltas")
	assert.NotEmpty(t, consolidatedTexts, "should have consolidated assistant text")
}

// ---------- EncodeUserTurn ----------

func TestEncodeUserTurn_Shape(t *testing.T) {
	data, err := EncodeUserTurn("hello world")
	require.NoError(t, err)
	assert.True(t, data[len(data)-1] == '\n', "must be newline-terminated")

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data[:len(data)-1], &parsed))

	assert.Equal(t, "user", parsed["type"])
	msg, ok := parsed["message"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "user", msg["role"])
	content, ok := msg["content"].([]interface{})
	require.True(t, ok)
	require.Len(t, content, 1)
	block, ok := content[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text", block["type"])
	assert.Equal(t, "hello world", block["text"])
}

func TestEncodeUserTurn_NoControlChars(t *testing.T) {
	data, err := EncodeUserTurn("line1\nline2\ttab")
	require.NoError(t, err)
	// JSON encoding escapes control characters
	jsonPart := string(data[:len(data)-1])
	assert.NotContains(t, jsonPart, "\n", "newlines within JSON should be escaped")
	assert.NotContains(t, jsonPart, "\t", "tabs within JSON should be escaped")
}

func TestEncodeUserTurn_EmptyText(t *testing.T) {
	data, err := EncodeUserTurn("")
	require.NoError(t, err)
	assert.True(t, len(data) > 0)
}

// ---------- BuildSpawnArgs ----------

func TestBuildSpawnArgs_NewSession(t *testing.T) {
	cfg := Config{
		CLICommand:     "claude",
		SessionEnabled: true,
	}
	sid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	args := BuildSpawnArgs(cfg, sid, false)

	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "--input-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "--verbose")
	assert.Contains(t, args, "--include-partial-messages")
	assert.Contains(t, args, "--session-id")
	assert.Contains(t, args, sid.String())
	assert.NotContains(t, args, "--resume")
}

func TestBuildSpawnArgs_Resume(t *testing.T) {
	cfg := Config{
		CLICommand:     "claude",
		SessionEnabled: true,
	}
	sid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	args := BuildSpawnArgs(cfg, sid, true)

	assert.Contains(t, args, "--resume")
	assert.Contains(t, args, sid.String())
}

func TestBuildSpawnArgs_NeverBare(t *testing.T) {
	// This test is a hard acceptance criterion — --bare must never appear.
	testCases := []struct {
		name   string
		cfg    Config
		resume bool
	}{
		{
			name:   "new session",
			cfg:    Config{CLICommand: "claude", SessionEnabled: true},
			resume: false,
		},
		{
			name:   "resume session",
			cfg:    Config{CLICommand: "claude", SessionEnabled: true},
			resume: true,
		},
		{
			name: "with allowed tools",
			cfg: Config{
				CLICommand:     "claude",
				SessionEnabled: true,
				AllowedTools:   []string{"Bash(echo *)"},
			},
			resume: false,
		},
		{
			name: "with permission mode",
			cfg: Config{
				CLICommand:     "claude",
				SessionEnabled: true,
				PermissionMode: "acceptEdits",
			},
			resume: false,
		},
		{
			name: "custom binary path",
			cfg: Config{
				CLICommand:     "/usr/local/bin/claude",
				SessionEnabled: true,
			},
			resume: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
			args := BuildSpawnArgs(tc.cfg, sid, tc.resume)
			for _, arg := range args {
				assert.NotEqual(t, "--bare", arg, "--bare must never be present")
			}
		})
	}
}

func TestBuildSpawnArgs_AllowedTools(t *testing.T) {
	cfg := Config{
		CLICommand:     "claude",
		SessionEnabled: true,
		AllowedTools:   []string{"Bash(echo *)", "Read"},
	}
	sid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	args := BuildSpawnArgs(cfg, sid, false)

	assert.Contains(t, args, "--allowedTools")
	// Each tool should be its own --allowedTools arg
	toolArgCount := 0
	for _, a := range args {
		if a == "--allowedTools" {
			toolArgCount++
		}
	}
	assert.Equal(t, 2, toolArgCount, "each allowed tool gets its own --allowedTools flag")
}

func TestBuildSpawnArgs_PermissionMode(t *testing.T) {
	cfg := Config{
		CLICommand:     "claude",
		SessionEnabled: true,
		PermissionMode: "acceptEdits",
	}
	sid := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	args := BuildSpawnArgs(cfg, sid, false)

	assert.Contains(t, args, "--permission-mode")
	assert.Contains(t, args, "acceptEdits")
}

func TestBuildSpawnArgs_NoAllowedToolsWhenEmpty(t *testing.T) {
	cfg := Config{
		CLICommand:     "claude",
		SessionEnabled: true,
		AllowedTools:   []string{},
	}
	sid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	args := BuildSpawnArgs(cfg, sid, false)

	assert.NotContains(t, args, "--allowedTools")
}

func TestBuildSpawnArgs_NoPermissionModeWhenEmpty(t *testing.T) {
	cfg := Config{
		CLICommand:     "claude",
		SessionEnabled: true,
		PermissionMode: "",
	}
	sid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	args := BuildSpawnArgs(cfg, sid, false)

	assert.NotContains(t, args, "--permission-mode")
}

func TestBuildSpawnArgs_CustomBinaryPath(t *testing.T) {
	cfg := Config{
		CLICommand:     "/opt/claude/bin/claude",
		SessionEnabled: true,
	}
	sid := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	args := BuildSpawnArgs(cfg, sid, false)

	// The binary path itself is NOT in args — it's used as the
	// exec.CommandContext name parameter, not an arg.
	// Args should contain -p and streaming flags.
	assert.Contains(t, args, "-p")
}

// ---------- SessionIDFor ----------

func TestSessionIDFor_Deterministic(t *testing.T) {
	id1 := SessionIDFor("INC-001")
	id2 := SessionIDFor("INC-001")
	assert.Equal(t, id1, id2, "same incident ID must produce same session UUID")
}

func TestSessionIDFor_DistinctPerIncident(t *testing.T) {
	id1 := SessionIDFor("INC-001")
	id2 := SessionIDFor("INC-002")
	assert.NotEqual(t, id1, id2, "different incidents must produce different UUIDs")
}

func TestSessionIDFor_StableAcrossRuns(t *testing.T) {
	// The UUID is deterministic UUIDv5 — verify the version bits
	id := SessionIDFor("test-incident")
	assert.Equal(t, uuid.Version(5), id.Version(), "must be UUIDv5")
}

func TestSessionIDFor_QueueSession(t *testing.T) {
	// Empty incident ID = "queue" session
	id := SessionIDFor("")
	assert.NotEqual(t, uuid.Nil, id)
}

// ---------- Session index JSONL ----------

func TestSessionEntry_RoundTrip(t *testing.T) {
	entry := SessionEntry{
		IncidentID: "INC-100",
		SessionID:  "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		CreatedAt:  "2026-07-27T10:00:00Z",
		LastUsedAt: "2026-07-27T10:05:00Z",
	}
	data, err := EncodeSessionEntry(entry)
	require.NoError(t, err)
	assert.True(t, data[len(data)-1] == '\n')

	entries, err := DecodeSessionIndex(data)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, entry, entries[0])
}

func TestSessionIndex_MultipleEntries(t *testing.T) {
	e1 := SessionEntry{IncidentID: "INC-1", SessionID: "a", CreatedAt: "t1", LastUsedAt: "t1"}
	e2 := SessionEntry{IncidentID: "INC-2", SessionID: "b", CreatedAt: "t2", LastUsedAt: "t2"}
	d1, _ := EncodeSessionEntry(e1)
	d2, _ := EncodeSessionEntry(e2)
	data := append(d1, d2...)

	entries, err := DecodeSessionIndex(data)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "INC-1", entries[0].IncidentID)
	assert.Equal(t, "INC-2", entries[1].IncidentID)
}

func TestSessionIndex_CorruptTrailingLine(t *testing.T) {
	entry := SessionEntry{IncidentID: "INC-1", SessionID: "a", CreatedAt: "t", LastUsedAt: "t"}
	data, _ := EncodeSessionEntry(entry)
	data = append(data, []byte(`{corrupt trailing`)...)

	entries, err := DecodeSessionIndex(data)
	require.NoError(t, err, "corrupt trailing line should be tolerated")
	require.Len(t, entries, 1)
	assert.Equal(t, "INC-1", entries[0].IncidentID)
}

func TestSessionIndex_CorruptMiddleLine(t *testing.T) {
	e1 := SessionEntry{IncidentID: "INC-1", SessionID: "a", CreatedAt: "t", LastUsedAt: "t"}
	e2 := SessionEntry{IncidentID: "INC-2", SessionID: "b", CreatedAt: "t", LastUsedAt: "t"}
	d1, _ := EncodeSessionEntry(e1)
	d2, _ := EncodeSessionEntry(e2)
	data := append(d1, []byte("{corrupt}\n")...)
	data = append(data, d2...)

	_, err := DecodeSessionIndex(data)
	assert.Error(t, err, "corrupt non-trailing line should error")
}

func TestSessionIndex_EmptyData(t *testing.T) {
	entries, err := DecodeSessionIndex([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// ---------- summarizeToolInput ----------

func TestSummarizeToolInput_Command(t *testing.T) {
	input := json.RawMessage(`{"command":"echo hello","description":"Print hello"}`)
	s := summarizeToolInput(input)
	assert.Equal(t, "Print hello", s, "should prefer description over command")
}

func TestSummarizeToolInput_FilePath(t *testing.T) {
	input := json.RawMessage(`{"file_path":"/tmp/test.go"}`)
	s := summarizeToolInput(input)
	assert.Equal(t, "/tmp/test.go", s)
}

func TestSummarizeToolInput_LongCommand(t *testing.T) {
	long := strings.Repeat("x", 100)
	input := json.RawMessage(`{"command":"` + long + `"}`)
	s := summarizeToolInput(input)
	assert.LessOrEqual(t, len(s), 84, "should truncate long commands")
}
