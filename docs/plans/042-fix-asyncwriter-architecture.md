# Plan 042: Fix asyncWriter Architecture

## Status: Completed

## Problem

The asyncWriter setup had a layering bug where two competing log
configurations fought for control:

1. `main.go` (lines 40-52) opened `~/.config/srepd/debug.log`, wrapped it
   in `asyncWriter`, and set it as `log.SetOutput()`.
2. `cmd/root.go`'s `configureLogging()` (called during `cobra.OnInitialize`)
   immediately replaced `log.SetOutput()` with either `journalWriter`, a
   different file handle, or `os.Stderr`.
3. The asyncWriter from `main.go` was silently abandoned -- its buffering
   was completely bypassed and never received any log messages after init.
4. The file handle opened in `main.go` was leaked (the defer ran at exit
   but the handle was not actually used for logging).

## Solution

Make `cmd/root.go`'s `configureLogging()` the single source of truth for
log configuration, and wrap every log destination in `asyncWriter` there.

### Changes

1. **Moved `asyncWriter` type from `main.go` to `cmd/asyncwriter.go`**:
   The type and its constructor/methods were moved to the `cmd` package so
   `configureLogging()` can use them directly. This is the package that
   owns log configuration.

2. **Moved tests from `main_test.go` to `cmd/asyncwriter_test.go`**:
   All existing asyncWriter tests were preserved verbatim, just relocated
   to the `cmd` package alongside the type they test.

3. **Simplified `main.go`**: Removed all log file setup, asyncWriter
   creation, and `log.SetOutput()` calls. `main()` now only calls
   `cmd.Execute()`.

4. **Updated `configureLogging()` in `cmd/root.go`**: Each log destination
   (journal, file, stderr) is now wrapped in `asyncWriter` before being
   passed to `log.SetOutput()`. A package-level `logWriter` variable holds
   the active writer.

5. **Added `CleanupLogging()`**: Called via `defer` in `Execute()` to
   flush and close the asyncWriter on application exit. Safe to call
   multiple times.

## Out of Scope

- Journal priority mapping (all messages sent as `PriInfo`) is a known
  issue tracked separately.
- Log rotation is not addressed here (existing TODO remains).

## Testing

- All existing asyncWriter tests pass in their new location.
- `make fmt-check`, `make vet`, and `make test` all pass.
