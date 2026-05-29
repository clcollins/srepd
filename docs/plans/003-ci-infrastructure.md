# Restructure Makefile and GitHub Actions CI for granular checks

> Retroactive plan document for PR #143, created after merge.

## Context

The existing CI only ran `make lint` and `make test`, where `test`
depended on `lint` and ran `go test`. There was no standalone `go vet`,
no `go fmt` check, no coverage upload, and the `test` target conflated
linting with testing. Every subsequent PR needed comprehensive,
granular CI gates with local/CI parity.

Predecessor: [002-project-docs.md](002-project-docs.md)

## Plan

1. Restructure Makefile: make `test` independent (remove lint
   dependency), add `fmt-check` (CI-friendly format check) and
   `test-all` (runs all checks)
2. Rewrite GitHub Actions with parallel `fmt`/`vet`/`lint` jobs,
   then `test`, `coverage`, and `build`
3. Each CI job calls `make <target>` directly for local/CI parity
4. Add `srepd/**` branch pattern to CI triggers
5. Apply `gofmt -s` formatting to all existing source files

## Files Modified

- `Makefile` — added `fmt-check`, `test-all`; `test` independent
- `.github/workflows/go-ci.yml` — rewritten with parallel jobs
- `pkg/tui/*.go` — gofmt formatting (8 files)

## Verification

- `make test-all` passes locally
- All GitHub Actions jobs pass on the PR
- `make help` shows all new targets
