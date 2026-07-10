# Plan 356: Fix `/` → `:` command prefix in error messages and docs

**Status:** Complete

## Problem

All colon commands (`:flag`, `:unflag`, `:flags`, `:agent`, `:watcher`) are
dispatched with the `:` prefix in code, but several error messages, help text,
config descriptions, and plan docs still referenced the old `/` slash prefix.
This caused user-visible confusion — e.g., the `:flags` list view told users
to type `/unflag all` which doesn't work.

## Solution

Replace every remaining `/` command prefix with `:` across error messages,
help text, config descriptions, test inputs, and plan docs.

## Files changed

| File | Change |
|------|--------|
| `pkg/tui/flags.go` | Fix help text: `/unflag` → `:unflag` |
| `pkg/tui/flag_commands.go` | Fix 5 error messages: `/flags`, `/flag`, `/unflag` → `:` prefix |
| `pkg/config/config.go` | Fix config description: `/agent` → `:agent` |
| `pkg/tui/claude_test.go` | Fix negative test input: `/flag` → `:flag` |
| `docs/plans/093-agent-slash-command.md` | Fix title and body: `/` → `:` prefix throughout |
| `docs/plans/091-flag-conditions.md` | Fix file table descriptions |
| `docs/plans/062-ambient-watcher.md` | Fix `/agent` → `:agent` |
| `docs/plans/061-configurable-agent-cli.md` | Fix `/agent` → `:agent` |
| `docs/plans/055-claude-cli-integration.md` | Fix `/flag` → `:flag` |
