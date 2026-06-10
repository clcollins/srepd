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

## Lessons Learned

**PROCESS GAP — no convention for documenting lessons from fix PRs**
(Fixed by: [059-lessons-learned-convention.md](059-lessons-learned-convention.md))

This plan fixed a compilation failure caused by PRs #184 and #186
merging in the wrong order (PR #184 referenced a struct field that
PR #186 had removed). The fix was straightforward but no lesson was
recorded about the merge ordering problem, allowing the same class of
mistake to repeat.

Why it wasn't caught: there was no convention requiring lessons-learned
documentation when a PR fixes issues from a previous PR. Fix PRs
silently corrected problems without recording root causes or prevention
strategies.

Prevention: the fix established a convention in CONVENTIONS.md and
AGENTS.md requiring a "Lessons Learned" section in fixing PR plan docs
and cross-reference updates to original plan docs. Process gaps are
themselves bugs that need fixes — establishing conventions for
post-mortems prevents institutional amnesia.
