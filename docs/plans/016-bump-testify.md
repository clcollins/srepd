# 016 - Bump testify 1.10.0 to 1.11.1

Retroactive plan document.

## Context

The `github.com/stretchr/testify` dependency is at v1.10.0 and has an available
update to v1.11.1. Dependabot PR #127 proposed this upgrade but is superseded by
this manual PR to ensure a clean, verified bump with plan documentation.

Notable version history:
- v1.11.0 introduced a breaking change in mock argument matching (strict
  `IsType` assertions).
- v1.11.1 reverted the breaking mock behavior, making it safe to upgrade.
- srepd only uses basic testify assertions (`assert.Equal`, `assert.NoError`,
  etc.) and does not use `mock.Mock`, so neither the breaking change nor the
  revert affects this project. This is a zero-risk upgrade.

Predecessor: [fix-mapstructure-vulnerability.md](fix-mapstructure-vulnerability.md)
(similar dependency bump pattern).

## Plan

1. Establish baseline: run `make test` to confirm all tests pass at v1.10.0.
2. Run `go get github.com/stretchr/testify@v1.11.1` to bump the dependency.
3. Run `go mod tidy` to clean up the module graph.
4. Run `make test-all` (fmt-check, vet, lint, test) to verify no regressions.
5. Run `go vet ./...` as an additional static analysis check.
6. Create this plan document.
7. Commit, push, and open PR against main.

## Files Modified

- `go.mod` -- version bump from v1.10.0 to v1.11.1
- `go.sum` -- updated checksums for testify v1.11.1
- `docs/plans/016-bump-testify.md` -- this plan document

## Verification

- `go test ./...` passes with no failures or regressions
- `go vet ./...` clean (no issues)
- `make fmt-check` clean (no formatting drift)
- `make vet` clean
- `make test` passes (all 4 test packages green)
- `make lint` skipped (golangci-lint not installed in worktree environment;
  CI will run this check)
