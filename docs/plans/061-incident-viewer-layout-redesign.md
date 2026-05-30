# Incident viewer layout redesign

## Context

Issue #235. The current incident viewer uses three top-level tabs
(Details, Alerts, Notes) where only one is visible at a time. This
forces the user to Tab away from the details to see alerts or notes,
losing context. The issue requests a split layout where details are
always visible at the top, and alerts/notes appear as selectable
sections below.

## Plan

### Model changes (`pkg/tui/model.go`)

- Replace `activeTab int` (0=details, 1=alerts, 2=notes) with
  `activeSection int` (0=alerts, 1=notes). Details are no longer a
  tab; they are always rendered.
- Keep `activeAlertIdx` and `activeNoteIdx` unchanged.

### View changes (`pkg/tui/views.go`)

- Remove `tabDetails`, `tabNotes`, `tabAlerts` tab constants and
  `tabCount`. Add `sectionAlerts = 0`, `sectionNotes = 1`,
  `sectionCount = 2`.
- Replace `renderTabHeader()` with `renderSectionHeaders()` that
  shows both Alerts and Notes headers with active/inactive markers
  (filled triangle for active, hollow for inactive) and item counts.
- Replace `template()` to always render the details block at the
  top, followed by the active-section alert or note content, then
  the inactive-section alert or note content.
- Remove the old `detailsTabTemplate`, `alertTabTemplate`,
  `noteTabTemplate` constants. Add new unified
  `incidentViewerTemplate` that renders details first, then both
  sections with their headers and current items.
- Add helper text `[Tab: cycle | Up/Down: select section]` next to
  section headers.

### Key handler changes (`pkg/tui/msgHandlers.go`)

- In `switchIncidentFocusMode`:
  - **Tab/Shift+Tab**: cycle items within the active section (next/prev
    alert or note), replacing the current tab-switch behavior.
  - **Up/Down arrows**: switch the active section between alerts and
    notes, replacing the current viewport-scroll behavior (viewport
    scrolling is handled by other keys like PgUp/PgDn inherently).
  - **1-9 number keys**: jump to specific item within the active section
    (unchanged behavior, but now they always know which section they
    operate on via `activeSection`).
  - **Left/Right arrows**: also cycle items within the active section
    (unchanged, keeps existing ItemNext/ItemPrev behavior).

### Keymap changes (`pkg/tui/keymap.go`)

- Update help text for Tab/Shift+Tab from "next tab"/"prev tab" to
  "next item"/"prev item".
- Update help text for Up/Down in the incident view context to reflect
  section selection.
- Remove TabNext/TabPrev from the help column since their role changes
  to item cycling (same as ItemNext/ItemPrev but within the active
  section).

### clearSelectedIncident changes

- Reset `activeSection` to 0 (alerts) instead of `activeTab`.

## Files

- `pkg/tui/model.go` -- replace `activeTab` with `activeSection`
- `pkg/tui/views.go` -- new layout, new templates, new section headers
- `pkg/tui/msgHandlers.go` -- Tab/arrow behavior changes
- `pkg/tui/keymap.go` -- help text updates
- `pkg/tui/model_test.go` -- update tests for new section model
- `pkg/tui/views_test.go` -- update template tests
- `pkg/tui/msgHandlers_test.go` -- update key handler tests
- `pkg/tui/keymap_test.go` -- no structural changes needed

## Verification

- `make test` passes with updated tests
- Details always visible when viewing an incident
- Tab cycles through alerts or notes depending on active section
- Up/Down switches which section Tab operates on
- Section headers show active marker and item counts
- Number keys jump to correct item in active section
