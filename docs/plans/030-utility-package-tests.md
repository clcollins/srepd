# 030 - Utility Package Test Coverage

## Context

The `pkg/deprecation` and `pkg/rand` utility packages both have 0% test
coverage. These are small, self-contained packages with pure functions that
are straightforward to test. Adding coverage here improves overall project
test health and prevents regressions in deprecation key checks and random
string generation.

Predecessor: [018-bump-cobra-config-tests.md](018-bump-cobra-config-tests.md)
(similar test-addition pattern).

## Plan

1. Read `pkg/deprecation/deprecation.go` and `pkg/rand/rand.go` to
   understand the API surface.
2. Create `pkg/deprecation/deprecation_test.go` with tests for:
   - Known deprecated keys ("shell", "silentuser") return true
   - Unknown keys ("token") return false
   - Empty string returns false
3. Create `pkg/rand/rand_test.go` with tests for:
   - `StringWithCharset` produces correct length output
   - `StringWithCharset` output contains only charset characters
   - `StringWithCharset` with zero length returns empty string
   - `String` produces correct length output
   - `String` output contains only uppercase alphanumeric characters
   - `ID` output starts with the given prefix
   - `ID` output has correct total length (prefix + 13)
4. Run `make test-all` to verify all checks pass.
5. Run `go vet ./...` and `gofmt -s -l cmd pkg` for additional validation.
6. Create this plan document.
7. Commit, push, and open PR against main.

## Files Modified

- `pkg/deprecation/deprecation_test.go` -- new test file (4 tests)
- `pkg/rand/rand_test.go` -- new test file (7 tests)
- `docs/plans/030-utility-package-tests.md` -- this plan document

## Verification

- `make test` passes with all 11 new tests green
- `make fmt-check` clean (no formatting drift)
- `make vet` clean (no issues)
- `gofmt -s -l cmd pkg` clean (no output)
- `golangci-lint run` reports 0 issues on the new files
- `make lint` skipped due to pre-existing BIN_DIR path mismatch in
  worktree environment; CI will run this check
