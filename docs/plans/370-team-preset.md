# 370: Team preset — srepd config --preset <file|https-url>

Issue: #353/#324 (onboarding overhaul — the "have to know to know" problem)
Branch: `srepd/team-preset`

## Problem

Some wizard values are pure team policy and fundamentally undiscoverable:
`custom_service_escalation_policies` mappings, an org-specific
`cluster_login_command`, sometimes which team/silent policy to pick. They are
"100% policy, not accurate or applicable to everyone" — no heuristic can
derive them, and hardcoding any org's choices into SREPD would be wrong.

## Approach: policy-as-artifact

A team publishes one YAML fragment in its onboarding docs; new SREs run
`srepd config --preset <file|https-url>` and inherit the decisions.

- **`pkgconfig.Preset` / `ParsePreset`**: strict allowlist — teams,
  default_silent_escalation_policy, custom_service_escalation_policies,
  cluster_login_command, terminal, editor. ANY other key (above all `token`
  and `llm_api`) rejects the whole preset loudly; a typo in a team preset
  should fail review, not be silently ignored.
- **`LoadPreset(ref, client)`**: file path or URL. URL guardrails: HTTPS
  only, 64KB cap (`presetMaxBytes`), non-200 rejected, explicit flag only
  (never auto-fetched), injectable client for tests, source recorded and
  shown in the wizard.
- **`ApplyPreset(existing, preset)`**: overlays only where existing is empty
  — the user's real config always wins over team defaults. Returns
  `PresetApplied` flags + source.
- **Wizard integration**: `prepareConfigWizardCmd` loads/applies the preset
  (load failure → hard error view; the user explicitly asked for it);
  keep-prompts show "(from preset: <source>)"; summary gets a "Preset
  applied" line; `ForcePresetChanges` marks seeded fields changed so they
  are persisted even though they equal the seeded "existing" values.
- **`viperConfiguredString`**: environment values count as existing only when
  actually user-set (config file / SREPD_* env) — `ensureViperDefaults`
  fills viper with in-process defaults that must not block preset overlay.
- **`cluster_login_command`** threaded through the write pipeline
  (ExistingConfig/ResolvedValues/changes/merge/BuildFullConfig) — it has no
  wizard step; it flows from config or preset straight to disk.
- Every value is still walked and confirmed in the wizard — a preset is
  defaults, not a config.

## Tests (TDD — written first)

`pkg/config/preset_test.go`: parse (valid/reject token/reject llm_api/reject
unknown/invalid YAML), apply (existing wins, empty fills, applied flags, nil
no-op), file load, HTTPS-only, TLS fetch via httptest, size cap, non-200,
change forcing. README documents the feature (config.go changed →
readme-check).

## Post-mortem (duplicate group, fixed alongside plan 378)

This PR's merge re-added the "Configure advanced options?" confirm group
to `buildConfigForm` during conflict resolution — the third identical
copy (see plan 369's post-mortem for the second). Users answered the
same prompt three times before the summary. Lesson: conflict resolutions
that re-apply a hunk "to be safe" duplicate silently when the hunk is an
element in a list literal; check group counts against both parents. A
walk-the-wizard regression test now asserts one enter dismisses the
prompt.
