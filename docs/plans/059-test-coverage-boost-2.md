# 059: Test Coverage Boost 2 - TUI Command Wrappers

## Status: Complete

## Objective

Second round of test coverage improvements targeting 0%-coverage TUI command
wrapper functions identified in Issue #205. Brings pkg/tui coverage from 55.6%
to 58.5% and moves eight previously-uncovered functions to 100%.

## Scope

### pkg/tui/commands.go - TUI command wrappers
- `reassignIncidents()` - Success, error, empty incident ID, GetCurrentUser error
- `reEscalateIncidents()` - Success, error, empty incident ID
- `fetchEscalationPolicyAndReEscalate()` - Success, policy fetch error, re-escalate error
- `silenceIncidents()` - Success, nil policy, empty incidents, zero level, empty policy name/ID, multiple incidents
- `ShouldBeAcknowledged()` - All conditions true, not assigned, already acked, auto-ack disabled, not on-call
- `readLogFile()` - Existing file, missing file
- `execErr.Error()` - Strips prefixes and suffixes
- `execErr.Code()` - Returns correct exit codes

## Approach

1. TDD: Write failing tests first
2. Use existing `MockPagerDutyClient` from `pkg/pd/mock.go`
3. Created `mockCurrentUserErrorClient` embedding the mock to test GetCurrentUser error path
4. Follow table-driven test patterns with `testify/assert`
5. All tests pass locally before committing

## Coverage Improvements

| Function | Before | After |
|----------|--------|-------|
| `execErr.Error()` | 0% | 100% |
| `execErr.Code()` | 0% | 100% |
| `readLogFile` | 20% | 100% |
| `ShouldBeAcknowledged` | 0% | 100% |
| `reassignIncidents` | 0% | 100% |
| `reEscalateIncidents` | 0% | 100% |
| `fetchEscalationPolicyAndReEscalate` | 11.1% | 100% |
| `silenceIncidents` | 37.5% | 100% |

## Files Modified
- `pkg/tui/commands_test.go` - 23 new test functions

## Post-Mortem / Lessons Learned

- Embedding `MockPagerDutyClient` in a test-local struct with a single method
  override is an effective pattern for testing specific error paths without
  modifying the shared mock.
- The `reassignIncidents` TUI wrapper calls `GetCurrentUserWithContext` internally
  rather than using `p.CurrentUser`, requiring a specialized mock to reach the
  error branch.
