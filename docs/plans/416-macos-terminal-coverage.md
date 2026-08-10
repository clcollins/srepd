# Plan 416: macOS terminal coverage — bundle detection, iTerm2 validation, PR #421 review fixes

**Branch:** `srepd/macos-terminal-coverage`

## Problem

Three detection/validation gaps remain after PR #421 (AppleScript env
var fix), plus four items from the PR #421 review:

1. **Bundle-installed terminals invisible:** kitty, alacritty, wezterm
   installed as `/Applications/*.app` on macOS are not on PATH, so
   `DetectTerminals()` misses them — the config wizard never offers
   them.
2. **iTerm2 unconditionally offered:** always appended on darwin even
   when `/Applications/iTerm.app` doesn't exist.
3. **validateTerminalExists too permissive:** only checks for `osascript`
   (always present on macOS), not whether the target app is installed.
4. **login() integration tests missing:** no tests exercise `login()`
   with an AppleScript terminal at the integration seam where the
   original env var bug lived.
5. **Env var duplication:** ocm-container + wrapper flow passes env vars
   both as `-e` flags AND as `export` lines in the wrapper script.
6. **Build*Command code duplication:** three methods copied the same
   profile-resolve-and-wrap pattern.

## Solution

### Bundle detection

Added `macOSBundleTerminals` map (kitty, alacritty, wezterm) with full
binary paths (e.g., `/Applications/kitty.app/Contents/MacOS/kitty`) and
`statFn func(string)(os.FileInfo, error)` parameter to `DetectTerminals`.
On darwin, after PATH probing, each known bundle's `.app` path is stat'd;
if the bundle exists and the terminal wasn't already found via PATH, it's
appended with the full binary path as Command so `exec.Command` can find
it. PATH-found terminals take precedence and keep their bare name.

Ghostty is intentionally excluded: its macOS CLI cannot reliably launch
the terminal — the supported route is `open -na Ghostty.app`, which needs
its own profile (ghostty#5739, #10203).

### Conditional iTerm2

Replaced unconditional iTerm2 append with a `statFn("/Applications/iTerm.app")`
check. Terminal.app stays unconditional (built into macOS).

### validateTerminalExists enhancement

For `iterm2`, after confirming `osascript` exists, also checks
`/Applications/iTerm.app` via `os.Stat`.

### login() integration tests

Two tests in `commands_test.go`, both using `t.TempDir()` via
`SREPD_TEST_WRAPPER_DIR` env override to avoid writing to the real cache:
- Wrapper branch wins for AppleScript (osascript in error proves path)
  and wrapper script is created with correct content
- OCM-container flow does NOT duplicate env vars as exports

### Env var duplication fix

When `LoginCommandContainsOCMContainer()` is true in the wrapper branch,
env vars are passed only as `-e` flags; the `export` lines are skipped.

### Shared command-build helper

Extracted `buildTerminalCommand()` in `launcher.go`. All three public
`Build*Command` methods delegate to it, eliminating ~50 lines of
duplicated profile-resolve-and-wrap logic.

## Files changed

| File | Change |
|------|--------|
| `pkg/launcher/detect.go` | `macOSBundleTerminals` map, `statFn` param, bundle probe, conditional iTerm2 |
| `pkg/launcher/detect_test.go` | `fakeStat` helper, bundle/iTerm2/PATH-precedence tests, updated existing calls |
| `pkg/launcher/profiles.go` | `validateTerminalExists` iterm2 app check |
| `pkg/launcher/launcher.go` | Extracted `buildTerminalCommand` shared helper |
| `pkg/launcher/launcher_test.go` | Comment on AppleScript test re: login() bypass |
| `pkg/tui/commands.go` | Fix env duplication, pass `os.Stat` to `DetectTerminals` |
| `pkg/launcher/wrapper.go` | `SREPD_TEST_WRAPPER_DIR` env override for test isolation |
| `pkg/tui/commands_test.go` | 2 login() integration tests for AppleScript flows (temp dir isolation) |
| `cmd/generate.go` | Pass `os.Stat` to `DetectTerminals` |
| `pkg/tui/tui.go` | Pass `os.Stat` to `DetectTerminals` |
| `README.md` | macOS terminal support section |

## Documentation

Added `docs/terminals.md` covering all supported terminals across Linux,
Flatpak, macOS, and Toolbox environments, including profile types,
environment variable passing mechanisms, wrapper scripts, variable
substitution, and configuration examples.

## Known limitations (acknowledged, not blocking)

- `SREPD_TEST_WRAPPER_DIR` is a test-only hook for wrapper script
  directory isolation; not intended as user-facing configuration.
- `~/Applications/` is not probed for bundle detection or iTerm2
  validation — only `/Applications/`. Per-user app installs are uncommon
  enough that this is future work if users request it.
- Ghostty macOS bundle needs an `open -na Ghostty.app`-based launch
  profile; currently only works when on `$PATH`.
- TCC error translation: macOS permission denials produce cryptic
  `-1743` AppleScript errors. Future work to detect and surface a
  user-friendly message.

## macOS testing note

No macOS device available. Validated by unit tests with injectable
`fakeStat`/`fakeLookPath`, integration tests proving the wrapper path
is taken, and build + `--dev` mode on Linux.
