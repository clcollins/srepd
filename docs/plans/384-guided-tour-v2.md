# 384: In-TUI guided tour (:tour)

Branch: `srepd/guided-tour-v2`

## What

Nine-step in-app walkthrough: incident table, navigation, viewer tabs, key
actions, command mode, chords, flags, watcher. Rendered as a step panel
beneath the live table. Keys described, never executed — safe with live data.

Entry: `:tour` command + one-time post-setup status suggestion. `tour_seen`
persisted so the suggestion shows once.

## Files

- `pkg/tui/tour.go` — steps, rendering, controls, tour_seen persistence
- `pkg/tui/tour_test.go` — content coverage, command parsing, navigation, render
- `pkg/tui/model.go` — tourMode, tourStep fields
- `pkg/tui/msgHandlers.go` — tour dispatch + :tour command
- `pkg/tui/views.go` — tour rendering case
- `pkg/tui/tui.go` — post-setup suggestion
