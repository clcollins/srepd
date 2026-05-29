# 058: Fix Codecov CI Integration

## Problem

The Codecov upload step in CI was broken in three ways:

1. **Outdated action version**: `codecov/codecov-action@v4` should be `@v5`.
2. **Token passed incorrectly**: v5 expects the token under `with:`, not `env:`.
3. **Coverage file deleted before upload**: `make coverage` removes `coverage.out`
   at the end of its run, so the subsequent upload step had no file to upload.

## Changes

### `.github/workflows/go-ci.yml`

- Replaced `make coverage` with a direct `go test` command that generates
  `coverage.out` without cleaning it up. The `make coverage` target remains
  useful for local development (it prints a summary and cleans up after itself).
- Upgraded `codecov/codecov-action` from `v4` to `v5`.
- Moved `CODECOV_TOKEN` from `env:` to `with: token:` as required by v5.

## Validation

- `go test ./... -count=1` passes locally.
- CI workflow syntax validated by review.
