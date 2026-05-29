# 055: Eager Pre-fetch on Navigation

**Issue**: #198
**Branch**: `srepd/eager-prefetch`
**Status**: In Progress

## Problem

When a user presses 'l' (login) or views an incident, srepd must wait for full
incident data (details, alerts, notes) to load from PagerDuty. This creates a
visible "waiting for incident info..." delay because data is only fetched
on-demand when the action is triggered.

## Solution

Pre-fetch incident details in the background when the user navigates to a new
row (j/k/Up/Down/g/G). If the cache already has full data for the highlighted
incident, skip the fetch. Rate limiting (token bucket: 10 req/s, burst 20)
handles rapid scrolling naturally; no explicit debounce needed.

## Design

### Change to syncSelectedIncidentToHighlightedRow

Currently `syncSelectedIncidentToHighlightedRow()` is a void method that only
mutates model state. It will be refactored to return a `tea.Cmd` that, when the
highlighted incident is not fully cached, emits a `getIncidentMsg` to trigger
background fetching of incident details, alerts, and notes.

Signature change:
```go
// Before
func (m *model) syncSelectedIncidentToHighlightedRow()
// After
func (m *model) syncSelectedIncidentToHighlightedRow() tea.Cmd
```

### Callers updated

All callers in `msgHandlers.go` (`switchTableFocusMode` for Up/Down/Top/Bottom
keys, `switchIncidentFocusMode` for Escape, and `updatedIncidentListMsg` in
`tui.go`) will propagate the returned command.

### Cache guard

The pre-fetch only fires when the incident is NOT already fully cached
(dataLoaded AND alertsLoaded AND notesLoaded all true). The existing
`gotIncidentMsg`, `gotIncidentAlertsMsg`, and `gotIncidentNotesMsg` handlers
already guard against overwriting the currently-viewed incident with stale
background data.

## Tests (TDD)

1. **TestSyncSelectedIncident_TriggersPrefetch** - uncached incident returns a
   non-nil tea.Cmd
2. **TestSyncSelectedIncident_SkipsCached** - fully cached incident returns nil
3. **TestSyncSelectedIncident_TriggersPrefetchPartialCache** - partially cached
   (e.g. data loaded but alerts not) still triggers prefetch
4. **TestSyncSelectedIncident_NilRowReturnsNil** - no highlighted row returns nil
5. **TestSyncSelectedIncident_SameIncidentReturnsNil** - already-selected
   incident returns nil (no re-fetch)

## Files Modified

- `pkg/tui/model.go` - signature change + pre-fetch logic
- `pkg/tui/model_test.go` - new test cases
- `pkg/tui/tui.go` - propagate cmd from sync in updatedIncidentListMsg
- `pkg/tui/msgHandlers.go` - propagate cmd from sync in navigation handlers

## Post-mortem / Lessons Learned

(To be filled after merge)
