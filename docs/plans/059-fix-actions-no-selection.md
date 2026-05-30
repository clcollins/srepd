# 059: Fix actions executing without selected incident

**Issue**: #224
**Branch**: srepd/fix-actions-no-selection
**Status**: Complete

## Problem

Actions triggered when there is no incident selected execute anyway instead of
returning a "no incident selected" status message. For example, pressing Enter
when no incident is highlighted opens an incident viewer with empty/placeholder
data instead of showing a status message.

## Root Cause

Several key handlers in `switchTableFocusMode` and `switchIncidentFocusMode`
were missing the `SelectedRow() == nil` guard or `selectedIncident == nil` check
before proceeding with their action.

## Changes

### `pkg/tui/msgHandlers.go`

**Table focus mode (`switchTableFocusMode`):**
- **Enter key**: Added `SelectedRow() == nil` check before opening the incident
  viewer. Previously would open a viewer with empty/loading placeholder data.
- **SOP key**: Added the full three-step guard pattern (SelectedRow check, sync,
  selectedIncident check) that was already used by Ack, Silence, UnAck, Note,
  and Open handlers.

**Incident focus mode (`switchIncidentFocusMode`):**
- **Refresh**: Added `selectedIncident == nil` guard to prevent nil pointer
  dereference when accessing `m.selectedIncident.ID`.
- **Ack**: Added `selectedIncident == nil` guard.
- **UnAck**: Replaced empty-string fallback pattern with early-return guard.
  Previously would show a confirmation prompt with an empty incident ID.
- **Silence**: Same fix as UnAck.
- **Note**: Added `selectedIncident == nil` guard before the `incidentDataLoaded`
  check.
- **Login**: Added `selectedIncident == nil` guard before the
  `incidentAlertsLoaded` check.
- **Open**: Added `selectedIncident == nil` guard before the `incidentDataLoaded`
  check.
- **SOP**: Added `selectedIncident == nil` guard before the
  `incidentAlertsLoaded` check.

### `pkg/tui/msgHandlers_test.go`

Added tests for every guarded path:
- `TestTableMode_EnterKeyWithNoRows`
- `TestTableMode_SOPKeyWithNoRows`
- `TestTableMode_SOPKeyWithNoSelectedIncident`
- `TestTableMode_LoginKeyWithNoRows`
- `TestIncidentViewMode_AckWithNilSelectedIncident`
- `TestIncidentViewMode_UnAckWithNilSelectedIncident`
- `TestIncidentViewMode_SilenceWithNilSelectedIncident`
- `TestIncidentViewMode_NoteWithNilSelectedIncident`
- `TestIncidentViewMode_LoginWithNilSelectedIncident`
- `TestIncidentViewMode_OpenWithNilSelectedIncident`
- `TestIncidentViewMode_SOPWithNilSelectedIncident`
- `TestIncidentViewMode_RefreshWithNilSelectedIncident`

## Testing

All tests pass with `make test-all`. No behavior changes for normal use cases
where an incident is properly selected.
