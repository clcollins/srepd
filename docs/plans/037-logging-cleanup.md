# Plan 037: Logging Cleanup

## Problem

The codebase had several logging inconsistencies that hurt debuggability:

1. A typo: `defaultIncidentStatues` instead of `defaultIncidentStatuses` in `pkg/pd/pd.go`
2. Inconsistent error key names (`"err"`, `"tea.errMsg"`) instead of the standard `"error"`
3. `fmt.Sprintf` used inside `log.Debug()` calls instead of native key-value pairs
4. `log.Printf` used instead of structured `log.Info` with key-value pairs
5. `log.Warn` used for actual errors in `login()` (pipe failures, process start failures)
6. Inconsistent log prefix formats (missing package name, missing parens)
7. String concatenation in log messages instead of key-value pairs

## Changes

### pkg/pd/pd.go
- Renamed `defaultIncidentStatues` to `defaultIncidentStatuses` (typo fix)

### pkg/tui/commands.go
- Fixed all `fmt.Sprintf` inside `log.Debug()` calls to use native key-value format
- Changed `log.Printf` to `log.Info` with key-value pairs in `silenceIncidents()`
- Changed `log.Warn` to `log.Error` for actual errors in `login()` (pipe creation, process start, I/O read failures, process exit errors)
- Standardized prefix format from `"commands.ShouldBeAcknowledged"` to `"tui.ShouldBeAcknowledged()"`
- Standardized prefix format from `"commands.UserIsOnCall"` to `"tui.UserIsOnCall()"`
- Fixed misformatted key-value pairs (error messages used as keys)
- Standardized `silenceIncidents` log prefixes to `"tui.silenceIncidents()"`

### pkg/tui/msgHandlers.go
- Changed `"tea.errMsg"` key to `"error"` in `errMsgHandler`
- Updated prefix to `"tui.errMsgHandler()"`

### pkg/tui/model.go
- Fixed misformatted key-value pair in `InitialModel` markdown renderer error log

### pkg/launcher/launcher.go
- Standardized prefix from `"launcher.ClusterLauncher()"` to `"launcher.BuildLoginCommand()"`
- Fixed `fmt.Sprintf` used as key format in argument logging
- Fixed `"ClusterLauncher():"` prefix in `replaceVars` to `"launcher.replaceVars()"`
- Standardized key names in replaceVars logging

### cmd/config.go
- Replaced string concatenation in `log.Warn` with key-value pairs

## Validation

- `make fmt-check` -- passes (no formatting issues)
- `make vet` -- passes (no vet warnings)
- `make test` -- all tests pass (no regressions)
