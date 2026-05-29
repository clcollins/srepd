# Plan 044: Toolbox Environment Variable Passing

## Problem

When running inside a Fedora Toolbox container, `exec.Cmd.Env` does not
propagate environment variables to the host process launched via
`flatpak-spawn --host`. This means PAGERDUTY_* env vars were lost for
non-ocm-container flows (e.g., `ocm backplane login`).

Additionally, the original implementation (#185) added BOTH `--env=` flags
on flatpak-spawn AND `-e` flags on ocm-container, which was redundant.

## Solution

Implement three-way flow detection in `login()` based on the command being
executed:

1. **ocm-container flow**: Use `-e` flags only. These are ocm-container CLI
   arguments that get passed through to podman. Detected by checking if any
   element of the command contains "ocm-container".

2. **Non-ocm-container flow in toolbox**: Use `--env=` flags on
   flatpak-spawn only. The `--env=` flags cause flatpak-spawn to set
   environment variables on the host process it spawns. Detected by
   `launcher.IsToolbox()` returning true and no ocm-container in the command.

3. **Non-ocm-container flow NOT in toolbox**: Use `exec.Cmd.Env` to set
   environment variables on the spawned process directly.

## Changes

### pkg/launcher/launcher.go
- Added `IsToolbox()` method to expose toolbox detection state
- Added `ToolboxEnvFlags()` method to convert `-e KEY=VALUE` pairs to
  `--env=KEY=VALUE` flags for flatpak-spawn
- Added `InsertToolboxEnvFlags()` function to insert `--env=` flags at the
  correct position in the command (after `--host` but before terminal command)

### pkg/tui/commands.go
- Added `commandContainsOCMContainer()` helper to detect ocm-container flow
- Added `insertOCMContainerEnvFlags()` to insert `-e` flags after the
  ocm-container argument in the command
- Added `extractEnvVarPairs()` to extract `KEY=VALUE` strings from
  `["-e", "KEY=VALUE", ...]` pairs for `exec.Cmd.Env`
- Modified `login()` to use three-way flow detection instead of always
  inserting `-e` flags

### Tests

#### pkg/tui/commands_test.go
- `TestCommandContainsOCMContainer_True` - ocm-container detected in various
  command structures
- `TestCommandContainsOCMContainer_False` - non-ocm-container commands
- `TestExtractEnvVarPairs` - correct extraction of KEY=VALUE pairs
- `TestInsertOCMContainerEnvFlags` - correct insertion position

#### pkg/launcher/launcher_test.go
- `TestIsToolbox` - IsToolbox() returns correct value
- `TestToolboxEnvFlags_AddsEnvFlags` - correct conversion to --env= format
- `TestToolboxEnvFlags_NoFlagsWhenNotToolbox` - returns nil when not in toolbox
- `TestToolboxEnvFlags_EmptyEnvFlags` - handles empty input
- `TestToolboxEnvFlags_NilEnvFlags` - handles nil input
- `TestToolboxEnvFlags_OddLengthEnvFlags` - graceful handling of malformed input
- `TestInsertToolboxEnvFlags_PositionAfterFlatpakSpawn` - correct insertion
- `TestInsertToolboxEnvFlags_EmptyFlags` - no-op with empty flags
- `TestInsertToolboxEnvFlags_NoFlatpakSpawn` - no-op without flatpak-spawn

## Post-mortem / Lessons Learned

(To be filled after PR review and merge)
