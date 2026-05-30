# Plan 059: Fix Dev Mode Incident List Reordering

**Issue**: #222
**Branch**: srepd/fix-dev-mode-reorder
**Status**: Complete

## Problem

In `--dev` mode, acknowledging an incident causes the incident list to
reorder. `DevPagerDutyClient.ListIncidentsWithContext` iterates over a
Go map (`map[string]*pagerduty.Incident`), which has non-deterministic
iteration order. After any state mutation (acknowledge, silence,
re-escalate), the next list call returns incidents in a different order.

## Solution

Store incident IDs in a `[]string` slice (`incidentOrder`) alongside
the existing map. The slice preserves the original fixture insertion
order. `ListIncidentsWithContext` iterates over the slice instead of
the map, using the slice entries as keys into the map for O(1) lookup.

### Changes

| File | Action |
|------|--------|
| `pkg/pd/dev.go` | Add `incidentOrder []string` field; populate during init; iterate slice in `ListIncidentsWithContext` |
| `pkg/pd/dev_test.go` | Add `TestDevClient_ListIncidents_StableOrder` with 4 subtests |

### Test Coverage

- `returns_incidents_in_fixture_insertion_order` -- verifies exact ID order matches fixture file
- `order_is_stable_across_repeated_calls` -- 20 repeated calls produce identical order
- `order_is_preserved_after_acknowledging_an_incident` -- mutation does not change order
- `filtered_results_preserve_relative_order` -- status filtering keeps relative order intact
