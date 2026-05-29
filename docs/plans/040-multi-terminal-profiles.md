# Plan 040: Multi-Terminal Profiles

## Problem

The current `ClusterLauncher` in `pkg/launcher/launcher.go` concatenates
terminal args and login-command args with no awareness of how different
terminal emulators expect the boundary between their own flags and the
command to execute. Terminals fall into three categories:

- **Separator terminals** (gnome-terminal, ptyxis, wezterm, BlackBox,
  tmux): expect a `--` separator between terminal flags and the
  executed command.
- **Flag terminals** (konsole `-e`, alacritty `-e`, terminator
  `--execute`): use a specific flag to introduce the executed command.
- **Direct terminals** (kitty, foot, Contour): the executed command
  follows terminal flags directly with no separator or flag.

Users must currently encode the separator or flag into their terminal
config string, which is error-prone and undocumented.

## Solution

Introduce a `TerminalProfile` strategy interface with concrete
implementations for each category, plus a `GenericProfile` fallback
that preserves the current concatenation behavior.

A `DetectTerminalProfile` function auto-detects the correct profile
from the terminal command string (executable name or Flatpak app ID).

The `ClusterLauncher.BuildLoginCommand` method delegates to the
detected profile's `BuildCommand` method, keeping full backward
compatibility.

## Implementation

### New file: `pkg/launcher/profiles.go`

- `TerminalProfile` interface: `Name() string`,
  `BuildCommand(terminalArgs, loginCmd []string) ([]string, error)`
- `SeparatorProfile` — inserts `--` between terminal args and command
- `FlagProfile` — inserts a configurable flag (e.g., `-e`)
- `DirectProfile` — concatenates without any separator
- `GenericProfile` — identical to `DirectProfile`, used as fallback
- `DetectTerminalProfile(terminalCmd string) TerminalProfile`

### New file: `pkg/launcher/profiles_test.go`

Table-driven tests covering detection and command building for each
profile type, including Flatpak app IDs.

### Modified file: `pkg/launcher/launcher.go`

- `ClusterLauncher` gains a `profile TerminalProfile` field
- `NewClusterLauncher` calls `DetectTerminalProfile` during
  construction
- `BuildLoginCommand` delegates to `profile.BuildCommand` after
  variable replacement

## Backward Compatibility

- Existing configs that manually include `--` or `-e` in the terminal
  string continue to work because `GenericProfile` preserves the
  current concatenation behavior.
- Detection only activates when the executable name matches a known
  terminal; unknown terminals fall through to `GenericProfile`.

## Testing

- `make test-all` must pass
- All existing launcher tests must remain green
- New tests cover every profile type and the detection function

## Risks

- None significant. The refactor is additive and the fallback
  preserves exact existing behavior.
