# AGENTS.md

## Project Overview

SREPD is a PagerDuty TUI for SREs built with Go and the Bubble Tea
framework. It manages incidents, displays alerts/notes, and launches
terminal windows for cluster investigation via ocm-container or
direct `ocm backplane` login.

## Build and Test Commands

| Command | Purpose |
|---------|---------|
| `make build` | Build via goreleaser snapshot |
| `make install` | Install to `$GOPATH/bin` |
| `make test` | Run unit tests (`go test ./...`) |
| `make lint` | Run golangci-lint |
| `make vet` | Run `go vet ./...` |
| `make fmt` | Format code with `gofmt -s` |
| `make fmt-check` | Check formatting (CI-friendly, non-zero on diff) |
| `make coverage` | Generate coverage report |
| `make clean` | Remove build artifacts |
| `make tidy` | Run `go mod tidy` |
| `make plan-check` | Verify plan document exists for this branch |
| `make readme-check` | Ensure README updated when config/keys/flags change |
| `make test-race` | Run tests with race detector |
| `make test-vuln` | Check for known vulnerabilities |
| `make test-coverage-threshold` | Enforce minimum coverage threshold (55%) |
| `make test-all` | Run all checks: fmt-check, vet, lint, test, test-race |

Pass extra test flags via `TESTOPTS`, e.g.:
`make test TESTOPTS="-run TestFoo"`

## Build and Versioning

- `make build` runs `goreleaser build --snapshot --clean --single-target`
- Goreleaser sets `Version` and `GitSHA` via `-ldflags` at build time
- **Snapshot builds** (local): `Version` = git short commit hash (e.g., `51e405a`),
  `GitSHA` = same hash. Shown in footer as `51e405a - 51e405a`
- **Release builds** (CI/tagged): `Version` = semver from tag (e.g., `1.4.0`),
  `GitSHA` = commit hash
- **CRITICAL**: Goreleaser bakes the **last committed** hash into the binary.
  Uncommitted changes are NOT reflected in the version string. You MUST
  commit before building if you want the binary to show a new version.
- The update banner triggers when `isNewerVersion(current, latest)` returns
  true. For snapshot builds (no dot in version string), it always returns true.
- Config: `.goreleaser.yaml` ldflags section

## Architecture

- **Pattern**: Bubble Tea Model-View-Update (MVU)
- **Async**: All PagerDuty API calls run as `tea.Cmd` closures
  returning `tea.Msg` values. The Update loop is single-threaded.
- **PD client**: Abstracted behind `PagerDutyClientInterface`
  (in `pkg/pd/pd.go`). Mock at `pkg/pd/mock.go`.
- **Launcher**: Builds terminal commands with variable substitution
  (`%%CLUSTER_ID%%`, `%%INCIDENT_ID%%`). Supports multiple terminal
  emulators via profiles.
- **Config**: Viper-managed YAML at `~/.config/srepd/srepd.yaml`
  with `SREPD_` env var prefix.

## Test Patterns

- Table-driven tests with `testing.T` subtests
- Assertions via `github.com/stretchr/testify/assert`
- Mock PD client in `pkg/pd/mock.go` with convention-based errors
  (ID = "err" triggers error responses)
- Test files: `*_test.go` alongside source files
- TDD workflow: write failing test, implement, verify green,
  run `make test-all`

## Key Invariants

- Never panic in library code; return errors
- All PagerDuty API calls must use `context.WithTimeout`
- Type assertions must use the comma-ok pattern
- Tests before code (TDD)
- Each PR must pass all CI checks
- **Run the FULL local CI suite before every commit.** Run
  all checks in parallel before pushing. Do not push code
  that fails any check.
- When fixing a bug introduced by a previous PR, document the
  lesson in both plan docs (the new fix PR's plan and the
  original PR's plan) describing what went wrong and how to
  prevent it

## Pre-Commit Checks (MANDATORY)

Run ALL of these in parallel before every commit/push:

```bash
# Run all 7 in parallel:
gofmt -s -l cmd pkg                              # fmt-check
go vet ./...                                     # vet
go test ./... -count=1                           # unit tests
CGO_ENABLED=1 go test -race ./... -count=1       # race detection
golangci-lint run --timeout 5m                   # lint
ls docs/plans/*.md                               # plan doc exists
# If keymap.go/config.go/root.go/commands.go changed:
git diff origin/main --name-only | grep README   # readme updated
```

**Every check must pass. Fix failures before committing.**

## PR Workflow

1. Create feature branch: `srepd/<description>`
2. Write failing tests
3. Implement minimum code to pass tests
4. Run the full pre-commit checks (see above) — **all must pass**
5. Fix any failures before proceeding
6. **Create a plan document** in `docs/plans/` (see below)
7. **Update README.md** if `keymap.go`, `root.go`, or `commands.go` changed
8. Push and create PR against `main`
9. CI runs all checks via `make` targets
10. Review, approve, merge

## CI Requirements for PRs

Every non-Dependabot PR must pass these CI checks:

| Check | Make Target | What It Verifies |
|-------|-------------|------------------|
| Format Check | `make fmt-check` | `gofmt -s` formatting |
| Go Vet | `make vet` | `go vet ./...` |
| Lint | `make lint` | `golangci-lint` |
| Unit Tests | `make test` | `go test ./...` |
| Race Detection | `make test-race` | `go test -race ./...` |
| **Plan Document** | `make plan-check` | A new/modified `docs/plans/*.md` file exists in the diff |
| **README Update** | `make readme-check` | If `keymap.go`, `root.go`, or `commands.go` changed, `README.md` must also be in the diff. Add label `skip-readme` to bypass when the change is purely internal (no user-facing config/flag/keybinding changes) |

### Plan Document

Every PR must include a plan document at `docs/plans/<number>-<description>.md`.
Use the next available number. The plan should describe the problem,
approach, and key design decisions. See existing plans for format examples.

## Key Files

| File | Purpose |
|------|---------|
| `pkg/pd/pd.go` | PagerDuty API wrapper (all API calls) |
| `pkg/pd/mock.go` | Mock PD client for testing |
| `pkg/tui/model.go` | TUI model (state) |
| `pkg/tui/tui.go` | Update loop and message handlers |
| `pkg/tui/commands.go` | Async commands (API calls, login) |
| `pkg/tui/views.go` | View rendering and templates |
| `pkg/tui/keymap.go` | Key bindings |
| `pkg/tui/msgHandlers.go` | Focus-mode key dispatch |
| `pkg/launcher/launcher.go` | Terminal command builder |
| `cmd/root.go` | CLI entry point |
| `cmd/config.go` | Config validation |
