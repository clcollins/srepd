# Add test coverage for cmd/ functions at 0% coverage

## Context

Issue #205: Several functions in `cmd/root.go` have 0% test coverage.
The existing test file covers `determineLogDestination()`, `journalPriority()`,
`StringValue()`, and `BoolValue()`, but `configureLogging()`, `setupFileLogging()`,
`bindArgsToViper()`, `CleanupLogging()`, and `journalWriter.Write()` are untested.

## Plan

1. **setupFileLogging()** -- Test that it creates/opens a log file and sets the
   global `logWriter`. Use `t.TempDir()` for file paths. Test error case by
   passing an invalid path (the function calls `log.Fatal` on error, so the
   error path is not directly testable without process-level tricks; test the
   success path only).

2. **CleanupLogging()** -- Test that calling `CleanupLogging()` when `logWriter`
   is non-nil calls `Close()` on the async writer. Test the nil case to verify
   no panic.

3. **bindArgsToViper()** -- Create a `cobra.Command` with the expected flags
   (debug, dev, fixtures-dir), set flag values, call `bindArgsToViper()`, and
   verify that `viper.Get()` returns the correct values.

4. **configureLogging()** -- This function uses `runtime.GOOS` and
   `journal.Enabled()` at call time, making it harder to test in isolation.
   Test the file-logging path by setting `viper.Set("log_to_journal", false)`
   on Linux (which routes to file logging at `~/.config/srepd/debug.log`).
   Use a temporary directory to avoid writing to the real config path. Since
   the function expands `~/` internally, we instead test via `setupFileLogging()`
   directly for file paths, and verify the stderr path by checking `logWriter`
   is set after calling `configureLogging()` with an unsupported GOOS scenario.
   In practice, `configureLogging()` is best tested indirectly through the
   functions it calls; we add a smoke test that verifies it sets `logWriter`.

5. **journalWriter.Write()** -- This calls `journal.Send()` which requires
   systemd. Skip if `!journal.Enabled()`. When available, verify the write
   returns the correct byte count and no error.

## Files

- `cmd/root_test.go` -- add tests for all five functions above

## Verification

- `make test-all` passes (fmt-check, vet, lint, test)
- `make test-race` passes
- `make plan-check` passes
- `make readme-check` passes
- Coverage for targeted functions increases from 0%
