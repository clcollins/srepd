# 372: Onboarding docs and headers (OB-2 + OB-7)

Issue: #353 (OB-2, OB-7); final documentation pass of the onboarding overhaul
Branch: `srepd/onboarding-docs`

## Problem

- OB-2: the README's getting-started path was a hand-edited YAML example
  with `token: <PagerDuty API token>` — following it created exactly the
  broken-but-present config OB-1 had to defend against, and it contradicted
  the README's own note that the wizard auto-launches.
- OB-7: the token-acquisition help lived only inside the wizard; the cobra
  `config` long description said nothing; two files still carried the
  `Copyright © 2025 NAME HERE <EMAIL ADDRESS>` placeholder header.
- Drift: the README's `terminal` default lacked the ` --`; the dead
  `flag_marker` key (issue #322) was still documented as live.

## Changes (docs land last so they describe the finished flow)

- **README "Getting Started"**: two steps — create a User Token (with the
  path), run `srepd`. Mentions team presets for orgs that publish one.
- **README "Advanced: manual configuration"**: the YAML example is demoted
  here, now led by `srepd config generate --out ...` and an explicit "do not
  copy placeholder values" warning; the example itself uses realistic
  non-placeholder forms and the correct `gnome-terminal --` default.
- Options table: `terminal` default corrected to `gnome-terminal --`;
  `flag_marker` row removed (dead key, markers follow `emoji`);
  `agent_cli_command` documents `""` = disabled.
- **`configCmd` long description** gains the token-acquisition path and the
  discovery/preset summary (OB-7).
- **Copyright headers** fixed in `cmd/config.go` and `pkg/config/config.go`.

## Verification

Docs-only plus comment/description strings — full CI suite still runs; the
`TestGenerateAnnotatedConfig_ActiveKeysMatchDefaults` guard keeps the
generate output aligned, and `TestClassifyConfigHealth_ReadmeExample...`
still pins that any legacy placeholder-styled config routes to the wizard.
