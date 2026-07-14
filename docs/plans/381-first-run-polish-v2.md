# 381: First-run polish — welcome step and step breadcrumbs

Branch: `srepd/first-run-polish-v2`

## Problem

The wizard opened cold on a password input — no orientation, no token help
before the field demanding it, no explanation when auto-launched over a
broken config. No sense of progress through the steps.

## Approach

- Welcome Note group: what srepd is, token acquisition path, ctrl+c escape,
  OB-1 "You're here because" reason, reconfiguration acknowledgement.
- Step breadcrumbs: "Title · n/6" on six numbered milestones (token, teams,
  silent policy, environment, options, summary). Keep-confirm variants share
  their milestone number; conditional steps unnumbered.
- Integration tests updated with an extra enter to dismiss the welcome step.

## Files

- `pkg/tui/tui.go` — welcomeDescription, stepTitle, welcome group, numbered titles
- `pkg/tui/config_polish_test.go` — welcome/breadcrumb tests
- `pkg/tui/config_policy_picker_repro_test.go` — extra enter for welcome step
