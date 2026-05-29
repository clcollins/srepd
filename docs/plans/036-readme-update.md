# Update README with new features, key bindings, and terminal examples

## Context

The README.md has not been updated to reflect many features added since
the initial release. Key bindings, new features (SOP launcher, urgency
filter, confirmation prompts, multi-cluster selection, action log, rate
limiting), build targets, Go version, and additional terminal examples
are all missing or incomplete. One config key (`cluster_login_cmd` in the
example YAML) is inconsistent with the actual config key name
(`cluster_login_command`).

## Plan

1. Add a "Key Bindings" section documenting all bindings from
   `pkg/tui/keymap.go`, organized by category (Navigation, Actions,
   Toggles, Quit)
2. Update the Features list with all features added since initial
   release
3. Add terminal configuration examples for ptyxis, konsole, kitty,
   alacritty, wezterm, and foot
4. Fix the `cluster_login_cmd` typo in the example YAML to use the
   correct `cluster_login_command` key
5. Add a "Building" section listing all available `make` targets
6. Document the Go 1.26.3 version requirement
7. Document the `PAGERDUTY_INCIDENT` and `ALERT_DETAILS` environment
   variables (already partially documented, verify accuracy)
8. Clean up the Planned Features list to remove features that have
   already been implemented

## Files Modified

- `README.md` -- comprehensive update with all new sections and fixes
- `docs/plans/036-readme-update.md` -- this plan document

## Verification

- Visual review of README.md for completeness and accuracy
- All key bindings cross-referenced with `pkg/tui/keymap.go`
- Config key names verified against `cmd/config.go` and
  `pkg/launcher/launcher.go`
- Make targets verified against `Makefile`
- Go version verified against `go.mod`
- Terminal examples based on CONVENTIONS.md platform list
