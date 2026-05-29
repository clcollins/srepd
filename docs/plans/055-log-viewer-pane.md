# 055: Scrollable Log Viewer Pane

**Issue**: #203
**Branch**: srepd/log-viewer-pane-v2
**Status**: In Progress

## Problem

No way to view the debug log from within srepd. Users must open a separate terminal to tail the log file.

## Solution

Add a scrollable log viewer pane using the same viewport pattern as the existing `incidentViewer`. The log viewer is opened with `ctrl+d`, reads `~/.config/srepd/debug.log`, shows the latest entries (scrolls to bottom), and is dismissed with Escape.

## Implementation Plan

### Model Changes (model.go)

- Add `viewingLog bool` field
- Add `logViewer viewport.Model` field
- Add `logFilePath string` field (set from platform config dir)
- Add `newLogViewer()` constructor matching `newIncidentViewer()` pattern
- Set `logFilePath` in `InitialModel()`

### Key Binding (keymap.go)

- Add `ViewLog` key binding for `ctrl+d`
- Add to help display in appropriate column

### Message Types (commands.go)

- Add `logFileContentMsg string` message type
- Add `readLogFile(path string) tea.Cmd` command function

### Key Handler (msgHandlers.go)

- In `keyMsgHandler`: before focus mode switch, check `m.viewingLog` and route to `switchLogFocusMode`
- In `switchTableFocusMode`: handle `ctrl+d` to trigger `readLogFile`
- Add `switchLogFocusMode()`: up/down/pgup/pgdn scroll via viewport, Escape dismisses

### Update Handler (tui.go)

- Handle `logFileContentMsg`: set viewport content, GotoBottom(), set `viewingLog = true`

### View Rendering (views.go)

- In `View()`: when `viewingLog`, render `logViewer` viewport (same as `viewingIncident` layout)

### Window Resize (msgHandlers.go)

- In `windowSizeMsgHandler`: set `logViewer` width/height (same calculation as `incidentViewer`)

### Edge Cases

- File does not exist: show "No log file found at <path>" in viewport
- Large file: load entire file (viewport handles scrolling)
- Show latest entries first: `GotoBottom()` after `SetContent`

## Test Plan

1. `TestViewLogKey_OpensLogViewer` - ctrl+d key sets viewingLog=true via readLogFile command
2. `TestViewLogKey_EscapeCloses` - Escape in log view returns viewingLog=false
3. `TestLogFileContentMsg_SetsViewport` - logFileContentMsg loads content into viewport
4. `TestLogFileContentMsg_FileNotFound` - missing file shows error message in viewport
5. `TestLogViewer_WindowResize` - logViewer dimensions set on WindowSizeMsg
6. `TestViewLogKey_InKeymap` - ctrl+d binding exists in keymap

## Files Modified

- `pkg/tui/model.go` - viewingLog, logViewer, logFilePath fields
- `pkg/tui/keymap.go` - ViewLog key binding
- `pkg/tui/commands.go` - logFileContentMsg, readLogFile
- `pkg/tui/msgHandlers.go` - log focus mode handler, key binding, window resize
- `pkg/tui/views.go` - render log viewer in View()
- `pkg/tui/tui.go` - handle logFileContentMsg
