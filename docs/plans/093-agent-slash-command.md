# Plan: Require /agent prefix for Claude queries (#309)

**Status:** Complete
**Issue:** #309
**PR:** TBD

## Problem

With flag conditions (PR #304), the input field supports `/flag` slash
commands. Bare text still falls through to Claude dispatch, which is
inconsistent. All input commands should use explicit prefixes.

## Solution

Require `/agent <query>` prefix for Claude queries. Bare text without a
recognized slash command shows an error with available commands.

The `i`/`:` keys open a blank input prompt (no pre-fill) so users can
type any command: `/agent`, `/flag`, `/flags`, `/unflag`.

## Files changed

- `pkg/tui/claude.go` — add `isAgentCommand()`, `parseAgentQuery()`
- `pkg/tui/claude_test.go` — 11 new tests, 2 updated existing tests
- `pkg/tui/msgHandlers.go` — update Enter dispatch to require prefix
- `pkg/tui/msgHandlers_test.go` — update 1 existing test for /agent
- `pkg/tui/keymap.go` — update help text to "command input"
- `README.md` — update key binding table

## Post-mortem / lessons learned

_(to be completed after merge)_
