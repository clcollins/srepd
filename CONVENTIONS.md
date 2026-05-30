# Project Conventions

## Language -- Go

- Language: Go
- Go version: 1.26.3 (from go.mod)
- Module path: `github.com/clcollins/srepd`
- Entry point: `main.go` using cobra + viper
- Linter: golangci-lint
- Test framework: `go test` with `testify` for assertions
- Build: goreleaser for releases, `go build` for dev installs

### Code Organization

```text
cmd/              CLI entry point (cobra root command + config subcommand)
pkg/pd/           PagerDuty API client wrapper and mock
pkg/tui/          Bubble Tea TUI (model, views, commands, key bindings)
pkg/launcher/     Terminal launcher for cluster investigation
pkg/deprecation/  Configuration deprecation utilities
pkg/rand/         Random ID generation
docs/plans/       Plan documents (one per PR)
```

### Go Style

- Follow standard Go conventions (Effective Go, Go Code Review
  Comments)
- Error handling: return errors, never panic in library code
- Context: pass `context.Context` as first parameter where applicable
- All PagerDuty API calls must use `context.WithTimeout`
- Type assertions must use comma-ok pattern (never bare assertions)
- Naming: use descriptive names; avoid single-letter variables outside
  loop indices
- Test-driven development: write failing tests before implementation

## Makefile Standards

- All targets `.PHONY`
- Must include `build`, `test`, `lint`, `clean`, `fmt`, `coverage`,
  `vet`, and `test-all` targets
- `test-all` runs all checks: `fmt-check`, `vet`, `lint`, `test`
- Each target is independently callable
- `test` runs unit tests only (no lint dependency)
- `fmt-check` exits non-zero if code is unformatted (CI-friendly)
- Variables for configurable values (Go binary path, lint version)
- Build output: goreleaser for releases, `/tmp/` or `GOPATH/bin` for
  dev installs

## CI Testing

### GitHub Actions

- CI runs on push to `main` and `srepd/**` branches, and on pull
  requests to `main`
- Each check runs as a separate parallel job calling
  `make <target>` directly
- Go version in CI matches `go.mod` version (extracted automatically)

### Local vs Remote Execution

- Locally: `make test-all` runs all checks
- Remotely: GitHub Actions runs each `make` target in separate
  parallel jobs
- Both paths use the same Makefile targets, ensuring identical behavior

### Required Checks

| Check | Tool | Make Target |
|-------|------|-------------|
| Format | gofmt -s | `make fmt-check` |
| Go vet | go vet | `make vet` |
| Lint | golangci-lint | `make lint` |
| Race detection | go test -race | `make test-race` |
| Unit tests | go test | `make test` |
| Coverage | go test -coverprofile | `make coverage` |
| Coverage threshold | go test + threshold check | `make test-coverage-threshold` |
| Vulnerability scan | govulncheck | `make test-vuln` |
| Build | go build | `make build` |

## Linting

- Fix all lint issues rather than suppressing rules, unless there is a
  documented reason
- Linter configuration: `.golangci.yml` at repo root (when created)

## Platform

- Primary platform: Linux (Fedora/RHEL)
- Container engine: podman (never docker)
- Build targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- Terminal support: gnome-terminal, ptyxis, konsole, kitty, alacritty,
  wezterm, foot, tmux

## Documentation

### Plan Documents

- Every PR must include a plan document in `docs/plans/`
- A PR without a plan document is not reviewable
- Filenames use incrementing numbers with descriptive slug:
  `###-slug.md` (e.g., `001-fix-mapstructure-vulnerability.md`,
  `002-project-docs.md`)
- Numbers are sequential creation order, not PR or issue numbers
- Plans must include: Context, Plan/Solution, Files Modified,
  Verification sections
- Plans must consider lessons learned from prior `docs/plans/`
  entries and reference predecessors when relevant
- Superseded plans are preserved with a note at the top pointing
  to the replacement
- CI enforces plan document presence for every PR
- When a PR fixes issues introduced by a previous PR, the fixing
  PR's plan document must include a "Lessons Learned" section
  describing: what went wrong, why it happened, and how to prevent
  it in the future
- The fixing PR should also update the original plan document's
  "Lessons Learned" section with a cross-reference to the fix

## Version Control

- Feature branches only; never commit directly to main
- Branch naming: `srepd/<description>` (e.g., `srepd/p0-fix-panic`)
- Concise commit messages with descriptive body
- Attribution trailers required for AI-assisted work
- No force-push without explicit user approval
- Each improvement, fix, or feature gets its own branch and PR
