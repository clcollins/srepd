# Plan 102: CI and Makefile hygiene

**Branch:** `srepd/ci-makefile-hygiene`

## Problem

Several low-risk CI/build hygiene gaps:

1. **No `permissions:` block** in `.github/workflows/go-ci.yml` — every job inherited
   the repo-default `GITHUB_TOKEN` scope, broader than any job needs.
2. **`test-fixtures` was not a CI job** — the customer-data sanitization check
   (`Makefile` `test-fixtures`, scanning `testdata/fixtures/*.json` for real UUIDs and
   domains) ran only locally via `make test-all`. A contributor who skipped it could
   merge real customer data — the opposite of the "never commit customer data" rule.
3. **`codecov/codecov-action` pinned to a mutable `@v6` tag** — third-party action, a
   supply-chain risk (a prior version was compromised).
4. **Makefile `GOPATH` bug** — `awk -F: '{print $1}'`: Make expands `$1` to empty
   before awk runs, so awk printed `$0` (the whole line). Worked by accident only when
   `GOPATH` had no `:`.
5. **`make release` token line not `@`-silenced** — Make echoed the `jq`-token recipe
   line (the command, not the resolved value) to stdout.

## Solution

- `.github/workflows/go-ci.yml`:
  - Add top-level `permissions: contents: read`.
  - Add a `fixtures` job running `make test-fixtures` (mirrors the `vet` job; pure
    grep, no Go toolchain).
  - Pin `codecov/codecov-action` to the `v6` commit SHA
    (`fb8b3582c8e4def4969c97caa2f19720cb33a72f # v6`).
- `Makefile`:
  - `awk -F: '{print $1}'` → `$$1` so awk receives the real program.
  - `@`-prefix the `make release` `GITHUB_TOKEN=$$(jq …)` line.

## Files Modified

- `.github/workflows/go-ci.yml`
- `Makefile`

## Tests / Verification

No Go code changed. Verified:
- Workflow YAML parses (`yaml.safe_load`).
- `GOPATH := $(shell …)` now resolves to the first path element
  (`/home/…/pkgsets/go1.26.4/global`, not the whole line).
- `make test-fixtures` passes.
- `go build ./...`, `gofmt -s` clean; `make test-all` unaffected.

The new `fixtures` CI job will run on this PR itself, exercising the change.

## Lessons Learned

- In Make recipes, `$1` inside a `$(shell …)` is eaten by Make — always `$$1` for awk/
  shell positional args.
- Fixture-sanitization is a safety-critical check; it belongs in CI, not just local
  `make test-all`, since local steps can be skipped.
- Pin third-party actions to a commit SHA (with a version comment) to resist
  supply-chain tag mutation.
