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

## Pre-Commit Checks (MANDATORY)

Run ALL of these in parallel before every commit/push:

```bash
# Run all 6 in parallel:
gofmt -s -l cmd pkg                              # fmt-check
go vet ./...                                     # vet
go test ./... -count=1                           # unit tests
CGO_ENABLED=1 go test -race ./... -count=1       # race detection
ls docs/plans/*.md                               # plan doc exists
# If keymap.go/config.go/root.go/commands.go changed:
git diff origin/main --name-only | grep README   # readme updated
```

If `golangci-lint` is installed, also run:
```bash
golangci-lint run --timeout 5m                   # lint
```

**Every check must pass. Fix failures before committing.**

## PR Workflow

1. Create feature branch: `srepd/<description>`
2. Write failing tests
3. Implement minimum code to pass tests
4. Run the full pre-commit checks (see above) — **all must pass**
5. Fix any failures before proceeding
6. Push and create PR against `main`
7. CI runs all checks via `make` targets
8. Review, approve, merge

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
