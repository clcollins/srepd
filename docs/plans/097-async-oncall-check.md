# Plan 097: Async on-call check for auto-acknowledge

**Branch:** `srepd/async-oncall-check`

## Problem

The auto-acknowledge sweep in the `updatedIncidentListMsg` handler
(`pkg/tui/tui.go`) called `UserIsOnCall(m.config, ...)` **synchronously**. That
function issues a live PagerDuty `ListOnCalls` request via `pd.GetUserOnCalls`. Because
the Bubble Tea `Update` loop is single-threaded, this blocked the entire UI for the
duration of the request on **every 15s incident-list refresh** while `autoAcknowledge`
was enabled — freezing navigation and rendering until the API responded (up to the 30s
client timeout on a hung network).

This is a UI-blocking bug, not a timeout bug: `pd.GetUserOnCalls` is already wrapped
with `contextWithTimeout()`.

## Solution

Move the on-call check **off** the Update loop into a one-shot async command, and
**never cache** the result.

- New `checkOnCallAndAcknowledge(p *pd.Config, id string, candidates []pagerduty.Incident) tea.Cmd`
  in `commands.go`. It runs `UserIsOnCall` off-loop, filters `candidates` by the fresh
  on-call result via the existing `ShouldBeAcknowledgedCached`, and returns
  `acknowledgeIncidentsMsg{incidents: ...}` — or a new `noAcknowledgeMsg{}` no-op when
  the user is not on-call, the check fails, or nothing matches.
- In `updatedIncidentListMsg`, first compute the on-call-independent candidate list
  (`AssignedToUser && !AcknowledgedByUser`); **only if it is non-empty** dispatch the
  command. This avoids firing an on-call API call every refresh when nothing is
  assigned to the user.
- New `noAcknowledgeMsg` handler in the Update switch is a no-op. It is deliberately
  distinct from `acknowledgeIncidentsMsg`, whose `nil`-incidents fallback would
  acknowledge the *selected* incident — which must never happen from the background
  sweep.

### Why no cache (critical safety property)

On-call status is checked **live every refresh**, never cached:

1. **Auto-ack:** if a user leaves SREPD running past the end of their shift, a cached
   "on-call = true" would keep auto-acknowledging incidents that are no longer theirs.
   A live check stops auto-ack within one refresh cycle of going off-call.
2. **Re-escalate / reassign:** re-escalation almost always happens right after the user
   goes off-call and reassigns to someone else — exactly when a cached value would be
   wrong. Those paths (`reEscalateIncidents`/`reassignIncidents`) must always check
   live and must never read a cached value. (They don't check on-call today; this
   change does not add caching to them.)

There is intentionally **no on-call field on the model** — eliminating it structurally
prevents a future maintainer from wiring a stale value into any path.

## Files Modified

- `pkg/tui/commands.go` — `checkOnCallAndAcknowledge`, `noAcknowledgeMsg`.
- `pkg/tui/tui.go` — candidate computation + async dispatch in `updatedIncidentListMsg`;
  `noAcknowledgeMsg` no-op handler.
- `pkg/tui/commands_test.go` — 5 new tests + `drainCmd` helper.

## Tests (TDD)

Written first, seen red (undefined symbols), then green:
- `TestCheckOnCallAndAcknowledge_OnCall_ReturnsAckMsg` — on-call + candidates → ack msg.
- `TestCheckOnCallAndAcknowledge_NotOnCall_ReturnsNoOp` — off-call → no-op (NOT an ack).
- `TestCheckOnCallAndAcknowledge_OnCallError_ReturnsNoOp` — check error → no-op.
- `TestCheckOnCallAndAcknowledge_OnCallButNoneMatch_ReturnsNoOp` — no match → no-op.
- `TestUpdatedIncidentList_AutoAck_DispatchesFreshOnCallCheck` — regression: the
  Update path dispatches an async command that performs a *fresh* `ListOnCalls` call
  (asserts `CallCounts["ListOnCallsWithContext"] >= 1`) and acknowledges the
  assigned+unacked incident. No cached on-call state exists on the model.

Existing `TestUserIsOnCall_*` and `TestShouldBeAcknowledgedCached_*` remain valid and
unchanged.

## Verification

`make test-all` (fmt-check, vet, lint, test, test-race, test-fixtures) green.
Manual: with auto-ack on, the incident list no longer stalls the UI on refresh, and
going off-call stops auto-acknowledging within one refresh cycle.

## Lessons Learned

- A blocking network call anywhere in the Bubble Tea `Update` loop (or `View`) freezes
  the whole UI — always dispatch I/O as a `tea.Cmd`. `UserIsOnCall` was the last such
  inline call.
- `acknowledgeIncidentsMsg{incidents: nil}` is not a safe "nothing to do" signal — its
  handler falls back to the selected incident. Background sweeps must use an explicit
  no-op message (`noAcknowledgeMsg`).
- On-call is a freshness-critical signal; caching it introduces a safety bug at shift
  boundaries. The absence of a cache field is a deliberate invariant.
