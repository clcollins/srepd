# Plan 393: Stream `:agent` CLI responses token-by-token

**Branch:** `srepd/stream-claude-responses`

## Problem

`:agent` queries invoke an external CLI (default: `claude --print`) as a blocking
subprocess — the user sees a spinner for 30-60 seconds with no feedback until the
full response arrives. The `:watcher` path already streams via the `ai.Provider` SDK
(plan 110), but `:agent` uses a CLI subprocess, which is a fundamentally different
integration point.

For users running Claude Code via Vertex AI (Google Cloud auth), the CLI subprocess
is the *only* working path — the SDK-based `ai.Provider` only supports direct
Anthropic API key auth.

## Solution

### Claude CLI detection (`isClaudeCLI`)

Check whether any whitespace-delimited token in the configured `agent_cli_command`
has a `filepath.Base` of `"claude"`. This handles bare commands, absolute paths,
`toolbox run` wrappers, and `flatpak-spawn` wrappers. No user config required —
auto-detection is strict (basename must be exactly `"claude"`; `claude-wrapper`
does not match) to avoid injecting streaming flags into incompatible CLIs.

### Streaming command (`streamAgentCmd`)

When `stream_responses` is true and `isClaudeCLI` matches, `handleClaudePrompt`
dispatches `streamAgentCmd` instead of the blocking `agentQuery`. It:

1. Appends `--output-format stream-json` and `--include-partial-messages` flags
2. Spawns the CLI subprocess with stdin pipe (prompt) and stdout pipe
3. Reads NDJSON lines via `bufio.Scanner` in a background goroutine
4. Extracts `text_delta` events from `stream_event` lines
5. Sends `streamEvent{text: ...}` to a buffered channel (capacity 64)
6. Returns `agentStreamStartedMsg{ch, cancel}` to start the Update loop drain

### Message types and Update wiring

Three new Bubble Tea message types mirror the watcher streaming pattern:
- `agentStreamStartedMsg` — stores cancel func, expands watcher, starts draining
- `agentStreamChunkMsg` — appends text to `agentStreamPartial`, updates buffer
  in-place via `SetLast`, re-issues `readAgentStreamCmd`
- `agentStreamDoneMsg` — clears querying state, flashes error if present

### Fallback

Non-Claude CLIs and `stream_responses: false` fall through to the existing blocking
`agentQuery` + typewriter animation. No behavior change for existing users.

### Dev mode fix

`cmd/root.go` `runDevMode()` was passing `nil`/`""` for `aiProvider`/`agentCLICommand`,
preventing AI features from working in dev mode. Now reads from config, enabling
streaming testing with `make dev`.

## Files modified

- `pkg/tui/claude.go` — `isClaudeCLI`, JSON structs, `buildStreamingArgs`,
  `streamAgentCmd`, `readAgentStreamCmd`, agent stream message types, streaming
  dispatch in `handleClaudePrompt`
- `pkg/tui/claude_test.go` — 28 test functions covering detection, parsing,
  dispatch, Update handlers, edge cases
- `pkg/tui/model.go` — `agentStreamPartial`, `agentStreamCancel` fields
- `pkg/tui/tui.go` — Update cases for the three agent stream messages
- `cmd/root.go` — dev mode AI provider + agentCLICommand initialization
- `README.md` — updated `stream_responses` description

## Tests

- `TestIsClaudeCLI` — 8 cases (bare, absolute, toolbox, flatpak-spawn, non-claude,
  empty, claude-prefix, claude-in-path)
- `TestParseCLIStreamLine_*` — text_delta, result, system event parsing
- `TestBuildStreamingArgs_*` — append and no-duplicate
- `TestReadAgentStreamCmd_*` — text chunk, done, closed channel
- `TestHandleClaudePrompt_StreamingDispatch` — streaming path chosen
- `TestHandleClaudePrompt_NonClaudeCLI_NoStreaming` — fallback
- `TestHandleClaudePrompt_StreamingDisabled_NoStreaming` — config toggle
- `TestAgentStreamStartedMsg_SetsState` — Update handler
- `TestAgentStreamChunkMsg_AppendsText` — accumulation
- `TestAgentStreamDoneMsg_ClearsState` / `_WithError` — cleanup

`make test-all` green (fmt, vet, lint, test, race, fixtures) on go1.26.4.

## Design decisions

- **CLI streaming, not SDK refactor**: Plan 110 deferred `:agent` streaming to an
  SDK refactor (issue #352). This PR takes a pragmatic shortcut — streaming the CLI
  subprocess stdout — because: (a) the CLI is the only working path for Vertex AI
  users, (b) the SDK refactor is a larger project, and (c) the architecture has a
  clean extension point for future CLIs.

- **Claude-specific detection**: There is no universal standard for CLI agent
  streaming output. Claude's `stream-json` NDJSON format is proprietary. The
  `isClaudeCLI()` guard ensures streaming flags are only injected when safe.

- **Shared `streamEvent` type**: Reuses the type from `stream.go` as common
  currency between watcher and agent streaming, with separate message types so the
  Update loop can distinguish the source.
