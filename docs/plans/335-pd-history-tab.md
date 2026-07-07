# Plan: Add PD History tab and incident view improvements (#335)

**Status:** Complete
**PR:** #335

## Problem

When investigating an incident, SREs need to quickly see whether the same
alert has fired before for this cluster and what other alerts have recently
fired. This context helps distinguish recurring issues from one-off problems
and speeds up root cause analysis. The same cluster can fire alerts through
different PD services (DMS, RHOBS, hive), so searching by service ID alone
misses cross-service history.

Additionally, the Reports tab only showed summaries (no report content),
loading states had no visual indicator, and unit tests were hitting real
external resources (systemd journal, shell commands, installed binaries).

## Solution

### PD History tab

Add a "PD History" tab (index 7) that fetches prior PagerDuty incidents for
the same cluster, split into "Same Alert" and "Other Alerts" tables. The
fetch uses a three-tier cluster matching strategy to avoid N+1 `GetAlerts`
API calls:

- **Tier 1** (`osd_hive`): Service is 1:1 with a cluster — every incident
  from that service matches without any filtering.
- **Tier 2** (`rhobs_hcp`): Cluster UUID is in the incident title
  (`for HCP: <uuid>`) — client-side `strings.Contains` match.
- **Tier 3** (all others): Extract `cluster_id` from
  `FirstTriggerLogEntry.Channel.Raw` via `include[]=first_trigger_log_entries`
  on the `ListIncidents` API call.

This reduces API calls from ~101/week (1 list + 100 alert fetches) to
~1-2/week (just paginated list). The fetch runs in sequential weekly chunks
over 90 days, with progressive rendering as each week completes.

### Other improvements

- **Full cluster report content**: `GetReport` fetches each report's Data
  field (base64-decoded) instead of just listing summaries.
- **Loading spinners**: All incident view tabs show animated spinners while
  data is loading.
- **Service log filtering**: Info-severity entries filtered from SLs tab.
- **Snapshot versioning**: `.goreleaser.yaml` uses `{{.Version}}` (includes
  SNAPSHOT suffix for local builds) so the update banner triggers correctly.
- **Test isolation**: Journal tests use mock sender, exec tests use re-exec
  pattern, integration tests gated behind build tag, source-reading tests
  use `go:embed`.

## Key files

| File | Change |
|------|--------|
| `pkg/tui/prior_alerts.go` | New: three-tier matching, weekly chunking, sequential dispatch |
| `pkg/tui/prior_alerts_test.go` | New: tests for all three tiers, rendering, tab constants |
| `pkg/tui/views.go` | PD History tab rendering, spinners on all tabs, SL filtering, report decoding |
| `pkg/tui/tui.go` | Message handler, fetch trigger, sequential week chaining |
| `pkg/tui/ocm_enrichment.go` | Full report fetching via GetReport |
| `pkg/tui/model.go` | Cache fields for prior alerts |
| `pkg/pd/mock.go` | ListIncidentsResponses queue, per-incident alert responses |
| `cmd/root.go` | journalWriter DI for testability |
| `.goreleaser.yaml` | Snapshot version uses git hash |

## Lessons learned

- PagerDuty's `include[]=first_trigger_log_entries` is the key to avoiding
  N+1 API calls — it inlines the first alert's payload data with each
  incident in the list response.
- Large SRE teams generate hundreds of incidents per week. Sequential
  weekly chunking is essential to avoid rate limiter starvation (429s).
- The `Channel.Raw` map in the PD Go SDK stores the entire JSON object
  from the trigger log entry, including `details` and `custom_details`
  with alert payload fields like `cluster_id`.
