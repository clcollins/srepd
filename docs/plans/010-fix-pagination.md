# Fix pagination gaps in GetTeamMemberIDs and GetUserOnCalls

> Retroactive plan document for PR #150, created after merge.
> Fixes #24.

## Context

Two pagination bugs in `pkg/pd/pd.go`:

1. `GetTeamMemberIDs`: `opts.Offset` was not reset between teams.
   After paginating through team A, team B started at the wrong
   offset and missed members.
2. `GetUserOnCalls`: `o = response.OnCalls` overwrote results each
   iteration instead of appending. Only the last page's on-calls
   were kept.

Predecessors: [008-enhanced-mock-client.md](008-enhanced-mock-client.md),
[006-add-api-timeouts.md](006-add-api-timeouts.md)

## Plan

1. Reset `opts.Offset = defaultOffset` at the start of each team's
   inner loop
2. Change `o = response.OnCalls` to
   `o = append(o, response.OnCalls...)`
3. Extend mock with `ListOnCallsResponses` queue and
   `ListMembersOffsets` tracking
4. TDD: 3 tests using enhanced mock pagination

## Files Modified

- `pkg/pd/pd.go` — offset reset, append fix
- `pkg/pd/pd_test.go` — 3 new tests
- `pkg/pd/mock.go` — `ListOnCallsResponses`, `ListMembersOffsets`,
  `ListOnCallsWithContext`

## Verification

- `TestGetTeamMemberIDs_PaginatedTeam` collects all pages
- `TestGetUserOnCalls_MultiplePages` appends all on-calls
- `go test ./...` passes

## Lessons Learned

- Conflict with PR #148 (enhanced mock) required rebase. Both PRs
  modified `mock.go`. Resolution: merge both sets of fields and
  methods, using HEAD's `recordCall` naming and `ListMembersResponse`
  type, while adding the pagination PR's `ListOnCallsResponses`
  and `ListMembersOffsets`. Also needed to wrap test data in
  `ListMembersResponse{Response: &resp}` to match the type.
