# Fix panic in loopManageIncidents by returning error instead

> Retroactive plan document for PR #144, created after merge.

## Context

`pkg/pd/pd.go:322` called `panic()` if the PagerDuty
ManageIncidents API returned `More=true`. ManageIncidents is not a
paginated endpoint, so this was defensive code, but a panic in
production crashes the entire TUI. The function was also
unnecessarily wrapped in a `for` loop with dead code after the panic.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Replace `panic(...)` with `return nil, fmt.Errorf(...)`
2. Remove the unnecessary `for` loop (single call, not paginated)
3. Remove dead code after the panic
4. TDD: write 3 tests first (success, API error, unexpected More),
   verify they fail, then implement

## Files Modified

- `pkg/pd/pd.go` — replaced panic with error return, simplified
- `pkg/pd/pd_test.go` — 3 new tests
- `pkg/pd/mock.go` — added `MockManageIncidentsMoreClient`

## Verification

- `TestLoopManageIncidents_UnexpectedMore` returns error (not panic)
- `go test ./...` passes (56 tests, 0 failures)
