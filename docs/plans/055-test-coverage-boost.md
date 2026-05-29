# 055: Test Coverage Boost - High Priority Functions

## Status: In Progress

## Objective

Boost test coverage toward the 70% target by adding tests for high-priority
untested functions identified in Issue #205. Focus on core PagerDuty operations,
rate-limit wrappers, and critical TUI commands.

## Scope

### pkg/pd/pd.go
- `NewConfig()` - Init with mock client; error on bad team ID; error on bad policy
- `AcknowledgeIncident()` - Direct unit test (currently only tested via TUI commands)
- `ReassignIncidents()` - Success case; empty incidents; nil user
- `ReEscalateIncidents()` - Success case; nil policy; level=0; empty incident ID

### pkg/pd/ratelimit.go (10 untested wrappers)
Each wrapper delegates to the inner client through `withRetry()`. Tests verify
that each wrapper calls `limiter.Wait()` and delegates correctly.
- `CreateIncidentNoteWithContext`
- `GetCurrentUserWithContext`
- `GetEscalationPolicyWithContext`
- `GetTeamWithContext`
- `ListMembersWithContext`
- `GetUserWithContext`
- `ListIncidentNotesWithContext`
- `ListOnCallsWithContext`
- `ManageIncidentsWithContext`

### pkg/tui/commands.go
- `UserIsOnCall()` - Test with mock on-call data; user on call; user not on call
- `login()` - Command construction; env var passing; toolbox wrapping
- `removeCommentsFromBytes()` - Regression test for fixed bug
- `runScheduledJobs()` - Job frequency; lastRun tracking

### cmd/root.go
- `configureLogging()` - Already has `determineLogDestination` tests; skip
  additional tests as `configureLogging` has side effects on global log state

## Approach

1. TDD: Write failing tests first
2. Use existing `MockPagerDutyClient` from `pkg/pd/mock.go`
3. Follow table-driven test patterns with `testify/assert`
4. Use `CallCounts` map on mock to verify delegation for rate-limit wrappers
5. All tests must pass locally before committing (`go test ./... -count=1`)

## Files Modified
- `pkg/pd/pd_test.go` - NewConfig, AcknowledgeIncident, ReassignIncidents, ReEscalateIncidents
- `pkg/pd/ratelimit_test.go` - 10 wrapper delegation tests
- `pkg/tui/commands_test.go` - UserIsOnCall, login command construction, removeCommentsFromBytes, runScheduledJobs

## Post-Mortem / Lessons Learned
(To be filled after implementation)
