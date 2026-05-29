# Multi-alert cluster selection

> Fixes #1.

## Context

When an incident has multiple alerts with different `cluster_id` values,
the login command (`l`) always used the first alert's cluster_id. Users
had no way to choose which cluster to log into.

The existing `loginMsg` handler had a TODO comment referencing Issue #1.

## Plan

1. Add a pure function `getUniqueClusters(alerts) []string` that extracts
   deduplicated cluster_ids from alerts, preserving first-appearance order
2. Write comprehensive unit tests for `getUniqueClusters` (TDD)
3. Add `clusterSelectMode` and `clusterSelectOptions` fields to model
4. Modify `loginMsg` handler to detect multiple unique clusters and
   enter cluster selection mode instead of defaulting to the first alert
5. Add `handleClusterSelectInput` key handler: digit keys 1-9 select a
   cluster, Escape cancels, other keys are ignored
6. Add `clusterSelectedMsg` message type that carries the chosen cluster
   and proceeds with the normal login flow
7. Show the cluster selection prompt in the header status area using
   the warning style (same pattern as confirmation prompts)
8. Clear cluster selection state on view transitions

## Files Modified

- `pkg/tui/commands.go` -- `getUniqueClusters`, `clusterSelectedMsg`
- `pkg/tui/model.go` -- `clusterSelectMode`, `clusterSelectOptions`,
  clear in `clearSelectedIncident`
- `pkg/tui/tui.go` -- modified `loginMsg` handler, added
  `clusterSelectedMsg` handler
- `pkg/tui/msgHandlers.go` -- `handleClusterSelectInput`, priority
  check in `keyMsgHandler`
- `pkg/tui/views.go` -- cluster select prompt in `renderHeader`
- `pkg/tui/commands_test.go` -- 8 tests for `getUniqueClusters`
- `pkg/tui/msgHandlers_test.go` -- 5 tests for cluster selection UX

## Verification

- `TestGetUniqueClusters_SingleCluster` -- one alert returns one cluster
- `TestGetUniqueClusters_MultipleDifferent` -- 3 alerts, 2 clusters
- `TestGetUniqueClusters_NoClusterID` -- alerts without cluster_id
- `TestGetUniqueClusters_Deduplication` -- same cluster deduplicated
- `TestGetUniqueClusters_PreservesOrder` -- order matches first appearance
- `TestGetUniqueClusters_EmptyAlerts` -- empty input
- `TestGetUniqueClusters_NilAlerts` -- nil input
- `TestGetUniqueClusters_MixedWithAndWithoutClusterID` -- mixed alerts
- `TestClusterSelect_DigitSelectsCluster` -- digit key selects cluster
- `TestClusterSelect_EscCancels` -- Escape cancels selection
- `TestClusterSelect_OutOfRangeIgnored` -- out-of-range digit ignored
- `TestClusterSelect_OtherKeysIgnored` -- non-digit keys ignored
- `TestClusterSelect_ClearedOnViewTransition` -- cleared on ESC/back
- `make fmt-check`, `make vet`, `make test` all pass
