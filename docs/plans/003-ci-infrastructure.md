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

## Lessons Learned

**GENUINE ERROR — push + pull_request triggers caused double CI runs**
(Fixed by: [021-fix-ci-double-trigger.md](021-fix-ci-double-trigger.md))

The workflow triggered on both `push` to `srepd/**` branches AND
`pull_request` to `main`. Every PR branch commit ran all CI jobs
twice — once on push and once on pull_request — doubling runner usage.

Why it wasn't caught: the CI was confirmed to "work" (jobs passed),
but nobody checked whether it was running redundantly. GitHub Actions'
dual-trigger behavior is a common pitfall.

Prevention: use only `pull_request` for PR validation, or add explicit
path/branch exclusions to prevent overlap between push and
pull_request triggers.

---

**GENUINE ERROR — Codecov upload silently failed due to three bugs**
(Fixed by: [058-fix-codecov-ci.md](058-fix-codecov-ci.md))

The Codecov integration had three compounding bugs: outdated action
version (v4 instead of v5), token passed via `env:` instead of `with:`
(required by v5), and `make coverage` deleted `coverage.out` before
the upload step could read it.

Why it wasn't caught: the upload step did not verify success, and
Codecov coverage was treated as optional rather than validated. All
three bugs contributed to silent failure — no error, just missing
coverage data.

Prevention: CI integrations that upload artifacts should include a
verification step confirming the upload succeeded. Test CI
integrations in a real CI run before merging, not just locally.
