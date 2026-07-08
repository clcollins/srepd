# Plan: Lazy enrichment, Info logging, and journal-aware log viewer (#337)

**Status:** Complete
**PR:** #337

## Problem

Three separate issues addressed together:

1. **Slow incident loading**: All incident data (alerts, notes, OCM, reports,
   PD history) was fetched only when the user opened an incident. First open
   always had a loading delay.

2. **Silent Info log**: Default log level was Warn, filtering out all Info
   messages. Key events (new incidents, enrichment completions, user actions)
   were invisible in the journal.

3. **Broken log viewer**: Ctrl+L only read the on-disk log file, showing
   nothing when logging to the systemd journal. Also showed old session data
   and didn't wrap long lines.

## Solution

### Lazy background enrichment

A scheduled job fires every 3 seconds and enriches one un-cached incident
via the existing `getIncidentMsg` cascade. Uses spiral ordering from the
highlighted row (highlight first, then +1, -1, +2, -2, ...) so nearby
incidents are ready first. The first incident is enriched immediately when
the incident list loads, no 3-second wait.

### Info logging

Changed default log level from Warn to Info. Promoted key events:
- Incident details/alerts/notes fetched (with counts)
- New incidents appearing, incident count changes
- OCM cluster enrichment (with name/region)
- Service logs, limited support, reports fetched (with counts)
- PD history completion (with totals)
- Reassigned incidents

### Journal-aware log viewer

- Detects log destination (`journal` vs `file`) and reads from the right source
- Journal: runs `journalctl _COMM=srepd --since <startupTime>`
- File: filters lines by timestamp to current session only
- Both paths wrap long lines to viewport width

### PD History cluster matching fix

Removed unconditional Tier 1 match for hive services — was matching ALL
hive incidents from any cluster in team-wide scan. Now always verifies
cluster_id via title or log entry extraction.

## Key files

| File | Change |
|------|--------|
| `pkg/tui/commands.go` | `lazyEnrichMsg`, `pickNextEnrichment`, `readJournalLog`, `readLogFile` session filter |
| `pkg/tui/lazy_enrich_test.go` | Tests for spiral ordering, cache skip, empty table, no config |
| `pkg/tui/model.go` | `logDestination`, `startupTime`, `readLog()`, lazy enrichment job |
| `pkg/tui/tui.go` | Info log promotions, `wrapLines`, lazy enrich handler, immediate enrichment on load |
| `pkg/tui/version.go` | `LogDestination` package variable |
| `cmd/root.go` | Set `LogDestination`, change default level to Info |
| `pkg/tui/prior_alerts.go` | Fix `matchIncidentToCluster` — always verify cluster |
