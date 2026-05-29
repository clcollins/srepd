# Plan 027: Bug Fixes and Pure Function Test Coverage

## Summary

Fix two known bugs and add table-driven test coverage for six pure functions
in the TUI commands package.

## Bug Fixes

### Bug 1: removeCommentsFromBytes() line duplication (pkg/tui/commands.go)

**Problem**: The nested loop iterated over prefixes inside the lines loop. When
multiple prefixes were provided, non-comment lines were written once per
non-matching prefix, causing content duplication. Comment lines matching only
some prefixes would also be partially preserved.

**Root cause**: The inner loop wrote the line immediately when a prefix did not
match, rather than checking all prefixes first.

**Fix**: Introduce a boolean flag (`isComment`) that is set when any prefix
matches, with a `break` to short-circuit. The line is only written if no prefix
matched.

### Bug 2: PostNote() missing error context (pkg/pd/pd.go)

**Problem**: `PostNote` returned the raw error from
`CreateIncidentNoteWithContext` without wrapping it, unlike every other function
in the pd package which wraps errors with function name and context.

**Fix**: Wrap the error with `fmt.Errorf("pd.PostNote(): failed to create note
for incident %v: %w", id, err)` to match the established pattern. Uses `%w` for
error chain preservation.

## New Tests

### Pure function tests (pkg/tui/commands_test.go)

All tests are table-driven using testify/assert:

| Function | Test Cases |
|---|---|
| `stateShorthand` | Acked by user (A), acked by other (a), not acked (dot), multi-ack |
| `acknowledged` | Has acks, multiple acks, empty, nil |
| `AssignedToUser` | Assigned, not assigned, no assignments, multi-assignee |
| `AcknowledgedByUser` | User acked, other acked, none, multi-ack |
| `AssignedToAnyUsers` | Match, no match, no assignments, empty IDs, nil IDs, multi-both |
| `removeCommentsFromBytes` | Single prefix, multi-prefix, no-duplication, all-comments, no-match, empty, no-prefixes, three-prefix mix |

### PostNote test (pkg/pd/pd_test.go)

Table-driven test covering success path and error wrapping verification.

### Mock additions (pkg/pd/mock.go)

Added mock implementations for:
- `CreateIncidentNoteWithContext` (uses "err" sentinel pattern)
- `GetCurrentUserWithContext`
- `GetEscalationPolicyWithContext`
- `GetUserWithContext`

## Verification

- `go test ./... -count=1` -- all pass
- `go vet ./...` -- clean
- `gofmt -s -l cmd pkg` -- clean

## Files Changed

- `pkg/tui/commands.go` -- removeCommentsFromBytes bug fix
- `pkg/tui/commands_test.go` -- 6 new table-driven test functions
- `pkg/pd/pd.go` -- PostNote error wrapping
- `pkg/pd/pd_test.go` -- PostNote test
- `pkg/pd/mock.go` -- 4 new mock methods
- `docs/plans/027-bug-fixes-pure-function-tests.md` -- this plan
