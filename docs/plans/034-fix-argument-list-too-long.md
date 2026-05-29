# Fix argument list too long with large alert payloads

## Context

When an incident has 25+ alerts, the ALERT_DETAILS base64 blob
exceeds Linux ARG_MAX (~2MB) when passed as a `-e` command-line
argument to the terminal. This causes "fork/exec: argument list
too long" and blocks cluster login for large incidents.

Predecessor: [033-multi-cluster-selection.md](033-multi-cluster-selection.md)

## Plan

Add size-aware compact fallback to alert data serialization:
1. Try full encoding first (preserves all PagerDuty data)
2. If encoded size exceeds 100KB, fall back to compact mode
3. Compact mode keeps only essential fields per alert: ID,
   alert_name, cluster_id, link, status, HTMLURL
4. A `compact: true` flag signals the format to consumers

## Files Modified

- `pkg/tui/commands.go` — compact types, compactAlerts(),
  size check in login() serialization
- `pkg/tui/commands_test.go` — 3 tests for compactAlerts

## Verification

- `TestCompactAlerts_ExtractsEssentialFields` passes
- `TestCompactAlerts_EmptyAlerts` passes
- `TestCompactAlerts_MissingBodyFields` passes
- Full test suite passes
