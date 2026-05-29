# Plan 025: Migrate glamour from v0.10.0 to v2.0.0

## Context

The charmbracelet/glamour library for terminal markdown rendering has been
migrated from its original `github.com/charmbracelet/glamour` module path to
`charm.land/glamour/v2`. Notably, there was never a v1.0.0 release -- the
project jumped directly from v0.x to v2.0.0 as part of the Charm ecosystem's
move to the `charm.land` domain.

This migration was blocked until Go 1.25.8+ was available, as glamour v2.0.0
requires it. That prerequisite was satisfied by PR #169 (Go 1.26.3 upgrade).

Key changes in glamour v2.0.0:
- New module path: `charm.land/glamour/v2` (was `github.com/charmbracelet/glamour`)
- `WithAutoStyle()` removed: dark style is now the default, making the option unnecessary
- New transitive dependency on `charm.land/lipgloss/v2` and `charmbracelet/ultraviolet`
- `muesli/reflow` removed as a transitive dependency (functionality absorbed upstream)

## Plan

1. Update import in `pkg/tui/model.go` from `github.com/charmbracelet/glamour` to `charm.land/glamour/v2`
2. Remove `glamour.WithAutoStyle()` from the `NewTermRenderer()` constructor (dark is default in v2)
3. Run `go get charm.land/glamour/v2@latest` to add the new dependency
4. Run `go mod tidy` to remove the old dependency and clean up transitive deps
5. Run `gofmt -s` to fix import ordering (charm.land sorts before github.com)
6. Verify with `go test ./...`, `go vet ./...`, and `gofmt -s -l`

## Files Modified

- `pkg/tui/model.go` -- import path and WithAutoStyle removal
- `go.mod` -- dependency update
- `go.sum` -- dependency checksums

## Verification

- All 7 packages pass tests (`go test ./... -v -count=1`)
- No vet issues (`go vet ./...`)
- No formatting issues (`gofmt -s -l cmd pkg`)
- CI checks pass on PR #173
