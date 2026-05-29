# Plan 045: Alert View Cleanup

## Problem

The incident detail view renders every field from the PagerDuty alert body
unfiltered, creating 15-30 lines of noise per alert. The essential information
(alert name, cluster, SOP link) is already extracted into named fields but gets
buried under a raw `Details` dump. URLs appear as plain text instead of
clickable markdown links that glamour can render as OSC 8 hyperlinks.

## Analysis

A review of 2,044 PagerDuty incidents identified 6 alert types with different
data structures. The current template iterates over `$alert.Details` (the full
`body["details"]` map), dumping every key-value pair. The useful fields --
`alert_name`, `cluster_id`, and `link` -- are already extracted by
`summarizeAlerts()` into dedicated struct fields (`Name`, `Cluster`, `Link`).

The `ToLink` template function already exists and is used for the incident
title. It produces `[text](url)` markdown that glamour renders as clickable
terminal hyperlinks.

## Changes

### Template (`incidentTemplate` in `pkg/tui/views.go`)

- Remove the entire `Details :` section and its `{{ range }}` over
  `$alert.Details`
- Replace the raw alert ID/status/URL block with a `###` heading using the
  alert name (falling back to alert ID when name is empty)
- Format SOP link using `ToLink` with `_none_` fallback for missing SOPs
- Format PagerDuty alert URL using `ToLink`
- Reorder fields: alert name heading first, then cluster, SOP, alert link,
  service, created

### Struct cleanup (`alertSummary` in `pkg/tui/views.go`)

- Remove the `Details map[string]interface{}` field from `alertSummary` since
  nothing else references it after the template change
- Remove the Details extraction code from `summarizeAlerts()`

### Tests (`pkg/tui/views_test.go`)

- `TestIncidentTemplate_AlertRendersAsMarkdownLink` -- verifies SOP and alert
  URLs render as `[text](url)` format
- `TestIncidentTemplate_AlertRendersSOPNoneWhenMissing` -- verifies `_none_`
  shown when SOP link is empty
- `TestIncidentTemplate_NoDetailsSection` -- verifies no "Details" text in
  output
- `TestIncidentTemplate_AlertNameAsHeading` -- verifies `###` heading format
- `TestIncidentTemplate_AlertWithEmptyName` -- verifies fallback to alert ID
  when name is empty
- `TestSummarizeAlerts_NoDetailsField` -- verifies extraction still works
  without Details field

## What is NOT changed

- The `summarizeAlerts()` extraction of `Name`, `Cluster`, `Link` fields
- The incident header section (title, service, urgency, etc.)
- The notes section
- The `ToLink` or other template functions
- The `getDetailFieldFromAlert` helper function
- Backward compatibility with alerts missing Name, Link, or Cluster

## Testing

- TDD approach: tests written first, verified to fail against old template,
  then implementation made to pass
- All existing tests continue to pass
- `go vet` and `gofmt` clean
