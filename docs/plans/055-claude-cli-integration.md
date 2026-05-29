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
