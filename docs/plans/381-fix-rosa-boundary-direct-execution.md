# Plan 381: Fix rosa-boundary Direct Execution

## Problem

SREPD's rosa-boundary launcher is incorrectly initialized with a terminal emulator wrapper, causing it to open a new terminal window unnecessarily. This is a bug because rosa-boundary is a standalone CLI tool that manages its own interactive session via `session-manager-plugin`, unlike `ocm-container` or `ocm backplane login` which need to run in a new terminal.

### Current Behavior

When a user initiates a rosa-boundary login:
1. SREPD builds the command: `gnome-terminal -- rosa-boundary start-task --cluster-id <id> --connect`
2. A new terminal window opens
3. rosa-boundary runs inside that window, managing its own session

### Expected Behavior

When a user initiates a rosa-boundary login:
1. SREPD builds the command: `rosa-boundary start-task --cluster-id <id> --connect`
2. rosa-boundary executes directly in the current context
3. rosa-boundary manages its own interactive session connection

## Root Cause

**Location**: `cmd/root.go:279-286`

The rosa-boundary launcher is initialized using the same terminal emulator configuration as the regular cluster login launcher:

```go
rbl, err = launcher.NewClusterLauncher(viper.GetString("terminal"), rbCmd, viper.GetString("toolbox_mode"))
```

This causes `rosaBoundaryLogin()` in `pkg/tui/commands.go` to call `l.BuildLoginCommand(vars)`, which wraps the rosa-boundary command with the terminal emulator.

## Solution

Add a new method `BuildRosaBoundaryCommand()` to the `ClusterLauncher` type that builds the rosa-boundary command directly without terminal wrapping.

### Changes

1. **New Method**: `pkg/launcher/launcher.go`
   - Add `BuildRosaBoundaryCommand(vars map[string]string) []string` method
   - This method performs variable substitution on `clusterLoginCommand` only
   - Does NOT add terminal wrapper (unlike `BuildLoginCommand`)
   - Does NOT add toolbox wrapper (rosa-boundary should run directly)

2. **Updated Function**: `pkg/tui/commands.go`
   - Modify `rosaBoundaryLogin()` to call `l.BuildRosaBoundaryCommand(vars)` instead of `l.BuildLoginCommand(vars)`
   - Update comment to explain why rosa-boundary uses direct execution

3. **New Tests**: `pkg/launcher/launcher_test.go`
   - Add `TestBuildRosaBoundaryCommand_DirectExecution` with subtests:
     - Basic rosa-boundary command without terminal wrapper
     - Verify toolbox mode doesn't add flatpak-spawn
     - Multiple variable replacements work correctly
     - Comparison test showing BuildLoginCommand adds wrapper but BuildRosaBoundaryCommand doesn't

## Design Decisions

### Why Not Modify BuildLoginCommand?

We could have added a flag to `BuildLoginCommand` to skip terminal wrapping, but that would:
- Add complexity to an already complex method
- Mix concerns (terminal-wrapped vs direct execution)
- Make the API less clear

A separate method makes the distinction explicit: `BuildLoginCommand` is for commands that need a terminal wrapper, `BuildRosaBoundaryCommand` is for rosa-boundary's direct execution.

### Why Not Make Terminal Optional During Initialization?

The `ClusterLauncher` is initialized once with a terminal emulator for the regular login workflow. We don't want to make the terminal optional because:
- The regular cluster login (ocm-container, backplane) genuinely needs the terminal
- Making terminal optional would weaken the validation that ensures the launcher is properly configured
- The rosa-boundary launcher instance is never actually used for regular logins, so we accept that it has an unused terminal field

### Toolbox Mode Handling

The new `BuildRosaBoundaryCommand` method does NOT add the `flatpak-spawn --host` wrapper even when `runInToolbox` is true. This is intentional because:
- rosa-boundary should be installed in the same environment where SREPD runs
- If running in a toolbox, rosa-boundary should also be in that toolbox
- The terminal wrapper needs `flatpak-spawn` because the GUI terminal emulator runs on the host
- rosa-boundary is a CLI tool with no such requirement

## Testing Strategy

### Unit Tests
- ✅ Test direct execution without terminal wrapper
- ✅ Test variable substitution works correctly
- ✅ Test toolbox mode doesn't affect rosa-boundary command
- ✅ Comparison test showing difference from BuildLoginCommand

### Integration Testing
- ✅ All existing launcher tests pass
- ✅ All existing TUI tests pass (including rosa-boundary chord tests)

### Manual Testing (Post-Merge)
After merging, users should test:
1. Trigger rosa-boundary login from an incident with a cluster_id alert
2. Verify rosa-boundary executes directly without opening a new terminal window
3. Verify the session connection works as expected
4. Test in both normal and toolbox environments

## Risk Assessment

**Low Risk** - This change is isolated to rosa-boundary execution path:
- No changes to regular cluster login flow
- New method is additive (doesn't modify existing behavior)
- Existing tests ensure no regression
- Rosa-boundary launcher is already optional feature (only used if configured)

## Future Improvements

If we add more CLI tools that need direct execution (similar to rosa-boundary), we could:
- Create a `BuildDirectCommand()` method with a more generic name
- Add a launcher profile flag to indicate "direct execution" vs "terminal wrapped"
- Refactor to use a strategy pattern for different command building approaches

For now, the rosa-boundary-specific method is the simplest solution that solves the immediate bug.
