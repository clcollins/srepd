# Enhance mock PagerDuty client with pagination and call counting

> Retroactive plan document for PR #148, created after merge.

## Context

The existing mock in `pkg/pd/mock.go` used convention-based error
signaling (ID = "err") and returned fixed responses. It did not
support configurable pagination (`More=true/false`), call count
tracking, or multiple response queues. This blocked testing for rate
limiting, pagination fixes, and timeout scenarios.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Add `CallCounts map[string]int` for tracking calls per method
2. Add `ListMembersResponses` queue with `ListMembersResponse` type
   (includes `Response` and `Err` fields)
3. Add `recordCall()` helper with lazy map initialization
4. Implement `ListMembersWithContext` with index-based queue popping
5. Maintain full backward compatibility with all existing tests

## Files Modified

- `pkg/pd/mock.go` — enhanced with call counting, response queues,
  `ListMembersWithContext`
- `pkg/pd/mock_test.go` — new file with 6 tests

## Verification

- `TestEnhancedMock_BackwardCompatible` passes
- `TestEnhancedMock_PaginatedResponse` passes
- All existing tests pass unchanged
