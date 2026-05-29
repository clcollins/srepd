# 029: Login Command Decomposition

## Status: Complete

## Summary

Extract testable pure functions from the `login()` command in `pkg/tui/commands.go`.
The original function was 193 lines long with three distinct responsibilities:

1. Build environment flags (pure data transformation)
2. Insert env flags into command at correct position (pure data transformation)
3. Execute the command (I/O)

Steps 1 and 2 were tightly coupled to the I/O step, making them untestable without
spawning processes. This plan extracts them as standalone pure functions.

## Changes

### Extracted Functions

**`buildAlertData(incident, alerts, notes) ([]byte, error)`**
- Serializes PagerDuty incident, alerts, and notes into a base64-encoded JSON string
- Returns `nil, nil` when there is no data (nil incident and empty slices)
- Uses `base64.RawStdEncoding` (no padding) to avoid `=` splitting issues in ocm-container

**`insertEnvFlagsIntoCommand(command []string, envFlags []string) []string`**
- Finds the `--` separator or first element boundary in a command slice
- Inserts env flags after the target command but before its arguments
- Returns the command unchanged when envFlags is empty
- Handles edge cases: separator at end, no separator, terminal flags before separator

### Refactored `login()`
- Calls `buildAlertData()` instead of inline JSON+base64 encoding
- Calls `insertEnvFlagsIntoCommand()` instead of inline separator-finding logic
- Renamed shadowed `err` variable to `stderrErr` in I/O section for clarity
- No behavioral change to the I/O section

## Tests Added

| Test | Purpose |
|------|---------|
| `TestBuildAlertData_NilIncident` | nil incident + nil slices returns nil |
| `TestBuildAlertData_NilIncidentEmptySlices` | nil incident + empty slices returns nil |
| `TestBuildAlertData_FullData` | full data produces decodable base64 JSON |
| `TestBuildAlertData_RoundTrip` | encode then decode matches original data |
| `TestBuildAlertData_IncidentOnlyNoAlertsOrNotes` | incident without alerts/notes works |
| `TestInsertEnvFlags_WithSeparator` | flags after `--` and target command |
| `TestInsertEnvFlags_WithoutSeparator` | flags after first element |
| `TestInsertEnvFlags_EmptyFlags` | command unchanged |
| `TestInsertEnvFlags_NoArgs` | single command + flags |
| `TestInsertEnvFlags_TerminalFlagsBeforeSeparator` | terminal args preserved |
| `TestInsertEnvFlags_SeparatorAtEnd` | separator at end appends flags |

## Files Modified

- `pkg/tui/commands.go` - extracted functions, refactored `login()`
- `pkg/tui/commands_test.go` - 11 new tests

## Validation

- `make fmt-check` - pass
- `go vet ./...` - pass
- `golangci-lint run ./...` - pass (0 issues)
- `go test ./... -count=1` - all packages pass
