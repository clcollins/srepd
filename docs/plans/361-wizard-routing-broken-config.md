# 361: Route broken/placeholder configs into the wizard (OB-1)

Issue: #353 (OB-1), part of the onboarding overhaul (#353, #324)
Branch: `srepd/wizard-routing-broken-config`

## Problem

The config wizard only auto-launches when the config file is **absent**
(root.go `Run`). A config file that exists but is broken — the most likely
real-world first-run failure — dead-ends instead:

- Missing required key → `PreRun` → `validateConfig()` → `log.Fatal`, no hint.
- Placeholder token (the literal `<PagerDuty API token>` from the README
  example) → passes validation, then fails at PD auth with a red ERROR view
  and no recovery path.

Placeholder detection existed for teams only (`HasPlaceholderTeams`), with a
private duplicate in `pkg/tui` (`hasPlaceholderTeamsCfg`), and it missed the
README's own `<team ID>` form.

## Approach

Classify config health once, before validation, and route wizard-shaped
problems into the existing in-TUI wizard:

- `pkg/config/health.go` — new:
  - `HasPlaceholderToken(string) bool`: empty/whitespace or `<...>`
    angle-bracket placeholder.
  - `ClassifyConfigHealth(token, teams, settings) (ConfigHealth, reason)`:
    `HealthOK | HealthNeedsWizard | HealthInvalid`. NeedsWizard = missing or
    placeholder token/teams. Invalid = structural damage the wizard's AST
    merge cannot safely fix (e.g. `service_escalation_policies` not a map).
    Token/teams come from viper accessors so `SREPD_*` env values count.
- `cmd/root.go`:
  - `classifyStartup()` maps health → `routeNormal | routeWizard | routeFatal`.
  - `PreRun`: `routeWizard` sets `needsWizard` + logs the reason instead of
    Fatal-ing; `routeFatal` keeps Fatal with "fix or remove <file>, or run
    `srepd config`" guidance; `routeNormal` proceeds to `validateConfig()` +
    auto-migration as before.
  - `Run`: `needsWizard` → `ensureViperDefaults()` + `launchTUIWithConfig()`.
- `validateConfig()` required-key check now falls back to `viper.GetString`
  before declaring a key missing (AllSettings omits env-resolved values).
- `HasPlaceholderTeams` generalized: any `<...>` entry is a placeholder (was:
  only the `<PagerDuty Team ID` prefix, which missed the README's `<team ID>`).
- Dedupe: `tui.hasPlaceholderTeamsCfg` deleted; `tui.go` Init uses
  `pkgconfig.HasPlaceholderTeams`.

The wizard-launch reason is logged (`Config incomplete — launching setup
wizard`). Surfacing it inside the wizard's welcome step is deferred to the
first-run polish PR of this overhaul, which adds the welcome group.

## Key design decisions

- **Classifier is pure and env-aware**: takes token/teams as values (callers
  use viper accessors) plus the raw settings map only for structural checks.
  This keeps it testable and avoids misrouting `SREPD_TOKEN`-only users.
- **Structural errors stay fatal**: the wizard's merge path edits the YAML AST
  in place; routing a malformed document into it risks data loss. HealthInvalid
  is deliberately narrow.
- **No signature changes**: `launchTUIWithConfig()` and `InitialModel` are
  untouched; routing is a flag between PreRun and Run.

## Tests (TDD — written first)

- `pkg/config/health_test.go`: `HasPlaceholderToken` table (10 cases);
  `ClassifyConfigHealth` table (10 cases incl. structural-wins ordering);
  generalized `HasPlaceholderTeams`; regression: the verbatim README example
  YAML must classify `HealthNeedsWizard`; empty-file case.
- `cmd/wizard_routing_test.go`: `classifyStartup` routes for valid /
  placeholder-token / missing-token / missing-teams / invalid-map configs;
  env-token routes normal; `validateConfig` accepts env-supplied required keys.
