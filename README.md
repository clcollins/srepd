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
* Acknowledge, re-escalate, silence, and merge incidents with confirmation prompts
* Open SOP/runbook links and incidents directly from alerts
* Log into clusters via ocm-container or ocm backplane with multi-cluster selection
* Add notes, auto-refresh with selection preservation, auto-acknowledge when on-call
* PagerDuty environment variables passed automatically to terminal sessions
* OCM integration: cluster enrichment with display names, service logs, limited support history
* 6-tab incident viewer: Details, Alerts, Notes, Cluster, SLs, LS History
* Auto-update notification and `srepd update` self-update command

## Installation

```bash
make install        # install to $GOPATH/bin
# or
go install .        # standard go install
```

## Commands

| Command | Description |
|---------|-------------|
| `srepd` | Start the TUI |
| `srepd update` | Update to the latest release in place |
| `srepd --version` | Print version and git SHA |
| `srepd --dev` | Run with fixture data (no PD connection) |

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
| `chord_prefix` | `string` | `ctrl+x` | Prefix key for chord commands |

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

## OCM Integration

SREPD enriches PagerDuty incident data with cluster details from the OpenShift Cluster Manager (OCM) API. On startup, it connects to the production OCM API using tokens from `~/.config/ocm/ocm.json`. If tokens are expired, a browser window opens for auth code login.

Enriched data includes:
* **Cluster display names** replace PD service names in the incident table (e.g., `mycluster.abc1.p1.example.org` instead of `osd-mycluster.abc1.p1.example.org-hive-cluster`)
* **Impacted Clusters** section on the Details tab lists all clusters in the incident
* **Cluster tab** shows OCM cluster details: name, ID, state, region, provider, version, CCS, Hypershift
* **Service Logs tab** shows recent service logs per cluster
* **Limited Support History tab** shows LS reasons per cluster
* Multi-cluster incidents show `(+N)` in the service column

OCM features are optional — if OCM is not configured, the remaining TUI functions normally.

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
| `ctrl+a` | Toggle auto-acknowledge | `ctrl+l` | View debug log |
| `ctrl+q`/`ctrl+c` | Quit | `1`-`9` | Select cluster |
| `i`/`:` | Ask Claude | `m` | Merge incident |
| `ctrl+x` + key | Chord commands | `ctrl+x ?` | Show chord help |
| `Tab`/`Shift+Tab` | Switch tabs (incident view) | `↑`/`↓` | Scroll within tab |

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

MIT License. See [LICENSE](LICENSE) for details.

### A Note on AI Contributions

Portions of this codebase were developed with the assistance of AI tools, including Claude by Anthropic. To the extent that AI-generated contributions are subject to copyright and capable of being licensed under applicable law, such contributions are licensed under the same MIT License terms stated above. The legal status of AI-generated code remains an evolving area of law, and this notice is provided in the interest of transparency. All AI-assisted contributions have been reviewed and approved by the project maintainers.
