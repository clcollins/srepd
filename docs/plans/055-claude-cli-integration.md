# 055: Claude CLI Integration via Input Field

**Issue**: #194
**Branch**: `srepd/claude-cli-integration`
**Status**: In Progress

## Problem

SREs investigating incidents in srepd must context-switch to separate
terminals for AI-assisted investigation. The existing input field
(`:` / `i` keys) is non-functional (Enter handler is a no-op with
a "Future: process the input command" comment).

## Solution

Wire up the existing input field to dispatch prompts to the local
Claude CLI binary via stdin. Responses render in the incident viewer
viewport for seamless investigation without leaving srepd.

## Changes

### pkg/tui/model.go
- Increase `newTextInput()` CharLimit from 32 to 500
- Increase Width from 50 to 120 (will be adjusted by window resize)
- Add `claudeQuerying bool` field to model for spinner tracking

### pkg/tui/commands.go
- Add `claudePromptMsg` and `claudeResponseMsg` message types
- Add `claudeQuery()` tea.Cmd that:
  - Uses `launcher.HasClaudeCode()` to check availability
  - Builds `exec.Command("claude")` with prompt via stdin
  - Passes PAGERDUTY_* context as env vars
  - Includes read-only system prompt via `--system-prompt` or stdin prefix
  - Returns response or error via `claudeResponseMsg`
  - Uses 60s timeout with context cancellation

### pkg/tui/msgHandlers.go
- Wire Enter key in `switchInputFocusMode()` to:
  - Extract input text
  - Reset and blur input
  - Return `claudePromptMsg` with the text
- Pass viewport width to input on window resize

### pkg/tui/tui.go
- Handle `claudePromptMsg`:
  - Check `HasClaudeCode()` -> status "Claude Code not installed" if missing
  - Set status "querying Claude..."
  - Set `claudeQuerying = true` and `apiInProgress = true`
  - Dispatch `claudeQuery()` command
- Handle `claudeResponseMsg`:
  - Clear `claudeQuerying` and `apiInProgress`
  - On error: set status with error message
  - On empty response: set status "no response from Claude"
  - On success: set incidentViewer content and viewingIncident = true

### Safety
- System prompt: "You are in read-only investigation mode. Suggest
  commands for the user to run if changes are needed. Do not modify
  cluster state."
- Prompt piped via stdin (no `-p` flag)
- 60-second timeout with process kill

## Tests (TDD)

1. `TestClaudePrompt_DispatchesCommand` - Enter in input mode
   triggers claudePromptMsg with input text
2. `TestClaudeResponse_RendersInViewport` - claudeResponseMsg sets
   incidentViewer content and viewingIncident
3. `TestClaudeNotFound_ShowsStatus` - claudePromptMsg when Claude
   not available shows status message
4. `TestClaudeQuery_PassesContext` - env vars include PAGERDUTY_*
5. `TestClaudeQuery_Timeout` - query respects timeout
6. `TestInputCharLimit_Increased` - newTextInput CharLimit is 500

## Non-goals

- Container ID tracking for podman exec (future work)
- Toolbox fallback detection (future work)
- Streaming response rendering (future work)

## Lessons Learned

**GENUINE ERROR — hardcoded binary name broke non-standard environments**
(Fixed by: [061-configurable-agent-cli.md](061-configurable-agent-cli.md))

This plan hardcoded `exec.CommandContext(ctx, "claude", "--print")`,
which failed when `claude` was a shell alias/function or not on PATH.

Why it wasn't caught: testing was done in an environment where `claude`
was on PATH as a real binary. No test verified behavior when the binary
was absent or aliased.

Prevention: external command invocations should always be configurable
via a config key, especially for tools that may be installed in
non-standard locations. Use injectable `lookPath` functions for
testability.

---

**GENUINE ERROR — global keybindings intercepted input field characters**
(Fixed by: [092-fix-input-mode-keybinding-conflict.md](092-fix-input-mode-keybinding-conflict.md))

After wiring up the input field, global key handlers in
`keyMsgHandler()` intercepted keypresses (`u`, `i`, `:`, `ctrl+r`,
`ctrl+a`) before they reached `switchInputFocusMode()`, preventing
users from typing those characters.

Why it wasn't caught: the implementation did not test typing characters
that collide with global bindings. The input field worked for
characters that had no global binding, masking the conflict.

Prevention: any TUI with both global keybindings and text input fields
must gate global handlers on input focus state. Add integration tests
that verify typing every printable character in input mode.

---

**GENUINE ERROR — fallthrough dispatch became inconsistent as commands grew**
(Fixed by: [093-agent-slash-command.md](093-agent-slash-command.md))

Bare text in the input field fell through to Claude dispatch. When flag
conditions (PR #304) added `/flag` slash commands, the dispatch model
became inconsistent: some commands used slash prefixes and others did
not.

Why it wasn't caught: at the time of this plan, the input field had only
one purpose (Claude queries), so fallthrough was reasonable. The design
did not anticipate future slash commands sharing the input field.

Prevention: when designing command dispatch, prefer explicit routing
(slash-command prefixes) over fallthrough defaults. This maintains
consistency as new commands are added. Design input dispatch for
extensibility even when starting with a single command.
