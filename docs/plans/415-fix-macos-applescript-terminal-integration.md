# Plan 415: Fix macOS AppleScript terminal integration

**Branch:** `srepd/fix-macos-terminal-integration`

## Problem

AppleScript terminal support (iTerm2, Terminal.app) has three bugs:

1. **Env vars broken:** `commandContainsOCMContainer()` in `login()`
   matched "ocm-container" inside the AppleScript string at `argv[2]`,
   then `-e KEY=VAL` flags were appended as extra `osascript` arguments
   rather than ocm-container arguments. Even if detection worked,
   `strings.Join` flattening in `AppleScriptProfile.BuildCommand` would
   re-tokenize env values containing spaces (incident titles, service
   names) into extra arguments.
2. **No escaping:** Login command interpolated raw into AppleScript
   double-quoted string. `"` or `\` in values would break the script.
3. **TCC permissions:** macOS TCC denial caused silent -1743 error.
   Now surfaced by PR #420's error modal (already fixed).

## Root cause

The `login()` function operated on the post-profile-wrap command. For
AppleScript profiles, the login command is buried inside an AppleScript
string literal — there's no way to reliably splice `-e` flags or
preserve argv boundaries at the exec.Cmd level.

## Solution

### 1. Wrapper script for ALL AppleScript terminals

Route all AppleScript terminal launches through a wrapper script at
`~/.cache/srepd/launch/login-<random>.sh`. The script:
- Exports env vars with single-quote escaping (`'\''` idiom)
- exec's the login command with each arg individually quoted
- Preserves argv boundaries through the AppleScript → shell transition

`NeedsWrapperScript()` on `ClusterLauncher` returns true whenever the
profile is `*AppleScriptProfile` — both ocm-container and non-ocm flows.

### 2. Pre-wrap ocm-container detection

`LoginCommandContainsOCMContainer()` checks the raw
`clusterLoginCommand` field (before profile wrapping), not the
post-wrap command. When true and the profile is AppleScript, `-e`
flags are inserted into the raw login command before writing the
wrapper script — they land inside the properly-quoted exec line.

### 3. AppleScript escaping (defense-in-depth)

`appleScriptEscape()` escapes `\` → `\\` and `"` → `\"` in strings
interpolated into AppleScript. Applied in `AppleScriptProfile.BuildCommand`.

### 4. Startup cleanup

`CleanupWrapperScripts()` prunes `.sh` files older than 1 hour from
the launch directory at startup.

## Key design decisions

- **TerminalProfile interface unchanged:** No changes to `BuildCommand`
  signature. The fix is entirely in the launcher and `login()` layers.
- **All AppleScript through wrapper:** Even ocm-container with
  AppleScript uses the wrapper. `strings.Join` flattening breaks env
  values with spaces, which routinely appear in PagerDuty data.
- **`BuildLoginCommandWithEnv` kept for non-AppleScript ocm profiles:**
  SeparatorProfile, FlagProfile, DirectProfile all preserve argv
  boundaries through exec.Command, so the simpler inject-then-wrap
  path works correctly for them.

## Files changed

| File | Change |
|------|--------|
| `pkg/launcher/launcher.go` | Add `LoginCommandContainsOCMContainer`, `NeedsWrapperScript`, `BuildLoginCommandWithEnv`, `BuildRawLoginCommand`, `BuildLoginCommandForScript`, export `InsertEnvFlagsAfterOCMContainer` |
| `pkg/launcher/wrapper.go` | **New:** `WriteWrapperScript`, `CleanupWrapperScripts`, `WrapperScriptDir`, `shQuote` |
| `pkg/launcher/profiles.go` | Add `appleScriptEscape`, apply in `AppleScriptProfile.BuildCommand` |
| `pkg/tui/commands.go` | Refactor `login()` env var flow to use pre-wrap detection; remove `commandContainsOCMContainer` and `insertOCMContainerEnvFlags` (moved to launcher) |
| `cmd/root.go` | Call `CleanupWrapperScripts()` at startup |
| `pkg/launcher/launcher_test.go` | Tests for all new launcher methods |
| `pkg/launcher/wrapper_test.go` | **New:** wrapper script tests including space-in-env regression |
| `pkg/launcher/profiles_test.go` | Tests for `appleScriptEscape` and escaping in `BuildCommand` |
| `pkg/tui/commands_test.go` | Remove tests for deleted functions |

## macOS testing note

No macOS device available for testing. The fix is validated by:
- Unit tests for all new functions
- Regression test for ocm-container + AppleScript with space-containing env values
- All existing tests pass (no View() changes, no golden snapshot changes)
- Build and `--dev` mode validation on Linux

## Related

- PR #420: Surface terminal login errors, remove tmux from separators
- Plan 414: Login error surfacing investigation findings
