# Plan 081: Add tests and refinements for config --create

## Context

PR #272 added a `createConfig()` function that makes `srepd config --create`
write the config file to disk. This PR builds on that contribution with test
coverage, shared constants, and a robustness improvement.

## Changes

1. Promote config path constants (`cfgFileName`, `cfgFileDir`) to package level
   so both `initConfig()` and `createConfig()` share a single source of truth.

2. Introduce a `configFS` interface for filesystem operations so `createConfig()`
   can be tested with a fully in-memory mock — no real filesystem access during
   tests.

3. Use `os.O_EXCL` flag for atomic file creation instead of a separate
   stat-then-write pattern.

4. Trim leading newline from `exampleConfig` when writing to disk so the file
   starts cleanly with the `#` comment line.

5. Add comprehensive tests for `createConfig()` using a mock filesystem.

## Verification

- `make test-all` passes
- `make test-coverage-threshold` stays above 55%
