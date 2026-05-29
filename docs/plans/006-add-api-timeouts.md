# Add request timeouts to all PagerDuty API calls

> Retroactive plan document for PR #146, created after merge.

## Context

Every PD API call in `pkg/pd/pd.go` used `context.Background()` with
no timeout. If PagerDuty was slow or unresponsive, the TUI would hang
indefinitely with no way to recover short of killing the process.
There were 14 bare `context.Background()` calls.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Create `contextWithTimeout()` helper returning
   `context.WithTimeout(context.Background(), 30*time.Second)`
2. Replace all 14 `context.Background()` calls in `pkg/pd/pd.go`
3. Add `defer cancel()` after each call
4. Do NOT modify `pkg/tui/commands.go` (those contexts are used
   differently for `tea.Cmd`)

## Files Modified

- `pkg/pd/pd.go` — `contextWithTimeout` helper, 14 call sites updated
- `pkg/pd/pd_test.go` — 2 new tests

## Verification

- `TestContextWithTimeout_HasDeadline` passes
- `TestContextWithTimeout_CancelWorks` passes
- `go test ./...` passes with zero regressions
