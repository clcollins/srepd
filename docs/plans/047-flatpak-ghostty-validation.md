# Plan 047: Flatpak Auto-Detection, Ghostty Support, and Terminal Validation

## Status: Implemented

## Summary

Adds four features to the terminal profile system introduced in plan 040:

1. **Flatpak app ID auto-detection**: If the terminal config value is a
   reverse-DNS name (e.g., `org.kde.konsole`), automatically prepend
   `flatpak run` before executing. The inner terminal is resolved from
   the existing `flatpakAppTerminals` map to select the correct profile.

2. **Redundant `flatpak run` prefix warning**: If the user manually writes
   `flatpak run org.kde.konsole` in their config, emit a startup warning
   suggesting they simplify to just `org.kde.konsole`.

3. **Terminal existence validation**: On startup, verify the terminal
   command exists on the system using `exec.LookPath` (bare commands),
   `os.Stat` (full paths), or checking for the `flatpak` binary
   (Flatpak app IDs). Emits a warning (not an error) if not found.

4. **Ghostty terminal support**: Add `ghostty` to the `flagTerminals`
   map with the `-e` flag, matching the same profile category as konsole
   and alacritty.

## Files Changed

- `pkg/launcher/profiles.go` — Added `isFlatpakAppID()`,
  `detectRedundantFlatpakPrefix()`, `validateTerminalExists()`;
  updated `DetectTerminalProfile()` to handle bare Flatpak app IDs;
  added `ghostty` to `flagTerminals` map.

- `pkg/launcher/launcher.go` — Updated `NewClusterLauncher()` to
  prepend `flatpak run` for Flatpak app IDs, warn on redundant
  prefix, and validate terminal existence.

- `pkg/launcher/profiles_test.go` — Added tests for all four features:
  `TestIsFlatpakAppID_Valid/Invalid`, `TestDetectProfile_FlatpakAppID*`,
  `TestDetectProfile_Ghostty*`, `TestGhostty_BuildCommand`,
  `TestRedundantFlatpakPrefix_*`, `TestValidateTerminal_*`.

## Design Decisions

- **Warnings, not errors**: Terminal validation and redundant prefix
  detection emit log warnings rather than returning errors. The user
  might install the terminal later, or might be testing config on a
  different machine.

- **Flatpak app ID heuristic**: A string with 2+ dots, no spaces, and
  no leading `/` is treated as a Flatpak app ID. This covers all
  standard reverse-DNS patterns without false positives on file paths
  or bare commands.

- **flatpak-spawn exclusion**: The redundant prefix check does NOT flag
  `flatpak-spawn --host flatpak run ...` because that pattern serves a
  legitimate purpose (launching from inside a Toolbox container).

- **Profile detection uses original terminal string**: The profile is
  detected from the original terminal config string (before `flatpak run`
  is prepended), so the inner terminal is resolved correctly via the
  `flatpakAppTerminals` map.

## Test Plan

- `TestIsFlatpakAppID_Valid` covers 9 valid app ID patterns
- `TestIsFlatpakAppID_Invalid` covers 7 invalid patterns (bare names,
  paths, strings with spaces, single-dot names)
- `TestDetectProfile_FlatpakAppID` verifies bare app ID resolves to
  the correct profile type
- `TestDetectProfile_Ghostty*` verifies ghostty detection with bare
  name, args, and full path
- `TestGhostty_BuildCommand` verifies the `-e` flag is inserted
- `TestRedundantFlatpakPrefix_*` covers detection and non-detection
  scenarios including flatpak-spawn exclusion
- `TestValidateTerminal_*` covers non-existent commands, Flatpak app
  IDs, non-existent paths, and empty strings
