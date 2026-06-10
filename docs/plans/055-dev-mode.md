# Plan 055: Dev Mode with DevPagerDutyClient

**Issue**: #201
**Branch**: srepd/dev-mode-v2
**Status**: In Progress

## Problem

srepd requires a live PagerDuty connection for development and UI iteration.
Developers need a way to run the TUI with realistic fixture data without
a PagerDuty API token.

## Solution

Add a `--dev` / `-D` CLI flag (and `SREPD_DEV=true` env var) that:

1. Skips token validation and PagerDuty API initialization
2. Loads JSON fixture data from `testdata/fixtures/`
3. Uses a `DevPagerDutyClient` with in-memory mutable state
4. Logs instead of launching terminals for cluster login

### Components

| File | Action |
|------|--------|
| `pkg/pd/dev.go` | New: DevPagerDutyClient with 12 interface methods |
| `pkg/pd/dev_test.go` | New: 7+ TDD tests |
| `testdata/fixtures/incidents.json` | New: 12 incident scenarios |
| `testdata/fixtures/alerts.json` | New: alert data keyed by incident ID |
| `testdata/fixtures/notes.json` | New: notes keyed by incident ID |
| `testdata/fixtures/config.json` | New: user, teams, escalation policies |
| `cmd/root.go` | Modify: --dev flag, init bypass |
| `cmd/config.go` | Modify: skip validation in dev mode |

### Fixture Scenarios (12 incidents covering 6 alert types)

1. osd_hive single alert
2. osd_hive multi-alert (3 alerts same cluster)
3. appsre alert
4. rhobs_hcp alert
5. rhobs_infra alert (no cluster_id)
6. deadmanssnitch alert
7. cee_escalation (zero alerts)
8. 25+ alerts for large payload testing
9. Multi-cluster incident (3 different cluster_ids)
10. Already acknowledged incident
11. Title mutations with bracket prefixes
12. Incident with 3 notes from different users

### Write Operations (In-Memory State Changes)

- Acknowledge: status -> "acknowledged", add to Acknowledgements
- Silence: update EscalationPolicy
- Re-escalate: update Assignments
- Add note: append to in-memory notes map

## Test Plan

- [ ] TestDevClient_ListIncidents - returns fixture incidents
- [ ] TestDevClient_GetIncident - returns specific incident
- [ ] TestDevClient_AcknowledgeUpdatesState - ManageIncidents modifies status
- [ ] TestDevClient_AddNoteAppendsToList - CreateNote adds to map
- [ ] TestDevClient_ListIncidentsAfterAck - state persists across calls
- [ ] TestDevClient_SilenceUpdatesPolicy - policy changes in memory
- [ ] TestLoadFixtures - JSON files load correctly
- [ ] All existing tests continue to pass

## Lessons Learned

**GENUINE ERROR — Go map iteration caused non-deterministic incident ordering**
(Fixed by: [059-fix-dev-mode-reorder.md](059-fix-dev-mode-reorder.md))

DevPagerDutyClient stored incidents in a `map[string]*pagerduty.Incident`
and iterated over it in `ListIncidentsWithContext`. Go map iteration is
non-deterministic, so after any state mutation (acknowledge, silence),
the incident list reordered unpredictably in the TUI.

Why it wasn't caught: tests did not verify iteration order stability
across mutations — they only checked that the correct incidents were
present, not their order.

Prevention: any Go code that iterates a map for user-visible list
output should use an order-preserving data structure (e.g., a separate
`[]string` for keys). Review should flag map iteration in display paths.

---

**GENUINE ERROR — incomplete dev fixtures caused rendering discrepancies**
(Fixed by: [066-fix-dev-fixture-display.md](066-fix-dev-fixture-display.md))

Dev mode fixtures were missing fields (`html_url`, incident references)
that real PagerDuty API responses include. This caused incident viewer
tabs to render differently in dev mode vs production, undermining the
purpose of dev mode for UI iteration.

Why it wasn't caught: fixtures were written to cover the fields needed
at implementation time, not to mirror the complete API response
structure. No comparison against real (sanitized) API responses was
done.

Prevention: when creating API fixtures, compare against a real API
response (sanitized) to ensure structural completeness. Dev mode
fixtures must mirror production response structure, not just the subset
of fields currently consumed.
