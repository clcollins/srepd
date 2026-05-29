# Plan 050: iTerm2 and Terminal.app Native Support

## Status: Implemented

## Summary

Adds native AppleScript-based terminal support for macOS terminals
(iTerm2 and Terminal.app), eliminating the need for wrapper shell scripts
documented in the README.

Previously, iTerm2 users needed to create a shell script that invoked
`osascript` to launch commands in iTerm2 windows. With this change, users
can simply set `terminal: iterm2` in their config and srepd constructs
the osascript command automatically.

## Approach

Since macOS terminals like iTerm2 and Terminal.app don't have simple
`-e` or `--` CLI flags for launching commands, they require AppleScript
via `osascript`. A new `AppleScriptProfile` type implements the
`TerminalProfile` interface and constructs the appropriate osascript
command internally.

When the user sets `terminal: iterm2`, srepd builds:
```
osascript -e 'tell application "iTerm2" to create window with default profile command "ocm-container -C abc123"'
```

When the user sets `terminal: terminal`, srepd builds:
```
osascript -e 'tell application "Terminal" to do script "ocm-container -C abc123"'
```

## Files Changed

- `pkg/launcher/profiles.go` -- Added `AppleScriptProfile` struct with
  `Name()` and `BuildCommand()` methods; added `appleScriptTerminals`
  map for case-insensitive terminal name to macOS app name mapping;
  updated `profileForExecName()` to check AppleScript terminals first;
  updated `validateTerminalExists()` to check for `osascript` when an
  AppleScript terminal is configured.

- `pkg/launcher/profiles_test.go` -- Added 13 tests covering detection,
  name, build command, error handling, quoting, and validation for both
  iTerm2 and Terminal.app.

## Design Decisions

- **AppleScriptProfile vs separate iTerm2Profile**: A single
  `AppleScriptProfile` handles both iTerm2 and Terminal.app with a
  switch on terminal name for the different AppleScript syntax. This
  is extensible to other AppleScript-controlled terminals without
  creating new profile types.

- **Case-insensitive matching**: Users may type "iterm2", "iTerm2", or
  "Terminal" -- all are normalized via `strings.ToLower()` for matching
  and the correct macOS application name is stored in `appName`.

- **osascript validation**: Since AppleScript terminals don't have their
  own binary in PATH, `validateTerminalExists()` checks for `osascript`
  instead. On Linux, this produces a warning (not an error) since the
  user may be developing config for a macOS machine.

- **No change to NewClusterLauncher flow**: The existing launcher flow
  works correctly because `BuildLoginCommand` delegates to the profile's
  `BuildCommand`, which returns the full osascript command. The terminal
  name in `l.terminal[0]` is only used for logging.

- **Backward compatibility**: The `alreadyHasSeparator` function returns
  false for `AppleScriptProfile` (unmatched by the switch), so the
  profile's `BuildCommand` is always used. Existing configurations are
  unaffected.

## Test Plan

- `TestDetectProfile_ITerm2` -- "iterm2" resolves to AppleScriptProfile
- `TestDetectProfile_ITerm2_MixedCase` -- "iTerm2" resolves correctly
- `TestDetectProfile_TerminalApp` -- "terminal" resolves to AppleScriptProfile
- `TestDetectProfile_TerminalApp_UpperCase` -- "Terminal" resolves correctly
- `TestAppleScriptProfile_Name_ITerm2` -- Name() returns "applescript (iterm2)"
- `TestAppleScriptProfile_Name_Terminal` -- Name() returns "applescript (terminal)"
- `TestAppleScriptProfile_BuildCommand_ITerm2` -- produces osascript with iTerm2 create window syntax
- `TestAppleScriptProfile_BuildCommand_TerminalApp` -- produces osascript with Terminal do script syntax
- `TestAppleScriptProfile_BuildCommand_EmptyTerminalArgs` -- returns error
- `TestAppleScriptProfile_BuildCommand_EmptyLoginCmd` -- returns error
- `TestAppleScriptProfile_BuildCommand_ITerm2_QuotesInCommand` -- handles multi-word commands
- `TestAppleScriptProfile_ValidateTerminal_ITerm2` -- checks for osascript, not iterm2
- `TestAppleScriptProfile_ValidateTerminal_Terminal` -- checks for osascript, not terminal
