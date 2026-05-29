# Bump viper from 1.20.1 to 1.21.0

## Context

Dependabot PR #131 flagged viper 1.20.1 as outdated. Viper 1.21.0
swaps the YAML library internally (gopkg.in/yaml.v3 to
go.yaml.in/yaml/v3) and adds pflag slice support. The public API
is unchanged.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Run `make test` to establish baseline
2. Run `go get github.com/spf13/viper@v1.21.0 && go mod tidy`
3. Run `make test-all` to verify no regressions
4. Close Dependabot PR #131

## Files Modified

- `go.mod` — viper v1.20.1 to v1.21.0, transitive bumps
- `go.sum` — updated checksums

## Verification

- `make test` passes with zero regressions
- `go vet ./...` clean
