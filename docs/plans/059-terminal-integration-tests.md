# 059 - Terminal Integration Tests

## Context

Issue #191: SREPD supports 14+ terminal emulators via terminal
profiles, but only a few can be tested manually. We need automated
integration tests that verify terminal binaries exist and can be
called, and that profile detection matches every entry in the
profile maps.

Prior work in plans 040 (multi-terminal-profiles) and 047
(flatpak-ghostty-validation) established the profile detection
system and validation logic. This plan adds integration tests that
exercise those systems against real binaries on the host.

## Plan

Add a new test file `pkg/launcher/integration_test.go` containing:

1. **Binary existence tests**: For each supported terminal, verify
   `exec.LookPath` finds it and the path points to a real file.
   Use `t.Skip` when the terminal is not installed so the test
   passes in CI and on machines without that terminal.

2. **Binary callable tests**: For each installed terminal, invoke
   it with a safe flag (`--help`, `--version`, `-V`) and verify it
   produces output. This catches broken installs (missing shared
   libraries, corrupt binaries).

3. **Profile detection exhaustive tests**: Iterate every entry in
   `separatorTerminals`, `flagTerminals`, `directTerminals`,
   `appleScriptTerminals`, and `flatpakAppTerminals` and verify
   `DetectTerminalProfile` returns the correct profile type with
   the correct internal fields (terminal name, flag, app name).

4. **Validation consistency tests**: Call `validateTerminalExists`
   for each terminal and verify the result is consistent with
   `exec.LookPath` (warning when missing, no warning when present).

5. **Completeness guard tests**: Verify that the test-level
   `supportedTerminals` list covers every entry in the profile
   maps, catching drift when new terminals are added to
   `profiles.go`.

## Files Modified

| File | Change |
|------|--------|
| `pkg/launcher/integration_test.go` | New file with all integration tests |
| `docs/plans/059-terminal-integration-tests.md` | This plan |

## Verification

- `make test` passes (integration tests skip gracefully for
  missing terminals)
- `make test-race` passes (no race conditions)
- `make fmt-check` passes (properly formatted)
- `make vet` passes (no vet issues)
- `make lint` passes (no lint issues)
- `make plan-check` passes (this plan document exists)
- `make readme-check` passes (no config/key/flag changes)
