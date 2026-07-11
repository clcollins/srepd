# 371: First-run polish — welcome step, breadcrumbs, styled summary

Issue: #353/#324 (onboarding overhaul — final wizard polish)
Branch: `srepd/first-run-polish`

## Problem

Three rough edges left on the wizard:
1. It opened cold on a password input — no orientation, no token-acquisition
   help before the field demanding the token, no explanation when the wizard
   auto-launched over a broken config (the PR-361 reason was log-only).
2. No sense of progress — users couldn't tell how many steps remained.
3. The summary was unstyled plain text.

## Approach

- **Welcome step**: a `Note` group opening the form — what srepd is, the
  token acquisition path (before the token field, not after failing it), the
  ctrl+c escape, and the OB-1 "You're here because: <reason>" line, which now
  flows PreRun → viper `config_wizard_reason` → `configWizardReadyMsg` →
  `welcomeDescription(isNewFile, reason)`. Reconfiguration runs acknowledge
  the existing config instead.
- **Breadcrumbs**: `stepTitle(title, n)` renders "Title · n/6" on the six
  numbered milestones (token, teams, silent policy, environment, options,
  summary). Keep-confirm variants share their milestone's number; welcome
  and the conditional AI/advanced/manual steps stay unnumbered — huh v1 has
  no current-group accessor, so numbering fixed milestones is the honest
  approach (documented in the roadmap plan).
- **Styled summary**: `BuildSummaryRows` (pkg/config) exposes the summary as
  `[]SummaryRow{Label, Value, Changed}`; `BuildSummary` re-renders the same
  rows as plain text (all existing tests pass unchanged, pinning that).
  `renderConfigSummary(rows, styles, presetSource)` renders muted labels,
  plain values, warning-colored "(changed)" markers, and the preset source
  line, replacing the plain-text summary in the wizard.

## Tests (TDD — written first)

- `pkg/config/summary_rows_test.go`: row structure, masking, name display,
  changed/unchanged flags; existing `BuildSummary` tests pin the text render.
- `pkg/tui/config_polish_test.go`: welcome copy (new user / with reason /
  reconfiguration), `stepTitle` format, summary rendering content + changed
  markers + preset line.
