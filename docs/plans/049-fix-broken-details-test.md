# 049: Fix broken test referencing removed Details field

## Status: Complete

## Problem

PR #186 removed the `Details` field from the `alertSummary` struct in
`pkg/tui/views.go`. PR #184 merged after #186 and re-introduced a test
(`TestSummarizeAlerts_AlertWithNilBody`) that asserts on `result[0].Details`,
which no longer exists. This causes a compilation failure on main.

## Fix

Remove the line:

```go
assert.Nil(t, result[0].Details, "details should be nil when body is nil")
```

from `pkg/tui/views_test.go` (line 410). The assertion is meaningless because
the field was removed from the struct. No other test references the removed
struct field.

## Verification

- `go test ./pkg/tui/... -v -count=1` passes
- `go vet ./...` passes
