# Plan 062: TUI Test Coverage

## Issue

#205 - Add comprehensive test coverage for pkg/tui/ functions at 0% coverage.

## Problem

Several functions in pkg/tui/ have zero test coverage. These functions handle
critical behavior: adding notes to incidents, opening editors, rendering incident
markdown, login command construction, model initialization, help toggling, error
handling, chord commands, and Claude Code detection. Without tests, regressions
in these functions go undetected.

## Approach

Write tests that verify **existing behavior** without changing production code.
Follow existing test patterns: table-driven tests with testify/assert, mock PD
client convention-based errors (ID="err"), and tea.Cmd execution with message
type assertions.

### Functions to test

#### commands.go
- `addNoteToIncident()` - reads temp file, strips comments, posts note via mock client
- `openEditorCmd()` - editor argument assembly (not actual file creation)
- `renderIncident()` - template + markdown rendering via tea.Cmd

#### commands.go (login helpers)
- `buildPagerDutyEnvVars()` - already tested, skip
- `commandContainsOCMContainer()` - already tested, skip
- `insertOCMContainerEnvFlags()` - already tested, skip
- `extractEnvVarPairs()` - already tested, skip

#### model.go
- `InitialModelWithConfig()` - model field initialization from pre-built config
- `toggleHelp()` - help visibility toggle
- `defaultLogFilePath()` - already tested, skip

#### msgHandlers.go
- `setStatusMsgHandler()` - status message setting
- `errMsgHandler()` - error handling and status setting

#### views.go
- `renderIncidentMarkdown()` - nil renderer fallback, styled output

#### chords.go
- `chordViewLog()` - returns a tea.Cmd for reading log file

#### claude.go
- `defaultHasClaudeCode()` - exec.LookPath wrapper, no-panic verification

## Test file organization

All new tests go into the existing `*_test.go` files alongside their source,
following the project convention.

## Post-mortem / Lessons Learned

(To be filled after PR merge)
