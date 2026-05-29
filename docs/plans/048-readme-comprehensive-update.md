# Plan 048: Comprehensive README Update

## Problem

The README was significantly out of date, still documenting the old `PAGERDUTY_INCIDENT` and `ALERT_DETAILS` environment variables and missing documentation for many features that have been merged since the original README was written. PR #177 attempted to update it but became stale.

## Changes

Replace the entire README with a comprehensive rewrite covering:

1. **Features list** -- Updated to include all current features: SOP launcher, urgency filter, confirmation prompts, multi-cluster selection, action log, rate limiting, selection preservation, toolbox auto-detection, clickable markdown links, and individual PAGERDUTY_* env vars. Also notes upcoming features from open PRs (#183, #185, #187).

2. **Key Bindings section** -- Complete table organized by category (Navigation, Actions, Toggles, Input Mode, Quit), sourced directly from `pkg/tui/keymap.go`.

3. **Configuration section** -- Restructured with proper tables for required and optional keys. Added `toolbox_mode` config key. Fixed `cluster_login_cmd` vs `cluster_login_command` inconsistency (the correct key is `cluster_login_command`).

4. **Terminal Support section** -- Documents all supported terminals with config examples: gnome-terminal, ptyxis, konsole, kitty, alacritty, wezterm, foot, ghostty, terminator, tmux, macOS Terminal, iTerm2. Includes Flatpak terminal examples.

5. **Environment Variables section** -- Documents all 13 individual PAGERDUTY_* variables and REASON, replacing the old ALERT_DETAILS blob documentation.

6. **Build and Development section** -- Complete make targets table, Go version requirement, PR workflow, plan document requirement.

7. **Architecture section** -- Brief overview of packages and the MVU pattern.

## Verification

- All key bindings verified against `pkg/tui/keymap.go`
- All config keys verified against `cmd/config.go` and `cmd/root.go`
- All env vars verified against `pkg/tui/commands.go` on `origin/main`
- All make targets verified against `Makefile`
- Header image verified present on `origin/main`

## Risk

Low -- documentation-only change with no code modifications.
