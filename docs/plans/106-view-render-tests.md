# Plan 106: View-layer render tests for cluster reports tab

**Branch:** `srepd/view-render-tests`

## Problem

`renderClusterReportsTab` (`pkg/tui/views.go`) was the only fully-untested (0%)
render function in the TUI package. View-layer rendering was under-covered, and its
output (branching on backplane/OCM availability, report sorting, base64 decoding) was
not locked in by any test.

## Solution

Add deterministic, fully-mocked render tests that drive `renderClusterReportsTab` with
model state only — no PagerDuty/OCM/backplane API calls, no host tools, no filesystem
(per the project's test-isolation rule). Every branch of the function is covered:

- backplane disabled (nil client)
- OCM not connected / OCM auth pending / loading (empty cache)
- cache present but no reports → "No cluster reports"
- with data: report numbering, summaries, newest-first sort, base64 `Data` decoded
- invalid base64 → raw fallback

These are characterization tests: they describe the desired verified behavior of an
existing, correct function, so they pass immediately and guard against regressions.

## Files Modified

- `pkg/tui/views_render_test.go` (new) — 7 tests + small helpers.

## Verification

- `renderClusterReportsTab` coverage: **0% → 100%**; `pkg/tui` package 64.0% → 64.9%.
- `make test-all` green (fmt, vet, lint, test, race, test-fixtures).

## Lessons Learned

- `renderClusterReportsTab` renders reports via `sortedClusterIDs()`, which requires a
  `selectedIncident`, an `incidentClusterMap` entry, **and** the cluster present in
  `clusterCache` — not just entries in `clusterReportCache`. Test setup must satisfy
  all three (reuse `setupModelWithCluster` + seed `clusterCache`) to reach the
  with-data path.
- View functions that are pure over model state are cheap, deterministic coverage —
  drive them directly rather than through a full `tea.Program`.
