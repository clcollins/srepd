# 059: Replace Action Log with Flash Notifications

## Problem

The action log (ctrl+l toggle) takes screen real estate from the incident table for a 5-row fixed panel showing recent actions. It's not searchable, not scrollable, and easy to miss.

## Solution

Two changes:
1. Replace the action log panel with auto-dismissing status bar flash notifications
2. Reassign ctrl+l from action log toggle to log viewer (more intuitive for "log")

## Implementation

### Flash notifications
- Remove `actionLogTable`, `showActionLog`, `actionLog` fields from model
- Remove action log rendering from View()
- Remove `newActionLogTable()`, `addActionLogEntry()`, `updateActionLogTable()`, `ageOutResolvedIncidents()`
- Add `flashNotification()` method that sets status and returns a tea.Tick for auto-clear
- Add `clearFlashMsg` type with message field for selective clearing
- Handle `clearFlashMsg` in Update() - only clear if status still matches

### Log viewer key change
- Remove `ToggleActionLog` from keymap struct and defaultKeyMap
- Change `ViewLog` from `ctrl+d` to `ctrl+l`
- Update help text and README

### Callers updated
- All `addActionLogEntry()` calls become `flashNotification()` calls
- Remove toggle handler from `keyMsgHandler`
- Remove action log resize logic from `windowSizeMsgHandler`

## Files Changed
- `pkg/tui/model.go`
- `pkg/tui/views.go`
- `pkg/tui/keymap.go`
- `pkg/tui/msgHandlers.go`
- `pkg/tui/tui.go`
- `pkg/tui/commands.go`
- `pkg/tui/model_test.go`
- `pkg/tui/views_test.go`
- `pkg/tui/keymap_test.go`
- `pkg/tui/msgHandlers_test.go`
- `README.md`

## Related
- Issue #220
- Issue #203 (log viewer)
- PR #214 (log viewer implementation)
