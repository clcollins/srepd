# OCM Integration for Cluster Enrichment

## Context

Enrich incident data with cluster details, service logs, cluster
reports, and limited support history from the OCM API. Display
cluster names instead of service names, add new tabs to the
incident viewer, and clean up cached data on resolve/silence.

## Changes

- New pkg/ocm/ package with OCM client, mock, and types
- Browser-based OCM auth (like ocm-container)
- Eager async enrichment when alerts are loaded
- 4 new incident viewer tabs: Cluster, ClusterReports, ServiceLogs, LimitedSupport
- Impacted Clusters list on Details tab
- Display name overrides in incident table
- Cache cleanup on resolve/silence
- Debug logs with cluster UUID only, no customer data
- TDD: tests first for all components

## Verification

- make test-all passes
- OCM tabs display data when connected
- Dev mode shows fixture data
- Cache cleared on resolve/silence
