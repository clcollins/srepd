# Plan 026: Migrate charmbracelet/log to v1.0.0

**Status**: Complete
**Predecessor**: [022-bump-bubbletea.md](022-bump-bubbletea.md)

## Summary

Upgrade `github.com/charmbracelet/log` from v0.4.2 to v1.0.0.

## Analysis

charmbracelet/log v1.0.0 is an honorary stable release. The only API change
is the `interface{}` to `any` type alias migration, which is fully backward
compatible in Go (they are identical types). No code changes are required in
srepd -- only `go.mod` and `go.sum` need updating.

## Changes

- `go.mod` / `go.sum`: bump `github.com/charmbracelet/log` v0.4.2 -> v1.0.0

## Verification

- `go test ./...` passes with no failures
- `go vet ./...` reports no issues
