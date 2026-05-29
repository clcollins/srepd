# 055: Tabbed Navigation for Incident Viewer

**Issue**: #199
**Branch**: srepd/tabbed-navigation
**Status**: In Progress

## Problem

Incidents with many alerts (10+) display all alerts at once in a long
scrollable view. Same with notes. This makes it hard to scan individual
alerts or notes.

## Solution

Add tabbed navigation to the incident viewer. Three top-level tabs:
`[Details] [Alerts (N)] [Notes (N)]`, with left/right arrows to switch
between tabs. Within Alerts/Notes tabs, left/right or number keys (1-9)
to page through individual items.

## Design

### Model changes (model.go)

Add three new fields to the `model` struct:

- `activeTab int` -- 0=details, 1=alerts, 2=notes
- `activeAlertIdx int` -- 0-based index of currently shown alert
- `activeNoteIdx int` -- 0-based index of currently shown note

Reset all three to 0 in `clearSelectedIncident()`.

### Template changes (views.go)

Split the current `incidentTemplate` into three separate templates:

- `detailsTabTemplate` -- incident metadata (ID, title, service,
  urgency, created, status, assignments, acknowledgements)
- `alertTabTemplate` -- single alert rendering (name, cluster, SOP,
  service, created, status, severity, type)
- `noteTabTemplate` -- single note rendering (content, user, created)

Add a `renderTabHeader()` function that renders the tab bar:
`[Details] [Alerts (5)] [Notes (3)]` with the active tab highlighted.

Modify the `template()` method to select which template to render based
on `activeTab`, and pass the appropriate data slice index.

### Key binding changes (keymap.go)

Add new key bindings to the keymap struct:

- `TabNext` -- Tab or right arrow to switch to next tab
- `TabPrev` -- shift+tab or left arrow to switch to previous tab

These are only active in incident viewer mode (switchIncidentFocusMode).

### Message handler changes (msgHandlers.go)

In `switchIncidentFocusMode()`:

- Left/right arrows switch tabs at the top level
- Within Alerts/Notes tabs, left/right also pages through items
  (Tab/Shift+Tab switch tabs exclusively)
- Number keys 1-9 jump to specific alert/note index
- Bounds checking: clamp index to valid range

### Data flow

- `selectedIncidentAlerts[activeAlertIdx]` feeds the alert template
- `selectedIncidentNotes[activeNoteIdx]` feeds the note template
- When alerts/notes are not loaded yet, show loading indicator

## Tests (TDD -- write first)

1. `TestTabSwitch_LeftRight` -- Tab key cycles through tabs 0->1->2->0
2. `TestAlertTab_ShowsSingleAlert` -- renders exactly one alert
3. `TestAlertTab_Navigation` -- left/right changes activeAlertIdx
4. `TestNoteTab_ShowsSingleNote` -- renders exactly one note
5. `TestTabBoundsCheck` -- index clamped to valid range
6. `TestTabReset_OnClearSelectedIncident` -- tab state resets
7. `TestDetailsTab_Template` -- details tab renders metadata only
8. `TestTabHeader_Rendering` -- tab bar shows counts and highlights
9. `TestNumberKeys_JumpToIndex` -- digit keys select specific item
10. `TestViewport_ScrollReset` -- viewport resets to top on tab switch

## Files Modified

- `pkg/tui/model.go` -- add tab state fields, reset in clear
- `pkg/tui/views.go` -- split template, add tab header, tab templates
- `pkg/tui/msgHandlers.go` -- tab navigation in incident focus mode
- `pkg/tui/keymap.go` -- add TabNext/TabPrev key bindings
- `pkg/tui/model_test.go` -- tab state tests
- `pkg/tui/views_test.go` -- template rendering tests
- `pkg/tui/msgHandlers_test.go` -- navigation tests

## Risks

- Changing the incident template is a visual change; need to ensure
  the details tab renders identically to the current full view minus
  alerts and notes sections.
- Left/right arrows are used by the viewport for horizontal scrolling;
  need to intercept them before the viewport consumes them.
