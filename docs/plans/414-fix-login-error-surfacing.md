# Plan 414: Surface terminal login errors and remove tmux from separator profiles

**Branch:** `srepd/fix-login-error-surfacing`

## Problem

When a terminal process launched by cluster login exits with an error,
srepd silently swallows it. The `login()` function discards stderr
(exec.Cmd.Stderr is nil) and reaps the child process in a fire-and-forget
goroutine that only logs at Debug level. The user sees "login completed"
with no indication anything went wrong.

This was discovered when using `tmux -C` as the terminal — tmux failed
with "error connecting to socket (No such file or directory)" but srepd
showed no error. Additionally, tmux was incorrectly listed as a separator
terminal despite being a multiplexer, not a terminal emulator.

## Solution

### 1. Capture stderr and surface exit errors

- Add `bytes.Buffer` on `exec.Cmd.Stderr` to capture terminal process stderr
- Add `loginProcessExitedMsg` type carrying `exitErr` and `stderr`
- Add `waitCmd tea.Cmd` field to `loginFinishedMsg` — a blocking command
  that waits on `c.Wait()` and returns `loginProcessExitedMsg`
- Handle `loginProcessExitedMsg` in the Update loop: on error, route
  through `errMsg` to the full-screen error modal (not the transient
  status bar, which gets overwritten by polls within seconds)

### 2. Remove tmux from separator terminals

tmux is a terminal multiplexer, not a terminal emulator. It cannot open
GUI windows on its own and should not be in `separatorTerminals`. Remove
it so it falls through to `GenericProfile` (direct concatenation).

## Key design decisions

- **Error modal vs status bar:** Terminal exit errors go through `errMsg`
  → `errMsgHandler` → full-screen error view. The status bar is too
  transient — background polls overwrite it within seconds.
- **waitCmd pattern:** Rather than blocking the entire `login()` command
  on `c.Wait()` (which would freeze srepd until the terminal exits),
  the wait is a separate `tea.Cmd` dispatched from `loginFinishedMsg`.
  srepd stays responsive while the terminal runs.
- **Stderr capture is best-effort:** For terminal emulators that open a
  GUI window (gnome-terminal, ptyxis, etc.), stderr may be empty even on
  failure. The exit code alone is still surfaced. For tools like tmux or
  osascript, stderr contains the actual error message.

## Files changed

| File | Change |
|------|--------|
| `pkg/tui/commands.go` | Add `loginProcessExitedMsg`, `waitCmd` field, stderr capture |
| `pkg/tui/tui.go` | Handle `loginProcessExitedMsg`, dispatch `waitCmd` |
| `pkg/launcher/profiles.go` | Remove tmux from `separatorTerminals` |
| `pkg/tui/cluster_login_test.go` | Tests for new message types and dispatch |
| `pkg/tui/commands_test.go` | Tests for waitCmd, exit error, stderr capture |
| `pkg/launcher/launcher_test.go` | Update tmux profile expectation |
| `pkg/launcher/profiles_test.go` | Update tmux profile detection test |

## Related investigation

A separate investigation into AppleScript terminal support (iTerm2,
Terminal.app) identified three additional bugs not addressed in this PR:
1. Environment variable passing is broken for AppleScript terminals
2. No escaping of login command inside AppleScript strings
3. macOS TCC permissions cause silent failures

These findings are documented in the PR description for future work.
