# Plan 082: Team selection TUI + --list-teams

## Context

Building on Mhodesty's PR #274 contribution, this adds interactive team
selection using charmbracelet/huh's MultiSelect component. Two entry points
trigger the same flow:

1. Auto on first run — placeholder teams detected in config
2. Explicit `srepd config --list-teams` command

Both fetch the user's PD teams, present a multi-select screen styled to
match srepd, write selections to the config file, and launch the main TUI.

## Changes

- Add `pd.GetCurrentUserTeams` using the existing `PagerDutyClientInterface`
- Add `configFS.ReadFile`/`WriteFile` for testable config updates
- Use `yaml.Node` round-trip parsing to preserve comments when updating teams
- Add `huh.MultiSelect` team selection screen embedded in the Bubble Tea model
- Add `--list-teams` flag triggering the interactive flow
- Detect placeholder teams on startup and auto-trigger selection

## Verification

- `make test-all` passes
- Manual test: `srepd config --create`, add token, run `srepd` — team selector appears
- Manual test: `srepd config --list-teams` — same team selector, then launches TUI
