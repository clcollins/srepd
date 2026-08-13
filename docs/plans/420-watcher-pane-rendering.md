# 420 — Watcher Pane Rendering Fixes

## Problem

Three rendering defects in the watcher/approvals panes:

1. **F1 — Approvals render below help bar**: expanded approvals appear
   below the bottom chrome, unwrapped, and never show `ask.Body`. Users
   can't review what they're approving.
2. **F2 — Markers repeated on every line**: `prefixLines` prepends the
   📡/🤖 marker to every line of a `---` block. Multi-line verdicts and
   streaming agent output get cluttered with duplicate emojis.
3. **F3 — Verdict markdown rendered raw**: watcher pane shows raw
   markdown syntax (`**bold**`, `` `code` ``) instead of rendered text.

## Approach

### F1 — Approvals in watcher pane slot

Expanded approvals render into the watcher pane slot (views.go ~121),
temporarily replacing the watcher viewport. `RenderExpanded` shows
wrapped titles, action+target lines using snapshotted
`ask.IncidentID`/`IncidentTitle` (never live `m.selectedIncident`),
and full `ask.Body`. Watcher expanded state is saved on open and
restored on close via `watcherWasExpanded`.

### F2 — One marker per block

Introduced `prefixMessage(marker, text)` which prepends the marker once
at the start of the block, replacing `prefixLines` at all call sites.
Both watcher and agent paths fixed identically.

### F3 — Glamour markdown rendering

Added `renderWatcherMarkdown` method that runs content through glamour
with the model's `GlamourStyle`. ASCII-mode guard falls back to plain
lipgloss wrapping in deterministic test environments. `stripControl`
security sanitisation is preserved at the buffer-append boundary.

## Key decisions

- **Security**: approval rendering uses snapshotted incident data from
  `Ask` struct, never reads live `m.selectedIncident`
- **ASCII guard**: `renderWatcherMarkdown` checks
  `lipgloss.ColorProfile() == termenv.Ascii` to prevent ANSI
  contamination in golden snapshot tests
- **No `prefixLines` removal**: kept as utility (still tested); all
  production call sites use `prefixMessage`

## Deliverables

| ID | Summary | Files | Commit |
|----|---------|-------|--------|
| T1 | Failing tests for F1, F2, F3 | `watcher_rendering_test.go` | `88c8dc8` |
| F2 | One marker per `---` block | `watcher.go`, `tui.go`, `claude.go`, `claude_test.go`, `view_render_test.go`, `watcher_integration_test.go` | `05f5259` |
| F3 | Glamour markdown rendering | `watcher.go` | `85b7d67` |
| F1 | Approvals in watcher pane slot | `views.go`, `approvals.go`, `model.go`, `msgHandlers.go` | `02aa812` |
| G  | Golden snapshots + ASCII guard + lint fix | `golden_test.go`, `watcher.go`, `watcher_rendering_test.go`, `approvals.go`, `model.go`, `testdata/*.golden` | `c4caa08` |

## Test traceability

| Test | File:Line | Covers |
|------|-----------|--------|
| `TestPrefixMessage_OneMarkerPerBlock` (6 subtests) | `watcher_rendering_test.go:16` | F2 |
| `TestWatcherBuffer_OneMarkerPerBlock_Integration` | `watcher_rendering_test.go:77` | F2 |
| `TestWatcherBuffer_StreamingSetLast_OneMarker` | `watcher_rendering_test.go:98` | F2 |
| `TestView_ApprovalsExpandedRendersInWatcherSlot` | `watcher_rendering_test.go:117` | F1 |
| `TestView_ApprovalsExpandedWrapsLongTitles` | `watcher_rendering_test.go:144` | F1 |
| `TestView_ApprovalsCollapsedShowsBadge` | `watcher_rendering_test.go:169` | F1 |
| `TestView_ApprovalsRestoredWatcherStateOnClose` | `watcher_rendering_test.go:182` | F1 |
| `TestUpdateWatcherViewport_RendersMarkdown` | `watcher_rendering_test.go:200` | F3 |
| `TestRenderApprovalsExpanded_ShowsBody` | `watcher_rendering_test.go:220` | F1 |
| `TestRenderApprovalsExpanded_ActionTargetLine` | `watcher_rendering_test.go:238` | F1 |
| `TestGolden_WatcherOneMarkerAgent` | `golden_test.go:193` | F2 golden |
| `TestGolden_WatcherOneMarkerWatcher` | `golden_test.go:203` | F2/F3 golden |
| `TestGolden_ApprovalsExpanded` | `golden_test.go:213` | F1 golden |

## Revert-check evidence

**F2** — reverted `prefixMessage` to delegate to `prefixLines`: 6 test
failures (marker count 3 instead of 1). Restored; all pass.

**F3** — reverted `renderWatcherMarkdown` to plain lipgloss wrapping:
build failure from unused glamour/termenv imports (function body removed).
Restored; all pass.

**F1** — reverted `views.go` approvals branch: 3 assertion failures
(header, title, body missing from view output). Restored; all pass.

## CI results

- `gofmt -s -l`: clean
- `go vet ./...`: clean
- `golangci-lint run`: 0 issues
- `go test ./pkg/tui/... -count=1`: PASS (47.8s)
- `go test -race ./pkg/tui/... -count=1`: PASS (96.0s)
- `cmd` package: pre-existing env-specific failure (`TestConfigureLogging_SetsLogWriter`
  — `/var/log/srepd.log` read-only) unrelated to this PR
