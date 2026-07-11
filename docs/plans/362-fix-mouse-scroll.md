# 362: Fix mouse wheel scrolling on main table and watcher views

## Problem

Mouse wheel scrolling stopped working on the main incident table and watcher
pane. The `tea.MouseMsg` handler in `Update()` only forwarded scroll events to
the watcher viewport when expanded, and dropped them entirely when collapsed.
The incident table never received scroll events. Meanwhile, incident view and
log view scrolling worked because their viewports received events through a
different code path.

## Approach

Extracted mouse event routing into `pkg/tui/mouse.go` with position-aware
dispatch based on the mouse Y coordinate and the current view mode.

### Routing logic

1. **Non-wheel events**: no-op (return nil)
2. **Incident view / log view**: forward unconditionally to the active
   viewport — no Y computation needed, and any future views added to this
   switch get mouse scroll for free
3. **Config / bulk silence / team select / cluster select / merge modes**:
   no-op (forms don't scroll via mouse)
4. **Main table view**: compute the Y boundary between the table and watcher
   using `mouseWatcherStartY()`, which derives the threshold from the current
   layout (adapts on window resize). Route to watcher viewport or translate
   wheel events to `MoveUp`/`MoveDown` on the table with incident sync.

### Key design decisions

- `mouseWatcherStartY()` is computed from layout constants and style overhead
  so it automatically adjusts after `tea.WindowSizeMsg` triggers
  `recomputeLayout()`
- Table scrolling translates wheel events to `MoveUp`/`MoveDown` + calls
  `syncSelectedIncidentToHighlightedRow()` for pre-fetch consistency
- `tea.MouseMsg` was already in the logging skip list — no change needed

## Testing

12 new tests in `mouse_scroll_test.go` covering: table scroll up/down,
watcher scroll, table scroll with watcher expanded, incident/log view
forwarding, non-wheel no-op, resize adaptation, selected incident sync,
and boundary calculation.
