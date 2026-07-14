# 369: srepd config generate — annotated default config

Issue: #324 item 2; part of the onboarding overhaul (#353, #324)
Branch: `srepd/config-generate`

## Problem

Some users prefer reading and editing a file over clicking through a wizard.
There was no way to get a complete, commented config: the README example was
partial (and its placeholders created OB-1's broken-config trap), and
`BuildFullConfig` is only reachable through the wizard's save path.

## Approach

- **`pkgconfig.GenerateAnnotatedConfig()`**: renders the full supported key
  surface with comments — required keys (token with the acquisition path,
  teams with the exactly-one tip) emitted EMPTY so a generated file routes
  into the wizard (OB-1) rather than failing auth on a placeholder; active
  environment/behavior keys at their `DefaultOptionalKeys` values (asserted
  in tests so the file can't drift from code defaults); optional sections
  (escalation policies, AI/llm_api, colors) present but commented out. The
  dead `flag_marker` key (#322) is deliberately absent.
- **`srepd config generate`** cobra subcommand (`cmd/generate.go`): stdout by
  default; `--out <path>` writes 0600 (a token will be pasted into it),
  refusing to overwrite without `--force`. `runConfigGenerate(w, outPath,
  force)` is a pure-ish seam for tests.
- README commands table gains the row.

## Tests (TDD — written first)

- `pkg/config/generate_test.go`: output is valid YAML; required-key help
  text; active defaults match `DefaultOptionalKeys` exactly; optional
  sections commented; no `flag_marker`; the generated file classifies
  `HealthNeedsWizard`.
- `cmd/generate_test.go`: stdout write; `--out` file with 0600 perms;
  refuses overwrite without `--force`; `--force` overwrites.

## Post-mortem (duplicate group, fixed alongside plan 378)

This PR's merge re-added the "Configure advanced options?" confirm group
to `buildConfigForm` while resolving conflicts with the #368 gating work,
leaving two identical copies bound to the same value (PR #371's merge
added a third). Users had to answer the same prompt repeatedly before
reaching the summary. Lesson: when resolving conflicts in the wizard's
group list, diff the *count* of groups against both parents — identical
adjacent groups are silent in review diffs and every test passed because
each copy individually behaved correctly. A walk-the-wizard regression
test now asserts one enter dismisses the prompt.
