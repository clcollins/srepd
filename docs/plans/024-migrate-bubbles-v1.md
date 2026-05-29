# Upgrade bubbles from v0.21.0 to v1.0.0

## Context

bubbles v1.0.0 is an honorary stable release with zero breaking API
changes. Includes table cursor out-of-bounds fix and cursor data
race fix from v0.21.1.

Predecessor: [022-bump-bubbletea.md](022-bump-bubbletea.md)

## Plan

1. `go get github.com/charmbracelet/bubbles@v1.0.0 && go mod tidy`
2. `make test` to verify no regressions

## Files Modified

- `go.mod` — bubbles v0.21.0 to v1.0.0
- `go.sum` — updated checksums

## Verification

- `make test` passes
- No source code changes required
