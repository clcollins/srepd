# Add Codecov config and local patch coverage check

## Context

Codecov posts informational status checks on PRs but nothing
enforces coverage on changed code. Need codecov.yml for remote
enforcement and a local approximation for pre-push checks.

## Plan

1. Add `codecov.yml` with project target 55% and patch target 70%
2. Add `make test-coverage-patch` for local approximation
3. The local check identifies changed Go files, measures their
   package coverage, and warns if below 70%

## Files

- `codecov.yml` — new, Codecov configuration
- `Makefile` — add test-coverage-patch target

## Verification

- Codecov status checks appear on PRs with pass/fail
- `make test-coverage-patch` runs locally and reports
