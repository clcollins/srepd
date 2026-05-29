# Bump bubbletea from 1.3.5 to 1.3.10

## Context

Bubbletea is the core TUI framework. Updates 1.3.5 to 1.3.10 are
all patch releases with zero breaking API changes. Key fixes:
Sequence/Batch compaction, nested panic recovery, windowSizeMsg
event loop fix, batch command channel execution.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Run `make test` baseline
2. `go get github.com/charmbracelet/bubbletea@v1.3.10 && go mod tidy`
3. Run `make test-all`
4. Close Dependabot PR #132

## Files Modified

- `go.mod` — bubbletea v1.3.5 to v1.3.10
- `go.sum` — updated checksums

## Verification

- `make test` passes with zero regressions
- `go vet` clean
