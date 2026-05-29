# Plan 020: Remaining Go Module Updates

## Context

Scan and update all outdated Go module dependencies beyond the
Dependabot PRs. Conservative approach: only patch/minor updates
with no breaking changes. Skip charmbracelet major versions and
modules requiring Go 1.25+.

Predecessor: [019-bump-viper.md](019-bump-viper.md)

## Modules Updated

### Direct Dependencies

| Module | From | To |
|--------|------|----|
| golang.org/x/time | v0.9.0 | v0.11.0 |

### Indirect Dependencies

| Module | From | To |
|--------|------|----|
| charmbracelet/colorprofile | v0.3.1 | v0.4.1 |
| charmbracelet/x/ansi | v0.8.0 | v0.11.7 |
| charmbracelet/x/cellbuf | v0.0.13 | v0.0.15 |
| charmbracelet/x/term | v0.2.1 | v0.2.2 |
| dlclark/regexp2 | v1.11.5 | v1.12.0 |
| fsnotify/fsnotify | v1.9.0 | v1.10.1 |
| google/go-querystring | v1.1.0 | v1.2.0 |
| lucasb-eyer/go-colorful | v1.2.0 | v1.4.0 |
| mattn/go-isatty | v0.0.20 | v0.0.22 |
| mattn/go-runewidth | v0.0.16 | v0.0.23 |
| pelletier/go-toml/v2 | v2.2.4 | v2.3.1 |
| yuin/goldmark | v1.7.11 | v1.8.2 |

## Modules Skipped

- charmbracelet/bubbles v1.0.0 (MAJOR - dedicated migration)
- charmbracelet/glamour v1.0.0 (MAJOR - dedicated migration)
- charmbracelet/log v1.0.0 (MAJOR - dedicated migration)
- charmbracelet/bubbletea v1.3.10 (handled in D6)
- golang.org/x/* latest (require Go 1.25+)
- alecthomas/chroma/v2 v2.26.1 (requires Go 1.25+)

## Files Modified

- `go.mod` — updated dependency versions
- `go.sum` — updated checksums

## Verification

- `make test` passes with zero regressions
- `go vet ./...` clean
- `make fmt-check` clean
