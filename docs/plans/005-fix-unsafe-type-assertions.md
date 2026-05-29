# Fix unsafe type assertions in alert detail extraction

> Retroactive plan document for PR #145, created after merge.

## Context

`pkg/tui/commands.go:787-788` used bare type assertions
`.(map[string]interface{})` and `.(string)` without the comma-ok
pattern. Non-standard alert sources (CPD alerts, custom integrations)
have different body structures, causing panics at runtime. The same
pattern existed in `summarizeAlerts` in `pkg/tui/views.go`.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Rewrite `getDetailFieldFromAlert` with comma-ok type assertions
   at each level (body nil, details key, details map, field key,
   field string)
2. Fix `summarizeAlerts` in `views.go` with the same pattern
3. TDD: write 5 tests covering nil body, missing details, wrong
   type details, wrong type field, and happy path

## Files Modified

- `pkg/tui/commands.go` — safe comma-ok in `getDetailFieldFromAlert`
- `pkg/tui/views.go` — safe comma-ok in `summarizeAlerts`
- `pkg/tui/commands_test.go` — 8 new tests

## Verification

- `TestGetDetailFieldFromAlert_NilBody` does not panic
- `TestGetDetailFieldFromAlert_DetailsNotMap` does not panic
- `go test ./...` passes with zero regressions
