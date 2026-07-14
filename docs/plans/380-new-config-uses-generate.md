# 380: New config files use the annotated generate template

Branch: `srepd/new-config-uses-generate`

## Problem

When the wizard saved a brand-new config file, `BuildFullConfig` wrote a bare
YAML with no section headers, no comments, and no detected alternatives.
Meanwhile `srepd config generate` produced a richly annotated file with
environment detection (terminals, editor, cluster login, agent), commented
alternatives, and section headers. The first config a new user created was
the least helpful format.

## Approach

`WriteConfig` for `isNewFile` now generates the annotated template via
`GenerateAnnotatedConfig(env)` — the same output as `srepd config generate`
— then overlays the wizard's resolved values using `MergeIntoExistingConfig`.
The result has both the annotations AND the real values.

`detectGenerateEnvironment()` in `pkg/tui/commands.go` probes terminals,
$EDITOR, claude, and ocm/ocm-container — reusing the same detection from
`cmd/generate.go` but callable from the TUI package.

`WriteConfig` gains an `env *GenerateEnvironment` parameter; existing-file
writes pass nil (no template needed). All test callers updated.

## Files

- `pkg/config/config.go` — `WriteConfig` signature + new-file path
- `pkg/tui/commands.go` — `detectGenerateEnvironment` + env plumbing
- `pkg/config/config_test.go` — updated callers + annotation assertion
