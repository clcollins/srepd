# 377: Fix infinite pagination loop in incident fetching

## Problem

srepd v1.5.0 fails on startup with `pd.GetIncidents(): failed to get
incidents: failed to read response body: context deadline exceeded` for users
on large teams (e.g. PASPK4G, 175 members) who don't have
`default_silent_escalation_policy` configured. Reported by Shawn Bai.

Root cause: `updateIncidentList` built `ListIncidentsOptions` with only
`TeamIDs` and `UserIDs` — no `Limit`, no `Statuses`. go-pagerduty's
`url:"limit,omitempty"` tag drops zero-valued fields, so PagerDuty defaulted
to `limit=25` across all statuses. `GetIncidents` advanced pagination via
`opts.Offset += opts.Limit`, adding zero — any `more=true` response refetched
page 1 forever until the 30-second context deadline expired.

Trigger condition: team query returns >25 incidents (any status). PASPK4G
currently has 27–29 open. Users with a silent escalation policy configured
happen to filter out enough assignees to stay under 25 results, masking the
bug.

## Constraints

- PagerDuty rejects request URIs over ~4096 bytes with HTTP 414.
  175 members × ~23 bytes/`user_ids[]` param ≈ 4KB. The prior per-team fix
  (commit 1d935de) fit by a few bytes; adding `limit=100&statuses[]=...`
  params to that URL 414s immediately (probe-verified against live API).
- Incident filtering must stay server-side per project requirements — no
  client-side assignee filtering.

## Approach

### 1. Chunk user_ids[] queries (pkg/tui/commands.go)

Rewrote `updateIncidentList` to:
- Start from `pd.NewListIncidentOptsFromDefaults()` (Limit=100,
  Statuses=triggered/acknowledged) as base options for every query
- Split the non-ignored member list into batches of ≤100 user IDs
  (`maxUserIDsInQuery` constant). 100 IDs ≈ 2.5KB, well under the 4KB limit
  even with limit/statuses/team params
- Issue one query per (team, chunk), merging results through the existing
  dedup map
- Empty member list (no team members returned) falls back to a single query
  with no user_ids, preserving parity with old behavior

New helper: `chunkStrings(items []string, size int) [][]string`

### 2. Guard all pagination loops against Limit=0 (pkg/pd/pd.go)

Added `if opts.Limit == 0 { opts.Limit = defaultPageLimit }` to all four
functions that paginate with caller-supplied options: `GetIncidents`,
`GetAlerts`, `GetUserOnCalls`, `GetTeamMemberIDs`. This is a defense-in-depth
fix — the callers now send correct limits, but the pagination loops themselves
can no longer spin on zero-increment offsets regardless of how they're called.

Reworded the HTTP 414 error message to describe the actual failure
("PagerDuty rejected the query URI as too long") rather than prescribing a
user action that may not apply.

### 3. Mock recording for test observability (pkg/pd/mock.go)

Added `RecordedListIncidentsOpts`, `RecordedListAlertsOpts`, and
`RecordedListOnCallOpts` fields to `MockPagerDutyClient`, populated in their
respective mock methods. Tests assert on the recorded options to verify
correct Limit, Statuses, TeamIDs, and UserIDs values.

## Testing

All tests written first (TDD), verified red before implementation.

### pkg/pd/pd_test.go (4 new tests)

- `TestGetIncidents_DefaultsLimitAndAdvancesOffset`: two-page mock,
  verifies Limit=100 applied and offset advances to 100
- `TestGetAlerts_DefaultsLimitWhenZero`: verifies Limit guard
- `TestGetUserOnCalls_DefaultsLimitWhenZero`: verifies Limit guard
- `TestGetTeamMemberIDs_DefaultsLimitWhenZero`: two-page mock, verifies
  offsets [0, 100]

### pkg/tui/commands_test.go (4 new subtests + 1 new test)

- "sends default limit and statuses on every query": verifies Limit=100,
  Statuses=triggered/acknowledged, correct TeamIDs/UserIDs
- "chunks large member lists across queries": 250 members → 3 queries of
  ≤100 each, dedup collapses to 2 unique incidents
- "queries once without user_ids when team has no members": empty
  TeamMembersByTeam → single query, no UserIDs
- Strengthened "excludes ignored users" subtest to assert recorded opts
- `TestChunkStrings`: nil input, under size, exact size, over size with
  remainder

### Live verification

- API probe confirmed: bare v1.5.0 opts → `more=true total=28`; chunked fix
  → chunk[0:100] + chunk[100:175], no 414, 28 unique merged
- End-to-end: fixed binary loaded 27 incidents in 2 seconds against the
  vulnerable token-only config

## Lessons learned from commit 1d935de

The June per-team 414 fix (1d935de) restructured queries to issue one request
per team instead of combining all teams. This solved the immediate 414 for
multi-team configs, but:

1. **Left the URL within bytes of the 4KB limit** for single large teams —
   adding any params (Limit, Statuses) pushed it over. The fix was validated
   against the specific failing config but not against the worst-case URL
   length with all default params populated.
2. **Did not set Limit or Statuses** on the options, preserving the zero-Limit
   infinite-loop vulnerability. The per-team restructuring changed *which*
   queries ran, not *how* they were built.
3. **No plan document existed** (predates the plan-doc requirement), so there
   was no recorded reasoning about URL budget, pagination behavior, or
   constraint margins to reference when this failure surfaced.

Takeaway: when fixing a URL-length issue, budget for the *maximum* encoded
URL including all params that could be populated, not just the params present
at failure time. And always leave margin — fitting "by a few bytes" means the
next feature or default change breaks it.
