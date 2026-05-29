# 056 - README CI Check

## Context

When PRs modify files that define user-facing configuration, key bindings,
or CLI flags, the README.md should be updated to reflect those changes.
Without enforcement, documentation drift accumulates as contributors
change behavior without updating the README.

This is modeled after the existing `plan-check` CI job (see plan 003)
which enforces plan document presence on every PR.

## Plan

Add a `make readme-check` target and a corresponding GitHub Actions CI
job that fails PRs which modify any of the following files without also
including a README.md change:

- `pkg/tui/keymap.go` (key bindings)
- `cmd/config.go` (viper config keys, flags)
- `cmd/root.go` (CLI flags, env vars)

The check gracefully exits when there is no merge base (e.g. on the
default branch or initial commits).

## Files Modified

| File | Change |
|------|--------|
| `Makefile` | Add `readme-check` target after `plan-check` |
| `.github/workflows/go-ci.yml` | Add `readme-check` job (PR-only, like `plan-doc`) |
| `docs/plans/056-readme-ci-check.md` | This plan document |

## Verification

- `go test ./... -count=1` passes (no Go code changed)
- `make readme-check` runs without error on this branch (no trigger
  files modified)
- CI workflow YAML is valid and the new job mirrors the structure of
  the existing `plan-doc` job
