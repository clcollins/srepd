# Plan 411: Persistent per-incident agent sessions

**Branch:** `srepd/ai-p1-agent-session`

## Problem

The `:agent` command invokes Claude Code as a one-shot subprocess: each query
spawns a new process, loses conversational context, and cannot use tools
across turns. The user must re-explain incident context every query, and the
agent cannot build on prior investigation. Tool-use activity (Bash, Read, etc.)
is invisible to the user during execution.

## Solution

### New package: `pkg/agent/`

A standalone package that owns the Claude Code stream-json protocol and
session lifecycle. No dependency on `pkg/tui/` or `pkg/ai/`.

#### Stream protocol parsing (`ParseStreamEvent`)

Parses Claude Code NDJSON (stream-json) output into typed `Event` values.
Returns `[]Event` per line because a single `type:"assistant"` message can
contain multiple content blocks (text + tool_use). Handles:

- `system/init` — session established
- `assistant` with `text` blocks — `TextDelta` events
- `assistant` with `tool_use` blocks — `ToolUse` events with tool name and
  compact input summary
- `user` with `tool_result` blocks — `ToolResult` events
- `result/success` and `result/error` — `Result` events
- `stream_event` — live `text_delta` streaming

Double-render prevention: when `--include-partial-messages` is used, both
`stream_event` deltas and consolidated `assistant` messages arrive. The parser
returns both; the session reader tracks which path is active.

#### Deterministic session IDs (`SessionIDFor`)

Uses UUIDv5 with a fixed namespace to derive a deterministic session ID from
the incident ID. Same incident always maps to the same session, enabling
`--resume` across process restarts.

#### Process lifecycle (`Session`, `SessionManager`)

`Session` wraps a long-lived Claude Code process with lazy spawn on first
`Send()`. The `SessionManager` manages per-incident sessions with LRU eviction
when `agent_max_sessions` is exceeded. Evicted sessions are marked resumable
and re-spawned with `--resume` on next access.

`StreamCommandExecutor` interface abstracts `os/exec` for test injection.

#### Spawn args (`BuildSpawnArgs`)

Builds the CLI argument list: `--output-format stream-json`,
`--session-id <uuid>`, and optionally `--resume`, `--allowedTools`,
`--permission-mode`. Never includes `--bare`.

### TUI integration

`handleClaudePrompt` gains a session path that activates when
`agent_session_enabled` is true and the CLI is detected as Claude Code.
Session events flow through the Bubble Tea Update loop:

- `agentSessionEventMsg` dispatched to `handleAgentSessionEvent`
- `ToolUse` events render as dim `⚙ <tool> <input-summary>` lines in the
  watcher pane (headline bug fix — tool activity was previously invisible)
- `TextDelta` events stream text into the watcher buffer
- `Result` events apply glamour markdown rendering
- `PermissionAsk` events surface a status line warning
- Legacy one-shot and blocking paths preserved as fallbacks

### Config keys

| Key | Default | Description |
|-----|---------|-------------|
| `agent_session_enabled` | `true` | Use persistent sessions |
| `agent_max_sessions` | `3` | Max live processes before LRU eviction |
| `agent_allowed_tools` | (empty) | Comma-separated tool allowlist |
| `agent_permission_mode` | (empty) | Claude Code `--permission-mode` value |

### Session persistence

Session index stored as JSONL at `~/.config/srepd/sessions/index.jsonl`.
Tolerates corrupt trailing lines (partial writes on crash).

## Files modified

- `pkg/agent/agent.go` — event types, parser, encoder, session ID, spawn args
- `pkg/agent/agent_test.go` — 31 tests covering all protocol paths
- `pkg/agent/session.go` — Session, SessionManager, StreamCommandExecutor
- `pkg/agent/session_test.go` — session lifecycle, LRU eviction, close safety
- `pkg/agent/testdata/raw-capture.ndjson` — sanitized real NDJSON fixture
- `pkg/agent/testdata/partial-capture.ndjson` — fixture with partial messages
- `pkg/tui/claude.go` — session event handler, startAgentSession, readAgentSessionCmd
- `pkg/tui/tui.go` — Update wiring for agentSessionEventMsg/DoneMsg
- `pkg/tui/model.go` — session manager fields, config resolution helpers
- `pkg/tui/view_render_test.go` — integration tests for tool-use/text rendering
- `pkg/config/config.go` — new DefaultOptionalKeys and OptionalKeys entries
- `go.mod` — promote google/uuid from indirect to direct

## Design decisions

1. **Per-incident sessions, not per-query** — conversational context accumulates
   across `:agent` queries for the same incident.
2. **Lazy spawn** — process not started until first `Send()`, so creating a
   `Session` object is free.
3. **LRU eviction** — bounded resource usage; evicted sessions resume via
   `--resume` flag.
4. **`[]Event` return** — a single NDJSON line can contain multiple content
   blocks, so the parser returns a slice.
5. **Never `--bare`** — Claude Code needs its system prompt for safe tool use.
6. **Fallback preserved** — non-Claude CLIs and `agent_session_enabled: false`
   fall through to existing blocking/streaming paths.

## Out of scope

- Full in-TUI permission approvals (phase 3)
- MCP server integration (`pkg/mcpserver/`)
- Tool policy engine (`pkg/ai/policy/`)
- Delta-based streaming (`pkg/delta/`)
