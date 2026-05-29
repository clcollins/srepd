![srepd](img/srepd.jpg)

# SREPD

[![Go Version](https://img.shields.io/github/go-mod/go-version/clcollins/srepd)](https://golang.org)
[![Build Status](https://github.com/clcollins/srepd/actions/workflows/go-ci.yml/badge.svg)](https://github.com/clcollins/srepd/actions/workflows/go-ci.yml)
[![codecov](https://codecov.io/gh/clcollins/srepd/graph/badge.svg)](https://codecov.io/gh/clcollins/srepd)
[![Go Report Card](https://goreportcard.com/badge/github.com/clcollins/srepd)](https://goreportcard.com/report/github.com/clcollins/srepd)
[![Go Reference](https://pkg.go.dev/badge/github.com/clcollins/srepd.svg)](https://pkg.go.dev/github.com/clcollins/srepd)
[![GitHub Release](https://img.shields.io/github/v/release/clcollins/srepd)](https://github.com/clcollins/srepd/releases)
[![License](https://img.shields.io/github/license/clcollins/srepd)](https://github.com/clcollins/srepd/blob/main/LICENSE)

A PagerDuty terminal user interface focused on common SRE tasks.

## Features

* View and manage PagerDuty incidents with team and individual views
* Acknowledge, re-escalate, and silence incidents with confirmation prompts
* Open SOP/runbook links and incidents directly from alerts
* Log into clusters via ocm-container or ocm backplane with multi-cluster selection
* Add notes, auto-refresh with selection preservation, auto-acknowledge when on-call
* PagerDuty environment variables passed automatically to terminal sessions

## Installation

```bash
make install        # install to $GOPATH/bin
# or
go install .        # standard go install
```

## Configuration

SREPD reads `~/.config/srepd/srepd.yaml` and supports `SREPD_` environment variable prefix. Generate a sample config with `srepd config --create` or validate with `srepd config --validate`.

### Required

| Key | Type | Description |
|-----|------|-------------|
| `token` | `string` | PagerDuty API OAuth token |
| `teams` | `[]string` | PagerDuty team IDs |
| `service_escalation_policies` | `map[string]string` | Must contain `DEFAULT` and `SILENT_DEFAULT` keys |

### Optional

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `editor` | `string` | `vim` | Editor for incident notes |
| `terminal` | `string` | `gnome-terminal` | Terminal emulator for cluster login |
| `cluster_login_command` | `string` | `ocm backplane login %%CLUSTER_ID%%` | Cluster login command |
| `ignoredusers` | `[]string` | (none) | PagerDuty user IDs to exclude |
| `toolbox_mode` | `string` | `auto` | Toolbox detection: `auto`, `true`, or `false` |

### Example

```yaml
token: <PagerDuty API token>
teams:
  - <team ID>
service_escalation_policies:
  DEFAULT: P123456
  SILENT_DEFAULT: P654321
terminal: gnome-terminal
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%
toolbox_mode: auto
```

Use `%%CLUSTER_ID%%` and `%%INCIDENT_ID%%` as placeholders in `terminal` or `cluster_login_command` for dynamic substitution at launch time. If `%%CLUSTER_ID%%` is not present in `cluster_login_command`, the cluster ID is appended automatically.

## Terminal Support

Terminal profiles are auto-detected from the executable name or Flatpak app ID. Set only the terminal name in your config; the correct argument style is handled automatically.

Supported terminals: gnome-terminal, ptyxis, wezterm, blackbox, tmux, konsole, alacritty, ghostty, terminator, kitty, foot, contour, iterm2, macOS Terminal

Flatpak-installed terminals are also supported using their application ID (e.g., `org.kde.konsole`).

When running inside a Fedora Toolbox, terminal commands are automatically prefixed with `flatpak-spawn --host` (controlled by `toolbox_mode`).

## Key Bindings

Press `h` to toggle the help overlay inside srepd.

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `j`/`k` | Move down/up | `a` | Acknowledge |
| `g`/`G` | Jump to top/bottom | `n` | Add note |
| `Enter` | View incident | `l` | Login to cluster |
| `Esc` | Go back | `o` | Open in browser |
| `h` | Toggle help | `s` | Open SOP link |
| `t` | Toggle team/individual | `ctrl+s` | Silence |
| `r` | Refresh | `ctrl+e` | Re-escalate |
| `ctrl+r` | Toggle auto-refresh | `u` | Toggle urgency filter |
| `ctrl+a` | Toggle auto-acknowledge | `ctrl+l` | Toggle action log |
| `ctrl+q`/`ctrl+c` | Quit | `1`-`9` | Select cluster |
| `ctrl+x` + key | Chord commands | `ctrl+x ?` | Show chord help |

Chord commands use a configurable prefix (default `ctrl+x`) followed by a second key. Set `chord_prefix` in config to change.

## Environment Variables

When using ocm-container, PagerDuty context is passed automatically as environment variables:

| Variable | Description |
|----------|-------------|
| `PAGERDUTY_INCIDENT_ID` | Incident ID |
| `PAGERDUTY_INCIDENT_TITLE` | Incident title (sanitized) |
| `PAGERDUTY_INCIDENT_URL` | Direct link to the incident |
| `PAGERDUTY_INCIDENT_SERVICE` | Service name |
| `PAGERDUTY_INCIDENT_URGENCY` | `high` or `low` |
| `PAGERDUTY_INCIDENT_STATUS` | `triggered`, `acknowledged`, or `resolved` |
| `PAGERDUTY_CLUSTER_ID` | Selected cluster ID |
| `PAGERDUTY_ALERT_COUNT` | Number of matching alerts |
| `PAGERDUTY_ALERT_NAMES` | Comma-separated alert names |
| `PAGERDUTY_ALERT_LINKS` | Comma-separated SOP/runbook links |
| `PAGERDUTY_NOTES_EXIST` | `true` or `false` |
| `PAGERDUTY_NOTE_COUNT` | Number of notes |
| `PAGERDUTY_CLAUDE_AVAILABLE` | `true` when Claude Code is detected on PATH |
| `REASON` | Incident URL (for compliance/audit) |

For non-ocm-container terminals, environment variables are set on the process directly or passed via `flatpak-spawn --env=` when in toolbox mode.

## Development

| Command | Purpose |
|---------|---------|
| `make build` | Build via goreleaser snapshot |
| `make install` | Install to `$GOPATH/bin` |
| `make test` | Run unit tests |
| `make lint` | Run golangci-lint |
| `make vet` | Run `go vet` |
| `make fmt` | Format code |
| `make fmt-check` | Check formatting (CI-friendly) |
| `make coverage` | Generate coverage report |
| `make test-all` | Run all checks: fmt-check, vet, lint, test |
| `make plan-check` | Verify plan document exists for branch |
| `make clean` | Remove build artifacts |
| `make help` | Show all available targets |

PRs follow TDD workflow and require a plan document in `docs/plans/`. Run `make test-all` before pushing.

## License

MIT License. See [LICENSE](LICENSE) for details. Portions of this codebase were developed with the assistance of AI tools. See the [AI-Assisted Contributions note](LICENSE) in the license file for details.
