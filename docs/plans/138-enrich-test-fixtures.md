# Plan 138: Enrich test fixtures with realistic data

## Problem

The dev mode test fixtures had minimal enrichment data. Most incident
view tabs (Service Logs, Limited Support, PD History, Notes, Cluster
Reports) were empty or had data for only one or two incidents, making
the dev demo less useful for testing the full incident view UI.

## Approach

1. Queried real PagerDuty alerts from the PASPK4G team over the past
   30 days to get authentic alert structures (etcdDatabaseQuotaLowSpace,
   ClusterOperatorDown, console-ErrorBudgetBurn, cluster missing, etc.)
2. Sanitized all customer data: cluster IDs → `fake-uuid-test` format,
   service names → `example.org`, IPs → `10.X.X.X`, URLs → `example.org`
3. Populated enrichment fixtures (service logs, limited support, cluster
   reports, notes) keyed by the correct internal cluster IDs so the
   MockClient returns them during dev mode enrichment
4. Added 8 resolved historical incidents with `first_trigger_log_entry`
   channel data so the PD History tab can match them to current incidents
   via cluster ID
5. Extended `fixtureIncident` with an optional `first_trigger_log_entry`
   field so fixture incidents can carry the channel metadata that the
   prior-alert scanner reads

## Key design decisions

- Fixture `servicelogs.json` and `limitedsupport.json` are keyed by
  OCM internal ID (e.g. `cluster-osd-001`) because `MockClient.GetServiceLogs`
  receives the internal ID from the phase-2 enrichment flow
- Historical incidents carry `first_trigger_log_entry.channel.details.cluster_id`
  so `matchIncidentToCluster` can match non-HCP incidents (HCP incidents
  already match via UUID in the title)
- Alert names and structures preserved from real PD data; only identifiers
  and URLs sanitized

## Files changed

- `testdata/fixtures/incidents.json` — added 8 resolved historical incidents
- `testdata/fixtures/alerts.json` — unchanged (already had good data)
- `testdata/fixtures/notes.json` — notes for INC_001, 003, 009, 010, 012
- `testdata/fixtures/servicelogs.json` — service logs for 5 clusters
- `testdata/fixtures/limitedsupport.json` — LS reasons for 3 clusters
- `testdata/fixtures/clusterreports.json` — CORA reports for 5 clusters
- `pkg/pd/dev.go` — `fixtureFirstTriggerEntry` struct, wiring in converter
- `pkg/pd/dev_test.go` — updated counts for new fixture data
