# 409: Fix auto-open incident viewer regression from PR #407

## Problem

PR #407 changed enrichment message handlers (`ocmServiceLogsMsg`,
`limitedSupportMsg`, `clusterReportsMsg`) to return `renderIncidentMsg`
unconditionally after caching data. This causes the incident viewer to
open automatically on startup when background enrichment data arrives,
even though the user is still in the table view.

The `renderIncidentMsg` handler calls `renderIncident()`, which produces
`renderedIncidentMsg`, which sets `m.viewingIncident = true` — forcibly
switching from table to incident view without user interaction.

## Approach

Guard each `renderIncidentMsg` dispatch with `m.viewingIncident` — only
re-render the incident viewer if the user is already viewing it. The
enrichment data is still cached regardless, so it's available when the
user does open the incident.

## Changes

### Update loop (`tui.go`)
- `ocmServiceLogsMsg` handler: wrap both success and error render
  dispatches with `if m.viewingIncident`
- `limitedSupportMsg` handler: same
- `clusterReportsMsg` handler: same

### Tests (`ocm_tab_handlers_test.go`)
- Existing tests updated to set `m.viewingIncident = true` since they
  test the re-render behavior
- Added 3 new companion tests (`*_NoRerenderWhenNotViewing`) verifying
  that data is cached but no re-render command is returned when the user
  is in the table view
