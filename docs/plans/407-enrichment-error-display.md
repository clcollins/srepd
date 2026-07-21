# 407: Surface enrichment fetch errors inline in tab views

## Problem

When backplane `ListReports` returns a 403 (or any error), the error is
silently swallowed: the handler logs at Debug level and returns nil. The
Reports tab stays stuck on "Loading cluster reports..." forever. The same
silent-swallow pattern affects Service Logs and Limited Support tabs.

Users have no indication that enrichment data failed to load.

## Approach

Store per-section errors in the model and display them as red (Warning
color) text in the tab where data was expected, with a concise summary
and "see logs for details" hint. Bypass glamour markdown rendering for
error content so lipgloss styling is preserved.

Also fix an existing bug: enrichment handlers did not dispatch
`renderIncidentMsg` on success, so the viewport was not re-rendered until
the user manually switched tabs.

## Changes

### Model (`model.go`)
- Added `serviceLogErrors`, `limitedSupportErrors`, `clusterReportErrors`
  maps (`map[string]error`) alongside existing data caches.

### Theme (`theme.go`)
- Added `InlineError` style: Warning color (#a4133c) foreground on
  default background.

### Handlers (`tui.go`)
- `ocmServiceLogsMsg`, `limitedSupportMsg`, `clusterReportsMsg` handlers:
  - Store errors in the new error maps on failure.
  - Upgraded log level from Debug to Warn.
  - Dispatch `renderIncidentMsg` on both success and error.

### Render pipeline (`commands.go`, `views.go`)
- `renderTabContent()` returns `(string, bool, error)` where the bool
  indicates pre-rendered content that should skip glamour.
- `renderIncident()` skips `renderIncidentMarkdown()` when preRendered.
- New `renderEnrichmentError()` helper formats concise red error text.
- Service Logs, Limited Support, and Reports tabs check error maps and
  return styled error text instead of perpetual loading messages.

### MCP config (`.mcp.json`)
- Moved tui-mcp MCP server config from `.claude/settings.json` (wrong
  location) to `.mcp.json` at project root (correct location for
  project-scoped MCP servers).

## Testing

- Unit tests for error storage in all three handlers.
- Unit tests for error display in all three tab render functions.
- Unit tests for preRendered flag and glamour bypass.
- Existing tests updated for new signatures.
- Full CI suite: fmt-check, vet, lint, test, test-race all pass.
- Visual verification via tui-mcp: no regressions in happy path.
