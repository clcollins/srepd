# Plan 109: Fix re-escalate landing on level 1, make target level configurable

**Branch:** `srepd/reescalate-skip-level-1`

## Problem

`ctrl+e` "re-escalate" was intended to skip the placeholder first tier and jump the
incident to escalation **level 2** (the first real on-call human). In practice it
landed on **level 1** — "Nobody SREP", the placeholder user on the standard
"OpenShift Escalation Policy" (PA4586M).

**Root cause (confirmed against the live PD policy + API semantics):**
`pd.ReEscalateIncidents` sent **both** `escalation_policy` (re-assigning the incident's
*current* policy) **and** `escalation_level: 2` in the same Manage-Incidents request.
PagerDuty restarts escalation at **level 1** whenever `escalation_policy` is set, so
the policy re-assignment overrode the requested level — dropping the incident to level
1 ("Nobody").

Live policy shape (PA4586M), which motivates the level-2 default:
| Level | Delay | Target |
|---|---|---|
| 1 | 15 min | user **"Nobody SREP"** (placeholder) |
| 2 | 10 min | schedule: Weekend Oncall (first real human) |
| 3 | 10 min | schedule: Weekday Primary |
| … | … | … |

## Solution

1. **Omit `escalation_policy` for in-place re-escalation.** In
   `pd.ReEscalateIncidents`, only set `EscalationPolicy` when moving to a *different*
   policy (`policy.ID != incident.EscalationPolicy.ID`, e.g. silencing to a silent
   policy). When the target is the incident's current policy, send `EscalationLevel`
   only, so PD applies the level in place instead of resetting to level 1. Silence
   (which intentionally moves to a different policy) is unaffected.
2. **Make the target level configurable.** New optional config key `reescalate_level`
   (default `2`). Resolved via `resolveReescalateLevel()` (falls back to
   `reEscalateDefaultPolicyLevel` = 2 when unset/≤0), stored on the model, and used by
   the `unAcknowledgeIncidentsMsg` (re-escalate) handler instead of the hardcoded
   constant.

## Files Modified

- `pkg/pd/pd.go` — `ReEscalateIncidents` omits `EscalationPolicy` for same-policy
  re-escalation.
- `pkg/pd/mock.go` — `LastManageIncidentsOpts` records sent options for assertions.
- `pkg/pd/pd_test.go` — new same-policy / different-policy tests.
- `pkg/tui/model.go` — `reescalateLevel` field + `resolveReescalateLevel()`.
- `pkg/tui/tui.go` — re-escalate handler uses `m.reescalateLevel`.
- `pkg/tui/reescalate_test.go` — `resolveReescalateLevel` tests.
- `pkg/config/config.go` — `reescalate_level` in DefaultOptionalKeys/OptionalKeys.
- `cmd/config.go` — `reescalate_level` added to the safe-to-log allowlist.
- `README.md` — documents the `reescalate_level` key.

## Tests (TDD)

Written first, seen red, then green:
- `TestReEscalateIncidents_SamePolicy_OmitsPolicySendsLevelOnly` — in-place: sends
  `EscalationLevel: 2`, `EscalationPolicy` nil (failed on the old always-send code).
- `TestReEscalateIncidents_DifferentPolicy_SendsPolicy` — move: sends the policy.
- `TestResolveReescalateLevel` — default 2, configured value, zero/negative fallback.

Existing `TestReEscalateIncidents_*` and silence tests remain green.

## Verification

Live investigation via PagerDuty API (policy PA4586M rules confirm level 1 = "Nobody",
level 2 = first on-call, 10-min level-2→3 delay). `make test-all` green (fmt, vet,
lint, test, race, test-fixtures) on go1.26.5.

## Lessons Learned

- Setting `escalation_policy` on a PagerDuty incident restarts escalation at level 1.
  To bump an incident to a higher level *in place*, send `escalation_level` alone and
  omit `escalation_policy`. Sending both makes the policy win and the level a no-op.
- The bug was invisible in code review because the level *was* passed correctly — the
  defect was the co-sent policy field, a PD API-semantics interaction. Live
  verification against a real policy was required to diagnose it.
