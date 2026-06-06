# Contributing to SREPD

This guide covers what CI checks run on every pull request, how to run
them locally, and the development workflow this project follows.

## Development Environment Setup

### Prerequisites

| Tool | Version | How to install |
|------|---------|----------------|
| Go | Match `go.mod` (currently 1.26.x) | [go.dev/dl](https://go.dev/dl/) |
| golangci-lint | v2.12.2 | `make getlint` (auto-installs) |
| goreleaser | v2.8.2 | `go install github.com/goreleaser/goreleaser/v2@v2.8.2` |
| govulncheck | latest | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
| bc | (system) | Package manager (`dnf install bc`, `apt install bc`) |

CI determines the Go version automatically from `go.mod`, so keep your
local Go version in sync with what `go.mod` declares.

### Clone and verify

```bash
git clone https://github.com/clcollins/srepd.git
cd srepd
make test-all
```

If `make test-all` passes, your environment is ready.

## CI Checks

Every pull request runs the GitHub Actions workflow defined in
`.github/workflows/go-ci.yml`. The table below lists every job, what it
does, and how to reproduce it locally.

### Jobs that run on every PR

| CI Job | What it checks | Make target / command | Depends on | Common failures |
|--------|---------------|----------------------|------------|-----------------|
| **Format Check** | Code is formatted with `gofmt -s` | `make fmt-check` | -- | Unformatted code. Fix with `make fmt`. |
| **Go Vet** | Common Go mistakes (printf args, struct tags, etc.) | `make vet` | -- | Mismatched printf verbs, unused variables, bad struct tags. |
| **Lint** | golangci-lint with the linters enabled in `.golangci.yml` | `make lint` | -- | Unchecked errors (`errcheck`), unclosed HTTP bodies (`bodyclose`), unused code (`unused`), ineffective assignments (`ineffassign`), staticcheck findings. Fix the code rather than suppressing the linter. |
| **Race Detection** | Data race detector via `go test -race` | `make test-race` | -- | Shared state accessed without synchronization. Requires `CGO_ENABLED=1`. |
| **Unit Tests** | All `*_test.go` tests | `make test` | fmt, vet, lint | Failing assertions, missing mocks, changed behavior without updated tests. |
| **Coverage** | Uploads coverage to Codecov and enforces a minimum threshold | `make test-coverage-threshold` | test | Overall coverage drops below 55%. Add tests for new or changed code. |
| **Vulnerability Scan** | `govulncheck` against known Go CVEs | `make test-vuln` | test | A dependency has a known vulnerability. Update it with `go get -u <module>` then `go mod tidy`. |
| **Build Check** | `go build -o /dev/null .` compiles successfully | `go build -o /dev/null .` | test | Compilation errors. |

### Jobs that run only on PRs (skipped for Dependabot)

| CI Job | What it checks | Make target / command | Common failures |
|--------|---------------|----------------------|-----------------|
| **Plan Document** | A file was added or modified in `docs/plans/*.md` relative to `origin/main` | `make plan-check` | No plan document in `docs/plans/`. Create one following the naming convention `NNN-slug.md` (see [Plan Documents](#plan-documents) below). |
| **README Update Check** | If `pkg/tui/keymap.go`, `cmd/root.go`, or `pkg/tui/commands.go` changed, `README.md` must also be updated | `make readme-check` | Changed a key-binding, CLI flag, or command without updating the README. Can be skipped by adding the `skip-readme` label to the PR. |

### Additional local-only checks

These are not separate CI jobs but are useful to run locally:

| Check | Make target | Purpose |
|-------|-------------|---------|
| Fixture sanitization | `make test-fixtures` | Ensures `testdata/fixtures/*.json` contains no real UUIDs, domains, or org names. |
| Patch coverage | `make test-coverage-patch` | Warns if changed packages have less than 70% coverage. |
| All-in-one | `make test-all` | Runs `fmt-check`, `vet`, `lint`, `test`, `test-race`, and `test-fixtures` together. |

## Running the Full Pre-Commit Suite

Run all checks before every commit. These can be run in parallel:

```bash
make fmt-check &
make vet &
make lint &
make test &
make test-race &
make test-fixtures &
make plan-check &
make readme-check &
wait
```

Or use the combined target that runs most of them:

```bash
make test-all
```

Note: `make test-all` runs `fmt-check`, `vet`, `lint`, `test`,
`test-race`, and `test-fixtures`. You still need to run `make plan-check`
and `make readme-check` separately if you want full CI parity.

Every check must pass. Fix failures before committing.

## Development Workflow

### Branch naming

All feature branches must follow the convention:

```
srepd/<description>
```

For example: `srepd/fix-panic-on-empty-alerts`,
`srepd/add-urgency-filter`.

### Test-Driven Development (TDD)

This project follows strict TDD:

1. **Write a failing test first.** Define the expected behavior in a
   `*_test.go` file before writing any implementation.
2. **Implement the minimum code** to make the test pass.
3. **Refactor** if needed, keeping tests green.
4. **Run `make test-all`** to confirm nothing else broke.

#### Test patterns used in this project

- Table-driven tests with `t.Run()` subtests
- Assertions via `github.com/stretchr/testify/assert`
- Mock PagerDuty client in `pkg/pd/mock.go` -- set ID to `"err"` to
  trigger error responses
- Test files live alongside source files (`foo.go` / `foo_test.go`)

Pass extra flags to test targets with `TESTOPTS`:

```bash
make test TESTOPTS="-run TestSpecificFunction"
```

### Plan Documents

Every PR must include a plan document in `docs/plans/`. CI will reject
PRs that do not add or modify a file in that directory.

- **Filename format**: `NNN-slug.md` where `NNN` is the next sequential
  number. Check existing files in `docs/plans/` to find the next number.
- **Required sections**: Context, Plan/Solution, Files Modified,
  Verification.
- **Lessons learned**: When fixing a bug introduced by a previous PR,
  add a "Lessons Learned" section to both the new plan and the original
  plan explaining what went wrong and how to prevent it.
- See `CONVENTIONS.md` for the full plan document specification.

### README update requirement

If your PR changes any of these files, you must also update `README.md`:

- `pkg/tui/keymap.go` (key bindings)
- `cmd/root.go` (CLI flags and entry point)
- `pkg/tui/commands.go` (commands)

If no README update is needed for your change, you can add the
`skip-readme` label to the PR to bypass this check.

### PR checklist

1. Create a feature branch: `srepd/<description>`
2. Write failing tests
3. Implement the change
4. Create a plan document in `docs/plans/`
5. Update `README.md` if key bindings, flags, or commands changed
6. Run `make test-all` -- all checks must pass
7. Run `make plan-check` and `make readme-check`
8. Commit and push
9. Open a PR against `main`

## Linter Configuration

The linter configuration lives in `.golangci.yml`. The enabled linters
are:

| Linter | What it catches |
|--------|----------------|
| `bodyclose` | HTTP response bodies not closed |
| `errcheck` | Unchecked error return values |
| `govet` | Go vet checks (printf args, struct tags, etc.) |
| `ineffassign` | Assignments to variables that are never read |
| `staticcheck` | Comprehensive static analysis |
| `unused` | Unused code (functions, variables, types) |

Fix lint issues in the code rather than suppressing rules, unless there
is a documented reason for the suppression.

## Key Invariants

- Never panic in library code; return errors.
- All PagerDuty API calls must use `context.WithTimeout`.
- Type assertions must use the comma-ok pattern (`v, ok := x.(T)`).
- Never commit real customer data. Test fixtures in
  `testdata/fixtures/` must use fake markers -- `make test-fixtures`
  enforces this.

## Build

For development:

```bash
make install          # installs to $GOPATH/bin
make build            # goreleaser snapshot build
make install-local    # copies snapshot build to ~/.local/bin
```

Binaries should never be built into the source tree. Use `make install`
(writes to `$GOPATH/bin`) or `make build` (writes to `dist/`).
