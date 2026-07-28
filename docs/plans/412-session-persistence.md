# Plan 412: Session persistence, restart-resume, and fire-and-return

**Branch:** `srepd/ai-p2-session-persistence`

## Problem

Plan 411 introduced persistent per-incident agent sessions, but session
state was lost on srepd restart — the session manager had no way to know
that a previous run had already established a session for a given incident.
Additionally, `:agent <query>` incorrectly opened chat mode instead of
dispatching and returning to the queue, and `startAgentSession` used
`context.Background()` for sends, meaning a hung child process could block
forever.

## Solution

### JSONL session index (`pkg/agent/index.go`)

An append-only JSONL file at `~/.config/srepd/sessions/index.jsonl`
(XDG_CONFIG_HOME-aware) records each established session as a JSON line
with `incident_id`, `session_id`, `created`, and `last_used` fields.

Key design decisions:

- **Record on establishment, not spawn:** The `onEstablished` callback fires
  only after `system/init` is received from the child process. A crash before
  init leaves no index entry, so the next spawn correctly uses `--session-id`
  (fresh) instead of `--resume` (which would fail for an unknown session).

- **Corrupt-line tolerance:** `load()` skips unparseable lines and logs a
  warning for the trailing corrupt line. This handles partial writes from
  crashes.

- **Unwritable dir fallback:** If the session directory cannot be created,
  the index operates in-memory only (no panic, no data loss for the current
  session).

### Fake claude harness (`pkg/agent/testdata/fakeclaude/main.go`)

A standalone Go binary that reproduces the verified session-ID semantics of
claude 2.1.220:

- `--session-id <fresh>` → normal NDJSON
- `--session-id <used>` → exit 1, stderr error, NO result line
- `--resume <known>` → normal NDJSON
- Supports scripted scenarios via `FAKECLAUDE_SCRIPT` env var
- Logs argv and stdin to `FAKECLAUDE_LOG` as JSONL for test assertions

### Integration tests (`pkg/agent/integration_test.go`)

All integration tests assert on the **spawn-flag decision** (`--resume` vs
`--session-id`), never on UUID equality (which proves nothing since
`SessionIDFor` is deterministic).

Tests:
- H1a: Restart-resume (two managers, same configDir, second uses `--resume`)
- H1b: Absent index → `--session-id`
- H2: Per-incident isolation (distinct session IDs)
- L1: Crash before init → no index entry → next uses `--session-id`
- V1: Duplicate session-id → Error event, no hang
- LRU eviction → `--resume` on re-access
- L2: Send with cancelled context → error
- Double-render prevention with scripted harness
- Crash mid-stream → Error surfaced, session resumable
- Index robustness: corrupt line tolerance, unwritable dir fallback

### B1: `:agent <query>` fire-and-return (`pkg/tui/msgHandlers.go`)

Removed `m.enterChatModeState()` from the `:agent <query>` path. The query
is dispatched and control returns to the incident queue immediately,
matching the `:watcher <query>` precedent. Bare `:agent` still opens chat
mode.

### L2: Send context timeout (`pkg/tui/claude.go`)

Replaced `context.Background()` with `context.WithTimeout(ctx, 30s)` in
`startAgentSession`. A hung child process now returns an error after 30
seconds instead of blocking forever.

### Flag flip (`pkg/config/config.go`, `pkg/tui/model.go`)

`agent_session_enabled` default flipped from `false` to `true`. The
resolver function and all related tests updated accordingly.

### Hardening fixes (PR #416 follow-up)

**FIX 1 — `--bare` denylist bypass on legacy spawn paths:**
`validateUserFlags` only guarded the session path inside `pkg/agent/spawn()`.
The legacy paths (`agentQuery` and `streamAgentCmd` in `pkg/tui/claude.go`)
parsed the same config and exec'd directly with no validation. Fixed by
exporting `ValidateUserFlags` and calling it once in `handleClaudePrompt`
before any path dispatches, so all three spawn paths share one gate.

**FIX 2 — Write goroutine leak in `Send`:**
Each `Send` spawned a fresh goroutine for the stdin pipe write. When the
child stopped draining stdin and the caller's context timed out, `Send`
returned but the goroutine stayed pinned on the blocked write. Replaced with
a single `writeLoop` goroutine per session, started at spawn time, that
drains a channel of write requests. Goroutine count stays constant regardless
of how many Sends time out; `Close()` closes the channel and stdin, allowing
`writeLoop` to exit.

**FIX 3 — Index write hygiene:**
- Combined the two-call `f.Write(data)` + `f.Write(newline)` into a single
  `f.Write(append(data, '\n'))` to prevent a corrupt trailing line on crash.
  Write errors are now logged instead of silently swallowed.
- Added `scanner.Err()` check after the load loop (identical omission was
  fixed in `readLoop` during PR #414).
- Narrowed `record()`'s critical section: the in-memory map is updated under
  the lock, then released before file I/O, so `has()` on the TUI thread is
  never blocked by slow filesystem operations.

## Post-mortem / Lessons learned

### `--resume <unknown-id>` remains UNVERIFIED against the real CLI

The fake claude harness scripts a plausible failure for `--resume <unknown>`,
but the actual Claude Code behaviour for this case was never captured from the
real binary. Plan 412's verified-facts table covers `--session-id <used>` and
`--resume <known>` but explicitly marks `--resume <unknown>` as UNVERIFIED.
The harness's scripted failure is a reasonable assumption, not a verified fact.
Any test relying on the exact shape of this failure (exit code, stderr message)
should be re-verified when the tested claude version bumps.

### Context-cascade bug: child killed after first send

During implementation, the initial integration tests used `context.WithCancel`
derived from the caller's `Send(ctx)` context for the spawned process. When
the caller's context was cancelled (e.g. timeout on the send), the child
process was killed — even though the child should outlive any single send.
The fix was to introduce a `lifecycleCtx` on the session (derived from the
manager's context, cancelled only by `CloseAll`) as the parent for `spawnCtx`,
decoupling subprocess lifetime from caller-send lifetime.

**The lesson:** the test harness originally stubbed `StreamCommandExecutor`
with a mock that discarded its `context.Context` argument entirely. Every mock
executor started a goroutine that ran until explicitly told to stop, so the
context-cascade bug was invisible — the mock never respected cancellation, and
the bug lived exactly on the boundary where real `exec.CommandContext` DOES
respect it. Future harnesses must exercise process lifetime, not just protocol.
The fake claude binary (a real subprocess) caught this because
`exec.CommandContext` actually kills the child when the context fires.

### Revert-check gate was itself unwired

The `ForTestStubIndexWrite()` helper was defined and documented as the 410a
§2c revert-check mechanism, but no test actually called it. A helper with no
caller proves nothing. The fix was to write `TestRevertCheck_StubIndexWrite`,
which stubs index writes and asserts the restart-resume test's inverse: with
writes disabled, the second manager falls back to `--session-id`. Both
`ForTestStubIndexWrite` and `IndexEntryCount` were then moved into `_test.go`
so no test-only helper remains in production code.

### Dead accessors `Session.ID()` and `Session.IncidentID()`

Both methods had zero callers anywhere in the codebase. They were written
speculatively during the initial implementation and never wired. Deleted.
`deadcode ./...` would have caught these immediately.

## Key files changed

| File | Change |
|------|--------|
| `pkg/agent/index.go` | New: JSONL session index |
| `pkg/agent/session.go` | `onEstablished` callback, index integration |
| `pkg/agent/agent.go` | `SessionDir` field in Config |
| `pkg/agent/testdata/fakeclaude/main.go` | New: fake claude harness |
| `pkg/agent/integration_test.go` | New: 10 integration tests |
| `pkg/tui/model.go` | `resolveSessionDir()`, default true |
| `pkg/tui/claude.go` | L2: context timeout |
| `pkg/tui/msgHandlers.go` | B1: fire-and-return |
| `pkg/config/config.go` | Default flip |
| `docs/ai-agents.md` | Updated docs |
| `README.md` | Updated docs |
