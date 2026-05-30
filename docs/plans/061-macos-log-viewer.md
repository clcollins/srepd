# 061: macOS Log Viewer Support

**Issue**: #196
**Branch**: srepd/macos-log-viewer
**Status**: In Progress

## Problem

The `defaultLogFilePath()` function in `pkg/tui/model.go` hardcodes the Linux log path (`~/.config/srepd/debug.log`). On macOS, the conventional log location is `~/Library/Logs/srepd.log`. The log viewer pane (issue #203) needs to read from the correct path based on the host OS.

## Solution

Make `defaultLogFilePath()` OS-aware using `runtime.GOOS` to select the correct path, matching the platform detection already used in `cmd/root.go`'s `determineLogDestination()`.

- Linux: `~/.config/srepd/debug.log`
- macOS (darwin): `~/Library/Logs/srepd.log`
- Other: returns empty string (no log file available)

Also fix `InitialModelWithConfig()` which is missing the `logViewer` and `logFilePath` initialization, ensuring dev mode has the same log viewer support.

## Implementation Plan

### defaultLogFilePath (model.go)

- Import `runtime` and `path/filepath`
- Use `runtime.GOOS` to select the correct log file path
- Use `filepath.Join` for path construction (cross-platform safe)
- Return empty string for unsupported platforms

### InitialModelWithConfig (model.go)

- Add missing `logViewer: newLogViewer()` initialization
- Add missing `logFilePath: defaultLogFilePath()` initialization

## Test Plan

1. `TestDefaultLogFilePath_Linux` - returns `~/.config/srepd/debug.log` equivalent on linux
2. `TestDefaultLogFilePath_Darwin` - returns `~/Library/Logs/srepd.log` equivalent on darwin
3. `TestDefaultLogFilePath_Unsupported` - returns empty string for unknown OS
4. `TestLogFilePathForOS` - helper function tests with explicit GOOS parameter

Since `runtime.GOOS` is a constant and cannot be changed at test time, extract the OS-dependent logic into a `logFilePathForOS(goos string)` function that can be tested with arbitrary OS values. The `defaultLogFilePath()` function calls it with `runtime.GOOS`.

## Files Modified

- `pkg/tui/model.go` - OS-aware `defaultLogFilePath()`, new `logFilePathForOS()`, fix `InitialModelWithConfig`
- `pkg/tui/model_test.go` - tests for `logFilePathForOS()` and `defaultLogFilePath()`
