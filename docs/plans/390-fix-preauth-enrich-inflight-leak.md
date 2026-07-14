# 390: Fix stuck OCM tab spinners for incidents viewed before OCM auth

Branch: `srepd/fix-preauth-enrich-inflight-leak`

## What

When srepd starts unauthenticated to OCM ("OCM tokens not valid — will
authenticate async") and the user opens an incident before auth completes,
the Cluster / Service Logs / Limited Support History / Reports tabs spin
forever — even after OCM auth succeeds and other incidents enrich normally.

Observed live with incident Q14TKHUE995O2B: journal showed "OCM connected
(async)" followed by enrichment for exactly the clusters of incidents *not*
yet viewed; the viewed incident's cluster never got a single OCM call, with
no error logged.

## Root cause

`gotIncidentAlertsMsg` marks each uncached cluster
`clusterEnrichInFlight[id] = true` *before* dispatching enrichment, but
`enrichClusters()` silently returns nil when the OCM client is nil. No
command is dispatched, so no `clusterInfoMsg` (the only handler that clears
the flag) ever arrives — the flag leaks permanently.

Every subsequent enrichment opportunity — including the `OCMClientReadyMsg`
post-auth sweep — deliberately skips in-flight clusters, so the leaked flag
blocks recovery forever. The tab spinners render directly off
`clusterEnrichInFlight`, and phase 2 (service logs, limited support,
reports) only dispatches from the `clusterInfoMsg` handler, so all four
tabs stay in the spinner state.

## Approach

Restore the invariant: **a cluster is marked in-flight only when an
enrichment command was actually dispatched.** In `gotIncidentAlertsMsg`,
skip the mark-in-flight loop entirely when `m.ocmClient == nil`. The
cluster IDs still land in `incidentClusterMap`, so the existing
`OCMClientReadyMsg` sweep picks them up naturally once auth completes —
no new recovery path needed.

Considered instead clearing/ignoring in-flight flags in the
`OCMClientReadyMsg` handler (nothing can genuinely be in flight before a
client exists). Rejected in favor of fixing the invariant at the source:
never set the flag without a dispatched command, so no other current or
future consumer of the flag can be poisoned.

## Tests (TDD — written first, verified red, then green)

- `TestGotIncidentAlertsMsg_NilOCMClient_NoInFlightLeak` — alerts arriving
  with a nil client must not mark the cluster in-flight, but must still
  record it in `incidentClusterMap` for the post-auth sweep.
- `TestOCMClientReadyMsg_EnrichesClusterViewedPreAuth` — full regression
  sequence: alerts pre-auth, then `OCMClientReadyMsg`; asserts the sweep
  marks the cluster in-flight *and* actually dispatches a phase-1
  `clusterInfoMsg` command (batch unwrapped via a `collectCmdMsgs` helper
  with a timeout, since the batch also contains a 4s flash-notification
  tick).

## Files

- `pkg/tui/tui.go` — guard the mark-in-flight loop in `gotIncidentAlertsMsg`
  with `m.ocmClient != nil`
- `pkg/tui/ocm_enrichment_test.go` — two regression tests plus
  `alertsForCluster` / `collectCmdMsgs` helpers

## Known follow-up (not in this PR)

The phase-2 msg handlers (`ocmServiceLogsMsg` / `limitedSupportMsg` /
`clusterReportsMsg`) don't trigger `renderIncidentMsg`, so an already-open
tab's body only refreshes on tab switch. Cosmetic; separate change.
