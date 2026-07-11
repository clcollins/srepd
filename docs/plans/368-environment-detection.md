# 368: Environment detection — terminal, editor, AI agent (OB-5)

Issue: #353 (OB-5), #324 agent item; part of the onboarding overhaul
Branch: `srepd/environment-detection`

## Problem

- The `terminal` default (`gnome-terminal --`) is wrong on macOS and
  non-GNOME systems, the wizard never surfaced it, and a bad terminal only
  failed as a journal warning at cluster-login time (OB-5).
- `editor` silently defaulted to vim, ignoring `$EDITOR`.
- The AI agent config (`agent_cli_command`) existed but was invisible in the
  wizard (#324): users had to know it was there.
- None of terminal/editor/agent were part of the wizard's resolve → detect →
  write pipeline at all — `BuildFullConfig` wrote hardcoded defaults.

## Approach

**Detection (`pkg/launcher/detect.go`)**: `DetectTerminals(lookPath, getenv,
goos)` probes PATH for every executable the profile registry understands,
ranked: `$TERM_PROGRAM` match first, tmux next (only when `$TMUX` is set — a
tmux window outside a session can't open), rest in probe order; darwin always
includes Terminal.app/iTerm2 (osascript-launched, not on PATH). Fully
injectable for tests.

**Wizard environment group** (after the policy steps):
- Terminal `Select`: current setting first, annotated "(current)" or
  "(current — not found on this system!)" (the OB-5 surfacing), then
  detected terminals deduplicated (`buildTerminalOptions`). Flatpak app IDs
  are exempt from the not-found warning (reverse-DNS heuristic). Falls back
  to the default option when nothing is detected.
- Editor `Input`: prefilled `resolveEditorDefault`: config → `$EDITOR` →
  `$VISUAL` → vim.
- AI confirm: shown only when `shouldOfferAgentSetup` — claude on PATH AND
  the agent command is unset/default (a customized command means the user
  already configured it; no claude → silent skip per #324). Yes → default
  `claude --print`; No → `agent_cli_command: ""` persisted (deliberate
  disable). Known quirk: `ensureViperDefaults` resets an empty agent command
  in the wizard session itself; normal launches respect the disable.

**Write path (`pkg/config`)**: `ExistingConfig`/`WizardInputs`/
`ResolvedValues` gain Terminal/Editor/Agent fields (`AgentTouched`
distinguishes "step ran, empty = disabled" from "step hidden, keep
existing"); `DetectChanges`/`DetectChangesForNewFile` cover them;
`MergeIntoExistingConfig` upserts the three keys; `BuildFullConfig` writes
the resolved terminal/editor instead of hardcoded defaults; `BuildSummary`
shows changed environment values. `prepareConfigWizardCmd` populates the
existing values from viper.

## Tests (TDD — written first)

- `pkg/launcher/detect_test.go`: probe results, none-found, `$TERM_PROGRAM`
  ranking, tmux only-inside-session, darwin Apple terminals + ranking,
  linux exclusion.
- `pkg/tui/config_environment_test.go`: terminal options (current-first
  annotation, missing-current warning, dedup), editor default chain, agent
  offer gating.
- `pkg/config/environment_test.go`: resolve (inputs win / fallback /
  deliberate disable), change detection, BuildFullConfig uses resolved
  values + defaults when empty + persists a disable, merge upserts while
  preserving untouched keys.
