# Add Claude Code detection env var

## Context

When launching terminal sessions for cluster investigation, signal
whether Claude Code is available so the investigation environment
can leverage AI-assisted workflows.

## Plan

1. Add `HasClaudeCode()` in `pkg/launcher/claude.go` using
   `exec.LookPath("claude")` with injectable stub for testing
2. Set `PAGERDUTY_CLAUDE_AVAILABLE=true` in `buildPagerDutyEnvVars()`
   when detected

## Files Modified

- `pkg/launcher/claude.go` — new, detection function
- `pkg/launcher/claude_test.go` — new, 2 tests
- `pkg/tui/commands.go` — add env var in buildPagerDutyEnvVars

## Verification

- `TestHasClaudeCode_NotInstalled` passes
- `TestHasClaudeCode_Installed` passes
- All tests pass
