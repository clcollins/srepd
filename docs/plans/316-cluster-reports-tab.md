# Plan: Add cluster reports tab via backplane API (#316)

**Status:** Complete
**Issue:** #316
**PR:** #325

## Problem

CORA (Cluster Observability and Remediation Agent) generates diagnostic
reports stored via the backplane API. SREs need to view these reports
during incident triage alongside existing service logs and cluster info.

## Solution

Add a new "Reports" tab (tab 6) that fetches cluster reports from the
backplane API using direct HTTP calls. No `backplane-cli` dependency —
reads `~/.config/backplane/config.json` for proxy config and resolves
the backplane URL from OCM environment metadata.

### Key design decisions

- **Direct HTTP over backplane-cli imports**: The API surface is simple
  (GET with Bearer token), and avoiding the dependency keeps the binary
  lean and avoids transitive dependency bloat.
- **URL resolution from OCM**: The standard backplane config has an empty
  `url` field. The URL is resolved at runtime from the OCM environment
  metadata via `conn.ClustersMgmt().V1().Environment().Get().Send()`.
- **Token freshness via closure**: The backplane client takes a
  `tokenFunc func() (string, error)` that calls `ocmClient.GetAccessToken()`
  at request time, ensuring the OCM SDK handles token refresh.
- **Deferred client creation**: When OCM auth is pending (async browser
  flow), the backplane config is stored and the client is created when
  `OCMClientReadyMsg` arrives.

## Files changed

- `pkg/backplane/backplane.go` — types and BackplaneClient interface
- `pkg/backplane/config.go` — config loader for ~/.config/backplane/config.json
- `pkg/backplane/client.go` — HTTP client with Bearer auth and proxy support
- `pkg/backplane/mock.go` — mock client for dev mode and testing
- `pkg/backplane/client_test.go` — 8 tests (92% coverage)
- `pkg/backplane/config_test.go` — 4 tests
- `pkg/backplane/mock_test.go` — 5 tests
- `pkg/ocm/ocm.go` — added GetAccessToken() and GetBackplaneURL() to interface
- `pkg/ocm/client.go` — implemented GetAccessToken() and GetBackplaneURL()
- `pkg/ocm/mock.go` — mock implementations
- `pkg/tui/model.go` — backplane fields, updated constructors, cache cleanup
- `pkg/tui/ocm_enrichment.go` — clusterReportsMsg type and getClusterReports()
- `pkg/tui/views.go` — tabReports constant, renderClusterReportsTab(), tab bar
- `pkg/tui/tui.go` — message handler, phase 2 dispatch, deferred client creation
- `cmd/root.go` — backplane client initialization with URL resolution
- `cmd/config.go` — nil backplane params for config mode
- `testdata/fixtures/clusterreports.json` — dev mode fixture data

## Post-mortem / lessons learned

- The backplane config file commonly has `"url": ""` — the URL is resolved
  from OCM environment metadata, not from the config. Initial implementation
  rejected empty URLs, which broke on real-world configs. Lesson: always
  test with real config files early.
