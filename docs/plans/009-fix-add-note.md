# Fix add-note requiring explicit row navigation before use

> Retroactive plan document for PR #149, created after merge.
> Fixes #98.

## Context

Pressing 'n' to add a note in table view failed with "no incident
selected" even when a row was highlighted. The Note key handler
checked `m.selectedIncident == nil` directly, but `selectedIncident`
is only populated by `syncSelectedIncidentToHighlightedRow()`, which
runs on explicit navigation (up/down keys). On first load or after
refresh without navigation, `selectedIncident` was nil despite a
highlighted row.

The Acknowledge and Silence handlers worked because they used
different patterns to resolve the incident.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. In `switchTableFocusMode` Note handler, add `SelectedRow()` nil
   guard for empty table case
2. Call `syncSelectedIncidentToHighlightedRow()` before checking
   `selectedIncident`
3. TDD: 3 tests covering no-selectedIncident, with-selectedIncident,
   and no-rows cases

## Files Modified

- `pkg/tui/msgHandlers.go` — 5 lines added to Note handler
- `pkg/tui/msgHandlers_test.go` — new file with 3 test functions

## Verification

- `TestTableMode_NoteKeyWithNoSelectedIncident` passes (the bug case)
- `go test ./...` passes with zero regressions

## Lessons Learned

- The test file was not formatted with `gofmt -s`, caught by CI
  `fmt-check`. Fixed in follow-up commit. All agents should run
  `gofmt -s` before committing.
- The same bug existed in the Open key handler ('o'). See
  [013-normalize-key-handlers.md](013-normalize-key-handlers.md) for the
  systematic fix.
