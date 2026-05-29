![srepd](img/srepd.jpg)

# SREPD

[![Go Version](https://img.shields.io/github/go-mod/go-version/clcollins/srepd)](https://golang.org)
[![Build Status](https://github.com/clcollins/srepd/actions/workflows/go-ci.yml/badge.svg)](https://github.com/clcollins/srepd/actions/workflows/go-ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/clcollins/srepd)](https://goreportcard.com/report/github.com/clcollins/srepd)
[![License](https://img.shields.io/github/license/clcollins/srepd)](https://github.com/clcollins/srepd/blob/main/LICENSE)

A PagerDuty terminal user interface focused on common SRE tasks.

## Features

* Retrieve and list incidents assigned to the current user, and their team(s)
* View a summary of an incident, including alerts and notes, with clickable markdown links
* Add a note to an incident
* Acknowledge and un-acknowledge (re-escalate) incidents with confirmation prompts
* Silence incidents by reassigning to a configured "silent" escalation policy
* Open SOP/runbook links directly from alerts (`s` key)
* Open incidents in the browser (`o` key)
* Log into clusters directly from an incident, launching a terminal window with `ocm-container` or `ocm backplane`
* Multi-cluster selection when an incident has alerts referencing multiple clusters (keys `1`-`9`)
* Toggle between team and individual incident views (`t` key)
* Filter incidents by urgency -- show all or high-urgency only (`u` key)
* Action log showing recent write actions (`ctrl+l` to toggle)
* Auto-refresh with selection preservation across refresh cycles
* Auto-acknowledge incidents when you are on-call
* Rate limiting with exponential backoff for PagerDuty API calls
* Toolbox auto-detection -- when running inside a Fedora Toolbox container, terminal commands are automatically prefixed with `flatpak-spawn --host`
* Individual `PAGERDUTY_*` environment variables passed to terminals/ocm-container for investigation context
* Alert normalization across 6 PagerDuty alert types (upcoming -- PR #187)
* Multi-terminal profile auto-detection for correct argument formatting (upcoming -- PR #183)
* Flatpak-spawn environment variable passing for toolbox workflows (upcoming -- PR #185)
* Resizes nicely when the terminal is resized

## Key Bindings

Press `h` to toggle the full help overlay inside srepd.

### Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move down |
| `k` / `Up` | Move up |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Enter` | View selected incident |
| `Esc` | Go back / dismiss |

### Actions

| Key | Action |
|-----|--------|
| `a` | Acknowledge selected incident |
| `n` | Add a note to selected incident |
| `l` | Log into cluster (opens terminal) |
| `o` | Open incident in browser |
| `s` | Open SOP/runbook link from alert |
| `ctrl+s` | Silence incident (reassign to silent escalation policy) |
| `ctrl+e` | Re-escalate incident (un-acknowledge) |

### Toggles

| Key | Action |
|-----|--------|
| `h` | Toggle help |
| `t` | Toggle team / individual view |
| `r` | Manual refresh |
| `ctrl+r` | Toggle auto-refresh |
| `ctrl+a` | Toggle auto-acknowledge |
| `u` | Toggle urgency filter (all / high only) |
| `ctrl+l` | Toggle action log |

### Input Mode

When in input mode (`i` or `:`), the following keys are active:

| Key | Action |
|-----|--------|
| `Enter` | Submit input |
| `Esc` | Cancel and go back |
| `ctrl+q` / `ctrl+c` | Quit |

### Quit

| Key | Action |
|-----|--------|
| `ctrl+q` / `ctrl+c` | Quit srepd |

## Configuration

SREPD uses the [Viper](https://github.com/spf13/viper) configuration setup, and reads values from `~/.config/srepd/srepd.yaml`.

Configuration variables have the following precedence:

`command line arguments > environment variables > config file values`

Environment variables use the `SREPD_` prefix (e.g., `SREPD_TOKEN`, `SREPD_TERMINAL`).

You can generate a sample config with `srepd config --create` and validate an existing config with `srepd config --validate`.

### Required Values

| Key | Type | Description |
|-----|------|-------------|
| `token` | `string` | PagerDuty API OAuth token |
| `teams` | `[]string` | List of PagerDuty team IDs to gather incidents for |
| `service_escalation_policies` | `map[string]string` | Escalation policy mapping (see below) |

The `service_escalation_policies` map must contain at least two keys:

* `DEFAULT` -- the escalation policy used when re-escalating incidents
* `SILENT_DEFAULT` -- the escalation policy used when silencing incidents

Optional keys are PagerDuty service IDs mapped to specific escalation policy IDs, allowing per-service silence behavior:

```yaml
service_escalation_policies:
  DEFAULT: P123456
  SILENT_DEFAULT: P654321
  PABC123: PXYZ890   # Service-specific silence policy
```

### Optional Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `editor` | `string` | `$EDITOR` or `vim` | Editor for writing incident notes |
| `terminal` | `string` | `gnome-terminal --` | Terminal emulator for cluster login |
| `cluster_login_command` | `string` | `ocm backplane login %%CLUSTER_ID%%` | Command to log into a cluster |
| `ignoredusers` | `[]string` | (none) | PagerDuty user IDs to exclude from incident lists |
| `toolbox_mode` | `string` | `auto` | Toolbox detection: `auto`, `true`, or `false` |

### Example Configuration

```yaml
---
token: <PagerDuty API token>

teams:
  - <PagerDuty Team ID>

service_escalation_policies:
  DEFAULT: P123456
  SILENT_DEFAULT: P654321
  PABC123: PXYZ890

ignoredusers:
  - <PagerDuty User ID>

editor: vim

terminal: gnome-terminal --
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%

# Toolbox mode: "auto" detects Fedora Toolbox and prefixes with flatpak-spawn --host
# Set to "true" to force, "false" to disable
toolbox_mode: auto
```

## Getting Started

### Prerequisites

* Go 1.24.2 or later
* A PagerDuty API token
* A configuration file at `~/.config/srepd/srepd.yaml`

### Install

```bash
# Clone and install
git clone https://github.com/clcollins/srepd.git
cd srepd
make install
```

Or build a snapshot binary:

```bash
make build
```

### Run

```bash
srepd
```

Enable debug logging:

```bash
srepd --debug
```

## Terminal Support

SREPD launches external terminal windows for cluster login. Each terminal has its own way of accepting commands. Configure the `terminal` value in your config file to match your terminal emulator.

### Linux Terminals

```yaml
# gnome-terminal (separator: --)
terminal: gnome-terminal --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# ptyxis / GNOME Console (separator: --)
terminal: ptyxis --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# konsole (flag: -e)
terminal: konsole -e
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# kitty (direct -- no separator needed)
terminal: kitty
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# alacritty (flag: -e)
terminal: alacritty -e
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# wezterm (separator: --)
terminal: wezterm start --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# foot (direct -- no separator needed)
terminal: foot
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# ghostty (flag: -e)
terminal: ghostty -e
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# terminator (flag: --execute)
terminal: terminator --execute
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

### Flatpak Terminals

Terminals installed via Flatpak can be launched using the `flatpak run` prefix with the application ID:

```yaml
# Konsole via Flatpak
terminal: flatpak run org.kde.konsole -e
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Contour via Flatpak
terminal: flatpak run org.contourterminal.Contour
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# BlackBox via Flatpak
terminal: flatpak run com.raggesilver.BlackBox --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

**Note:** When multi-terminal profiles land (PR #183), the terminal profile (separator style) will be auto-detected from the executable name or Flatpak app ID, so the separator/flag configuration will become optional.

### tmux

If you are already using tmux, you can open new tmux windows instead of launching a full terminal:

```yaml
terminal: tmux new-window --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

### macOS

#### Default macOS Terminal

```yaml
terminal: osascript -e
cluster_login_command: tell application "Terminal" to do script "ocm-container -C %%CLUSTER_ID%%"
```

#### iTerm2

iTerm2 requires a wrapper script. Create a script (e.g., `~/bin/iterm2-srepd.sh`):

```bash
#!/bin/bash
osascript \
  -e 'tell application "iTerm" to tell current window to set newWindow to (create tab with default profile)' \
  -e "tell application \"iTerm\" to tell current session of newWindow to write text \"${*}\""
```

Then configure:

```yaml
terminal: ~/bin/iterm2-srepd.sh
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

### Toolbox Auto-Detection

When running srepd inside a Fedora Toolbox container, terminal commands need to execute on the host system. SREPD detects this automatically and prefixes terminal commands with `flatpak-spawn --host`.

This behavior is controlled by the `toolbox_mode` config key:

| Value | Behavior |
|-------|----------|
| `auto` (default) | Auto-detect Fedora Toolbox environment |
| `true` | Always prefix with `flatpak-spawn --host` |
| `false` | Never prefix (even inside a toolbox) |

## Automatic Login Features

### Variable Substitution

When you include `%%VARIABLE%%` placeholders in your `terminal` or `cluster_login_command` configuration, they are dynamically replaced with incident-specific values at launch time.

Supported variables:

| Variable | Description |
|----------|-------------|
| `%%CLUSTER_ID%%` | The cluster ID extracted from the incident's alerts |
| `%%INCIDENT_ID%%` | The PagerDuty incident ID |

**Note:** The first argument of the `terminal` setting must not be a replaceable value.

If `%%CLUSTER_ID%%` does not appear in `cluster_login_command`, the cluster ID is appended to the end of the command automatically.

Examples:

```text
# Assume the cluster ID is "abcdefg"

# Runs: ocm backplane login abcdefg --multi
cluster_login_command: ocm backplane login %%CLUSTER_ID%% --multi

# Runs: ocm backplane login --multi abcdefg  (auto-appended)
cluster_login_command: ocm backplane login --multi

# Runs: ocm-container --cluster-id abcdefg --launch-opts --env=INCIDENT_ID=Q1ABC23
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%% --launch-opts --env=INCIDENT_ID=%%INCIDENT_ID%%
```

### Multi-Cluster Selection

When an incident has alerts referencing multiple clusters, srepd prompts you to choose which cluster to log into. Press the number key (`1`-`9`) corresponding to the cluster in the selection list.

## Environment Variables

When using `ocm-container` as your cluster login command, srepd automatically passes PagerDuty incident and alert information as individual environment variables to the container. This allows you to access incident context from within the ocm-container session without manual configuration.

The following environment variables are set:

| Variable | Description |
|----------|-------------|
| `PAGERDUTY_INCIDENT_ID` | PagerDuty incident ID |
| `PAGERDUTY_INCIDENT_TITLE` | Incident title (sanitized for shell safety) |
| `PAGERDUTY_INCIDENT_URL` | Direct link to the incident in PagerDuty |
| `PAGERDUTY_INCIDENT_SERVICE` | PagerDuty service name for the incident |
| `PAGERDUTY_INCIDENT_URGENCY` | Incident urgency (`high` or `low`) |
| `PAGERDUTY_INCIDENT_STATUS` | Incident status (`triggered`, `acknowledged`, `resolved`) |
| `PAGERDUTY_CLUSTER_ID` | The selected cluster ID |
| `PAGERDUTY_ALERT_COUNT` | Number of alerts matching the selected cluster |
| `PAGERDUTY_ALERT_NAMES` | Comma-separated alert names for the selected cluster |
| `PAGERDUTY_ALERT_LINKS` | Comma-separated SOP/runbook links from alerts |
| `PAGERDUTY_NOTES_EXIST` | `true` or `false` -- whether the incident has notes |
| `PAGERDUTY_NOTE_COUNT` | Number of notes on the incident |
| `REASON` | Set to the incident URL (for compliance/audit integration) |

Example usage inside ocm-container:

```bash
# View incident context
echo "Incident: $PAGERDUTY_INCIDENT_ID"
echo "Title: $PAGERDUTY_INCIDENT_TITLE"
echo "Cluster: $PAGERDUTY_CLUSTER_ID"
echo "Alerts: $PAGERDUTY_ALERT_NAMES"

# Check if notes exist before fetching
if [ "$PAGERDUTY_NOTES_EXIST" = "true" ]; then
  echo "$PAGERDUTY_NOTE_COUNT notes on this incident"
fi

# Use REASON for compliance tracking
echo "REASON: $REASON"
```

These environment variables are automatically set when you use the login feature (press `l` on an incident). Only alerts matching the selected cluster are included in the alert-related variables. No additional configuration is required.

**Note:** Values containing characters that could cause shell issues (newlines, quotes, etc.) are sanitized automatically.

## Build and Development

### Requirements

* Go 1.24.2 or later
* GNU Make

### Make Targets

| Command | Purpose |
|---------|---------|
| `make build` | Build via goreleaser snapshot |
| `make install` | Install to `$GOPATH/bin` |
| `make install-local` | Build and install to `~/.local/bin` |
| `make run` | Run the application locally |
| `make test` | Run unit tests (`go test ./...`) |
| `make lint` | Run golangci-lint |
| `make vet` | Run `go vet ./...` |
| `make fmt` | Format code with `gofmt -s` |
| `make fmt-check` | Check formatting (CI-friendly, exits non-zero on diff) |
| `make coverage` | Generate test coverage report |
| `make tidy` | Run `go mod tidy` |
| `make clean` | Remove build artifacts |
| `make test-all` | Run all checks: `fmt-check`, `vet`, `lint`, `test` |
| `make plan-check` | Verify a plan document exists for the branch |
| `make release` | Create a release using goreleaser |
| `make help` | Show all available targets |

Pass extra test flags via `TESTOPTS`:

```bash
make test TESTOPTS="-run TestFoo"
```

### PR Workflow

1. Create a feature branch: `srepd/<description>`
2. Write failing tests (TDD)
3. Implement minimum code to pass tests
4. Create a plan document in `docs/plans/`
5. Run `make test-all` locally
6. Push and create PR against `main`

Every PR must include a plan document in `docs/plans/` and pass all CI checks (`make test-all` and `make plan-check`).

## Architecture

SREPD is built with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework using the Model-View-Update (MVU) pattern.

| Package | Purpose |
|---------|---------|
| `cmd/` | CLI entry point and config validation (Cobra) |
| `pkg/tui/` | TUI model, update loop, views, key bindings |
| `pkg/pd/` | PagerDuty API wrapper and mock client |
| `pkg/launcher/` | Terminal command builder with variable substitution |
| `pkg/container/` | Container/toolbox environment detection |

All PagerDuty API calls run as `tea.Cmd` closures returning `tea.Msg` values. The update loop is single-threaded. The PagerDuty client is abstracted behind `PagerDutyClientInterface` for testability.

## License

MIT License. See [LICENSE](LICENSE) for details.
