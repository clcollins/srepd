# Preserve selected incident across auto-refresh list updates

> Retroactive plan document for PR #147, created after merge.
> Fixes #72.

## Context

When `updatedIncidentListMsg` rebuilt table rows, `table.SetRows()`
reset the cursor position. The user's currently highlighted incident
was lost on every auto-refresh cycle, kicking them out of their
workflow. This was particularly disruptive during active incident
response when auto-refresh was enabled.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Create pure function `findRowIndex(rows []table.Row,
   incidentID string) int` that finds the row index for a given
   incident ID
2. In the `updatedIncidentListMsg` handler, capture the highlighted
   incident ID before `SetRows()`, find and restore cursor after
3. If the incident no longer exists (e.g., resolved), leave cursor
   at its current position

## Files Modified

- `pkg/tui/model.go` — `findRowIndex()` pure function
- `pkg/tui/model_test.go` — 3 test functions (Found, NotFound,
  EmptyRows) with 5 test cases
- `pkg/tui/tui.go` — cursor preservation in
  `updatedIncidentListMsg` handler

## Verification

- `TestFindRowIndex_Found` finds correct indices
- `TestFindRowIndex_NotFound` returns -1
- `go test ./...` passes (53 tests, 0 regressions)

## Lessons Learned

- Initial implementation used `currentRow != nil && len(currentRow) > 1`
  which triggered staticcheck S1009 (redundant nil check before len).
  Fixed in a follow-up commit. CI caught this — validates the value of
  the lint check added in [003-ci-infrastructure.md](003-ci-infrastructure.md).
