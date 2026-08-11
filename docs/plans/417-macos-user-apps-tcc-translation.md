# Plan 417: ~/Applications probing + TCC error translation

**Branch:** `srepd/macos-terminal-phase3`

## Problem

Two remaining gaps from the Phase 3 macOS terminal plan:

1. **`~/Applications/` not probed:** Bundle detection and iTerm2 validation
   only check `/Applications/`. macOS users who install apps to their
   per-user `~/Applications/` directory are not detected.
2. **TCC errors cryptic:** macOS TCC (Transparency, Consent, and Control)
   permission denials produce raw osascript errors like `-1743` in the
   error modal. Users don't know what to do about them.

## Solution

### 1a. Probe ~/Applications for bundles

Changed `macOSBundleTerminal` struct to store `appName` (e.g., "kitty.app")
and `binaryRelPath` (e.g., "Contents/MacOS/kitty") instead of full paths.
`DetectTerminals` builds candidate roots `[/Applications, $HOME/Applications]`
using the injected `getenv("HOME")`, probes in order, first hit wins. The
full binary path is derived by joining the matched root + appName +
binaryRelPath.

Same two-root treatment for the iTerm2 check in `DetectTerminals`.

### 1b. Validate iTerm2 in both locations

`validateTerminalExists` for iterm2 now checks both `/Applications/iTerm.app`
and `~/Applications/iTerm.app` before warning.

### 2. Translate TCC / osascript failures

New `translateLoginStderr(stderr string) string` in `commands.go` — a pure
function that returns appended guidance when known macOS error patterns match:

- `-1743` / "Not authorized to send Apple events": guidance about System
  Settings > Privacy & Security > Automation, toggle workaround, and
  `tccutil reset AppleEvents`.
- `-600` / "application isn't running" / `-10810`: guidance to verify the
  app is installed and can be opened manually.

Wired into the `loginProcessExitedMsg` handler in `tui.go`: translation
appends to the detail string, never replaces the raw stderr.

## Files changed

| File | Change |
|------|--------|
| `pkg/launcher/detect.go` | `macOSBundleTerminal` struct refactored to relative paths; `~/Applications` probing; iTerm2 two-root check |
| `pkg/launcher/detect_test.go` | 4 new tests: user-home bundle, system precedence, no-HOME skip, iTerm2 from ~/Applications |
| `pkg/launcher/profiles.go` | `validateTerminalExists` checks both iTerm.app locations |
| `pkg/tui/commands.go` | `translateLoginStderr` pure function |
| `pkg/tui/commands_test.go` | Table-driven tests for `translateLoginStderr` |
| `pkg/tui/tui.go` | Wire `translateLoginStderr` into `loginProcessExitedMsg` handler |
| `pkg/tui/cluster_login_test.go` | Handler-level test for TCC -1743 guidance |
| `docs/terminals.md` | Updated detection docs, TCC section, removed resolved limitations |
| `docs/plans/416-macos-terminal-coverage.md` | Removed resolved limitations |

## macOS testing note

No macOS device available. Validated by unit tests with injectable
`fakeStat`/`fakeGetenv`/`fakeGetenv(HOME)` for detection, pure function
tests for TCC translation, and handler-level test for the wiring.
