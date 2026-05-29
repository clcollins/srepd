# Plan 039: Replace ALERT_DETAILS blob with individual PAGERDUTY_* env vars

## Problem

When srepd launches ocm-container for a cluster login, it serializes the full
incident, all alerts, and all notes into a JSON blob, base64-encodes it, and
passes it as a single `-e ALERT_DETAILS=<huge_blob>` argument. With 25+ alerts
this can exceed the operating system's ARG_MAX limit, causing the login command
to fail silently or with a cryptic error.

Additionally, the base64 blob is opaque -- users inside ocm-container cannot
easily inspect the PagerDuty context without decoding and parsing JSON.

## Solution

Replace the monolithic base64-encoded `ALERT_DETAILS` env var with individual
small `PAGERDUTY_*` environment variables, each containing a simple string
value. Filter alerts to only those matching the selected cluster, so the
env var payload is proportional to one cluster's alerts rather than the entire
incident's alert set.

### New Environment Variables

| Variable | Source |
|----------|--------|
| `PAGERDUTY_INCIDENT_ID` | `incident.ID` |
| `PAGERDUTY_INCIDENT_TITLE` | `incident.Title` |
| `PAGERDUTY_INCIDENT_URL` | `incident.HTMLURL` |
| `PAGERDUTY_INCIDENT_SERVICE` | `incident.Service.Summary` |
| `PAGERDUTY_INCIDENT_URGENCY` | `incident.Urgency` |
| `PAGERDUTY_INCIDENT_STATUS` | `incident.Status` |
| `PAGERDUTY_CLUSTER_ID` | selected cluster ID |
| `PAGERDUTY_ALERT_COUNT` | count of alerts matching this cluster |
| `PAGERDUTY_ALERT_NAMES` | comma-separated alert names for this cluster |
| `PAGERDUTY_ALERT_LINKS` | comma-separated SOP links for this cluster |
| `PAGERDUTY_NOTES_EXIST` | "true" or "false" |
| `PAGERDUTY_NOTE_COUNT` | number of notes |

### Key Design Decisions

1. **Pure function**: `buildPagerDutyEnvVars()` is a pure function with no
   side effects, making it straightforward to unit test.

2. **Cluster filtering**: Only alerts whose `cluster_id` matches the selected
   cluster contribute to `ALERT_NAMES` and `ALERT_LINKS`. This keeps the
   payload small and relevant.

3. **SOP link extraction**: Uses the same `sopLinkFields` priority order as
   `getSOPLink()` -- checks "link" first, then "runbook_url".

4. **Renamed PAGERDUTY_INCIDENT**: The old `PAGERDUTY_INCIDENT` env var is
   renamed to `PAGERDUTY_INCIDENT_ID` for naming consistency.

## Changes

### Modified Files

- `pkg/tui/commands.go`:
  - Removed `alertData` struct
  - Removed base64/JSON serialization block from `login()`
  - Removed `encoding/base64` and `encoding/json` imports
  - Added `strconv` import
  - Added `buildPagerDutyEnvVars()` pure function
  - Updated `login()` to call `buildPagerDutyEnvVars()` with cluster ID from vars map
  - Renamed `PAGERDUTY_INCIDENT` to `PAGERDUTY_INCIDENT_ID`

- `pkg/tui/commands_test.go`:
  - Removed `TestLoginEnvironmentVariables` (tested old alertData serialization)
  - Removed `encoding/base64` and `encoding/json` imports
  - Added `strings` import
  - Added `envVarMap()` test helper
  - Added `TestBuildPagerDutyEnvVars_FullIncident`
  - Added `TestBuildPagerDutyEnvVars_FiltersByCluster`
  - Added `TestBuildPagerDutyEnvVars_NilIncident`
  - Added `TestBuildPagerDutyEnvVars_NoMatchingAlerts`
  - Added `TestBuildPagerDutyEnvVars_NotesExist`
  - Added `TestBuildPagerDutyEnvVars_NoNotes`
  - Added `TestBuildPagerDutyEnvVars_ManyAlerts`
  - Updated `TestLoginCommandStructureWithEnvVars` to use `PAGERDUTY_INCIDENT_ID`

## Testing

- All 7 new `TestBuildPagerDutyEnvVars_*` tests cover the full matrix:
  full data, cluster filtering, nil incident, no matching alerts, notes
  present/absent, and 50-alert stress test.
- Existing tests continue to pass unchanged.
- `make test-all` (fmt-check, vet, lint, test) passes cleanly.

## Risks

- **Breaking change for ocm-container consumers**: Any scripts inside
  ocm-container that parse `$ALERT_DETAILS` will need updating to use the
  new individual `$PAGERDUTY_*` variables instead.
- **Renamed variable**: `PAGERDUTY_INCIDENT` is now `PAGERDUTY_INCIDENT_ID`.
  Consumers referencing the old name need updating.
