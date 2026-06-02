# Auto-update: notify on new releases and optionally self-update

## Context

Users have no way to know when a new SREPD release is available.
This adds update detection via GitHub API, notification in the
bottom status bar, and optional self-update.

## Changes

- Embed Version at build time via goreleaser ldflags
- checkForUpdate() async command checks GitHub releases API
- Scheduled hourly update check (also runs on startup)
- Bottom status bar shows "v1.0.0 - abc1234" normally,
  "Update: v1.0.0→v1.1.0" when update available
- Dev mode always shows update available (v99.0.0)
- auto_update config key and --auto-update flag for self-update
- downloadUpdate() downloads and replaces binary

## Verification

- make test-all passes
- Dev mode shows update notification
- Real release shows version in status bar
- Auto-update downloads and replaces binary when enabled
