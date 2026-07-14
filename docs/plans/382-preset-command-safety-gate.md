# 382: Preset command safety gate

Branch: `srepd/preset-command-safety-gate`
Related: plan 374 (supply-chain hardening), plan 370 (team presets)

## Problem

A `--preset` file or HTTPS URL is remote input, and three of its allowed
keys — `terminal`, `editor`, `cluster_login_command` — are commands srepd
executes. A malicious preset could plant arbitrary command execution
behind an innocuous-looking team config, and the wizard's summary alone
is easy to enter through.

## Approach

When a preset seeded any executable field
(`PresetApplied.ExecutableAny()`: terminal, editor, or cluster login),
the wizard requires two extra confirmations after the final
"Save changes?":

1. A **bold red** "⚠ SECURITY" confirm listing every preset-supplied
   command with its final value ("They are safe" / "No — discard",
   default No).
2. "Are you sure you trust the source?" showing the preset file/URL
   ("I trust this source" / "No — discard", default No).

Declining either discards the save. The gate is enforced twice: by the
group flow (the trust confirm hides unless the command review was
affirmed) and re-checked at form completion in `switchConfigFocusMode`,
so a future form-navigation bug cannot skip it. The red styling is
deliberately theme-independent (bold, ANSI 196) so it reads as an alarm.

Never gated: wizard-typed values, existing configs, and API-only preset
fields (teams, silent policy, custom mappings) — those are IDs sent to
the PagerDuty API, not executed.

Presets cannot set the agent CLI or agentic prompts today (the allowlist
rejects them); a tripwire comment on `presetAllowedKeys` requires any
future executable or agentic-prompt key to join `ExecutableAny`.

## Tests (TDD — written first)

- `pkg/tui/config_preset_safety_test.go` drives the real wizard via the
  message pump through save → red warning → trust confirm:
  affirm-both path (config written), decline path (nothing written),
  no-preset path (no gate), API-fields-only preset (no gate).
- `TestPresetApplied_ExecutableAny` covers the gate predicate.
