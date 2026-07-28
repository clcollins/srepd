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

---

## 410a rejection and salvage (PR #414)

### Rejection reason

PR #414 was rejected under plan 410a (TDD amendment). The session persistence
layer (`SessionIndexPath`, `EncodeSessionEntry`, `DecodeSessionIndex`) existed
as pure functions called by nothing — unit tests passed on in-memory strings
while no code path ever invoked them. This is the "false-green" failure class:
tests verify internal consistency of dead code while the headline acceptance
criteria (persistent sessions) remain unmet.

### False-green post-mortem

**Root cause:** Tests were written against the encoding/decoding functions in
isolation. No integration test verified that any production code path called
these functions. The session manager's `MarkResumable` method was the intended
call site, but it was never wired into the eviction flow.

**Detection gap:** CI passed because the dead-code functions were syntactically
correct and their unit tests exercised the right edge cases. But `deadcode`
analysis would have flagged them as unreachable. The 410a amendment now requires
a deadcode gate before merge.

**Prevention:** (1) Walking skeleton first — every new capability must have at
least one end-to-end path before function-level tests are written. (2) Revert
check — stub each fix, confirm the mapped test fails, restore. (3) Deadcode
gate — `deadcode ./...` must report zero findings.

### Salvage scope (this PR)

The work was split into two PRs per 410a:

**PR (i) — salvage (this PR):**
- Removed dead session persistence code (`SessionIndexPath`, `EncodeSessionEntry`,
  `DecodeSessionIndex`) and their tests
- Set `agent_session_enabled` default to `false` with documentation explaining
  that session persistence is not yet implemented
- Fixed 7 defects (D1–D7) with tests per 410a TDD rules:
  - D1: Double-render prevention (consolidated text events filtered in readLoop)
  - D2: MarkResumable race eliminated (replaced with eviction map pattern)
  - D3: Silent event loss under backpressure (blocking send with context cancel)
  - D4: CLI command args discarded (prepend user args from CLICommand)
  - D5: Key leakage into chat input (hoist chatMode check above chord machine)
  - D6: Chat pane viewport (separate chatViewport with smart scroll, scroll keys)
  - D7: Help text overflow (clamp help content to Padded container width)

**PR (ii) — deferred (separate PR):**
- Session persistence implementation with hermetic test harness
- Resume semantics with `--resume` flag
- Session index storage and corruption recovery
