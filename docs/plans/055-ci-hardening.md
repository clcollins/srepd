# 055: CI Hardening

## Context

Issue #204. The CI pipeline covers basic checks (fmt, vet, lint, test,
coverage, build) but lacks race detection, vulnerability scanning, and
coverage threshold enforcement. The golangci-lint configuration relies
on implicit defaults which can change between versions.

Predecessors: 003-ci-infrastructure.md established the initial CI
structure. This plan extends it with additional safety checks.

## Plan

1. Add `.golangci.yml` with explicit linter configuration (bodyclose,
   errcheck, gosec, govet, ineffassign, staticcheck, unused).
2. Add three new Makefile targets: `test-race` (race detector),
   `test-vuln` (govulncheck), `test-coverage-threshold` (55% floor).
3. Update `test-all` to include `test-race`.
4. Add CI jobs: `race` (parallel, no dependencies) and `vuln` (after
   test). Add coverage threshold step to existing coverage job.
5. Update AGENTS.md to document new targets.

## Files Modified

| File | Action |
|------|--------|
| `.golangci.yml` | Created -- explicit linter config |
| `Makefile` | Added test-race, test-vuln, test-coverage-threshold; updated test-all |
| `.github/workflows/go-ci.yml` | Added race and vuln jobs; added threshold step to coverage |
| `AGENTS.md` | Documented new make targets |
| `docs/plans/055-ci-hardening.md` | This plan document |

## Verification

- `go test ./... -count=1` passes (no test regressions)
- `make lint` passes with new .golangci.yml
- `make test-race` passes (no data races)
- `make test-vuln` passes (no known vulnerabilities)
- `make test-coverage-threshold` passes (above 55%)
- CI workflow has 9 jobs: plan-doc, fmt, vet, lint, race, test,
  coverage+threshold, vuln, build
