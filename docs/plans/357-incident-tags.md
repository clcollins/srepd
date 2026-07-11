# 357: Add incident tags via ctrl+t

## Problem

Incident summaries in PagerDuty sometimes have bracket-delimited tags at the
front (e.g., `[HCP] [RHOBS] (Warning) SomeAlert...`). SREs need to add or
modify these tags from within SREPD to track incident state without leaving
the TUI.

## Approach

Added `ctrl+t` keybinding that opens the command input with a custom prompt
(`enter tags (comma-sep) > `). The user types comma-separated tags, presses
Enter, and the tags are parsed, formatted as `[tag1][tag2]...`, and prepended
to the incident title via the PagerDuty ManageIncidents API.

### Tag parsing rules

1. Comma is the primary delimiter — split on `,`, trim whitespace
2. Brackets are stripped then re-added — `foo` and `[foo]` both produce `[foo]`
3. Best-effort bracket-delimited input — `[FOO] [Bar]` splits on `] [`
4. Spaces are NOT delimiters — `foo bar baz` becomes `[foo bar baz]`
5. Duplicate tags (case-insensitive) already in the title are skipped

### Design decisions

- Pure functions for all parsing logic (`ParseTags`, `FormatTags`,
  `PrependTags`, `ExtractExistingTags`) — fully testable without TUI state
- Tags modify the actual PD incident title via `ManageIncidentsWithContext`
  (not a local-only visual marker like flags)
- `tagInputActive` boolean on the model distinguishes tag input from regular
  `:` command input in `switchInputFocusMode`

## Files changed

| File | Change |
|------|--------|
| `pkg/tui/tags.go` | New — pure parsing functions, message types, command |
| `pkg/tui/tags_test.go` | New — 48 tests (parsing + TUI integration) |
| `pkg/tui/keymap.go` | Added `Tag` binding (`ctrl+t`) |
| `pkg/tui/model.go` | Added `tagInputActive` field |
| `pkg/tui/msgHandlers.go` | `ctrl+t` handler + tag processing in input mode |
| `pkg/tui/tui.go` | `updatedIncidentTitleMsg` handler |
| `pkg/pd/pd.go` | `UpdateIncidentTitle` function |
| `pkg/pd/dev.go` | Title mutation in dev client |
| `pkg/pd/pd_test.go` | 5 tests for `UpdateIncidentTitle` |

## Testing

- 48 tests covering all parsing edge cases and TUI integration
- All pre-commit checks pass (fmt, vet, lint, test, race)
