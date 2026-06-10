# Toolbox container detection

> Fixes #174.

## Context

When srepd runs inside a Fedora Toolbox container, terminal launch
commands (like `gnome-terminal --`) fail because the terminal emulator
binary does not exist inside the container. The fix is to detect the
toolbox environment and automatically prefix commands with
`flatpak-spawn --host` so they execute on the host system.

## Plan

1. Create `pkg/container/` package with `IsRunningInToolbox()` function
   that checks three indicators in priority order:
   - `/run/.toolboxenv` file exists
   - `TOOLBOX_PATH` environment variable is set
   - `container` environment variable equals `"toolbox"`

2. Make detection testable by using an internal `checkToolbox()` function
   that accepts the file path and an env-getter function as parameters,
   allowing tests to inject mocks without touching real environment state.

3. Modify `pkg/launcher/launcher.go`:
   - Add `runInToolbox` field to `ClusterLauncher`
   - Add `toolboxMode` parameter to `NewClusterLauncher()` (values:
     "auto", "true", "false")
   - Add `NewClusterLauncherWithToolbox()` for testing with injectable
     detection function
   - In `BuildLoginCommand()`, prepend `flatpak-spawn --host` when
     `runInToolbox` is true

4. Modify `cmd/config.go`:
   - Add `toolbox_mode` optional config key with default `"auto"`
   - Add to example config documentation

5. Update `cmd/root.go` call site to pass the new `toolboxMode`
   parameter

## Lessons Learned

From plan 029 (login-decomposition): keeping launcher logic testable
with injectable dependencies avoids environment-dependent test flakiness.
The `checkToolbox()` / `NewClusterLauncherWithToolbox()` pattern follows
this same principle.

**GENUINE ERROR — env vars silently lost across flatpak-spawn boundary**
(Fixed by: [044-toolbox-env-var-passing.md](044-toolbox-env-var-passing.md))

The initial implementation added BOTH `--env=` flags on `flatpak-spawn`
AND `-e` flags on `ocm-container` redundantly, and did not handle the
fact that `exec.Cmd.Env` does not propagate environment variables to
the host process launched via `flatpak-spawn --host`. Environment
variables silently disappeared.

Why it wasn't caught: tests did not run inside a Toolbox container, so
the `flatpak-spawn` env var boundary was never exercised. The unit
tests verified command construction but not runtime env propagation.

Prevention: when implementing container-aware features, document and
test the env var propagation path for each execution flow. The fix
required three-way flow detection (ocm-container, non-ocm in toolbox,
non-ocm outside toolbox), each with different env var mechanisms.

## Files Modified

- `pkg/container/container.go` -- new package: `IsRunningInToolbox()`,
  `checkToolbox()`
- `pkg/container/container_test.go` -- 6 tests for detection logic
- `pkg/launcher/launcher.go` -- `runInToolbox` field,
  `NewClusterLauncherWithToolbox()`, `resolveToolboxMode()`,
  `flatpak-spawn --host` prefix in `BuildLoginCommand()`
- `pkg/launcher/launcher_test.go` -- 10 new tests for toolbox wrapping
  and mode resolution
- `cmd/config.go` -- `toolbox_mode` optional key with default `"auto"`
- `cmd/config_test.go` -- verify `toolbox_mode` defaults
- `cmd/root.go` -- pass `toolbox_mode` to `NewClusterLauncher()`

## Verification

Container detection tests:
- `TestIsRunningInToolbox_FileExists` -- temp file simulates
  `/run/.toolboxenv`
- `TestIsRunningInToolbox_ToolboxPathEnvVar` -- TOOLBOX_PATH env var
- `TestIsRunningInToolbox_ContainerEnvVar` -- container=toolbox
- `TestIsRunningInToolbox_ContainerEnvVarWrongValue` -- container=podman
- `TestIsRunningInToolbox_NotInToolbox` -- no indicators present
- `TestIsRunningInToolbox_PriorityOrder` -- file check short-circuits

Launcher toolbox tests:
- `TestNewClusterLauncher_ToolboxDetection` -- 6 subtests for mode
  resolution (auto/true/false/empty)
- `TestBuildLoginCommand_ToolboxWrapping` -- flatpak-spawn prepended
- `TestBuildLoginCommand_NoWrappingNormal` -- no wrapping without
  toolbox
- `TestBuildLoginCommand_ToolboxWrappingWithTmux` -- works with tmux

Config test:
- `TestValidateConfig_OptionalKeysGetDefaults` -- toolbox_mode defaults
  to "auto"

CI checks:
- `make fmt-check` -- passes
- `make vet` -- passes
- `make test` -- all tests pass
