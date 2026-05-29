# SREPD

[![Go Version](https://img.shields.io/github/go-mod/go-version/clcollins/srepd)](https://golang.org)
[![Build Status](https://github.com/clcollins/srepd/actions/workflows/go-ci.yml/badge.svg)](https://github.com/clcollins/srepd/actions/workflows/go-ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/clcollins/srepd)](https://goreportcard.com/report/github.com/clcollins/srepd)
[![License](https://img.shields.io/github/license/clcollins/srepd)](https://github.com/clcollins/srepd/blob/main/LICENSE)

A PagerDuty terminal user interface focused on common SRE tasks

**Note: This project is still in Beta phase and there are bugs to be squashed.**

Features:

* Retrieve and list incidents assigned to the current user, and their team(s)
* View a summary of an incident, including alerts and notes
* Add a note to an incident
* Reassign incidents to a (configured) "silent" escalation policy (ie. silence the alert), with confirmation prompt
* Acknowledge incidents
* Re-escalate incidents (un-acknowledge and reassign to the default escalation policy), with confirmation prompt
* Resizes nicely(-ish) when the terminal is resized
* Launch SOP links from alerts directly in the browser
* Open incidents in the browser
* Toggle urgency filter to show only high-urgency incidents
* Multi-cluster selection when an incident has alerts for multiple clusters (keys 1-9)
* Action log tracking recent actions and resolved incidents
* Rate limiting for PagerDuty API calls
* Selection preservation on auto-refresh (cursor stays on the same incident)
* Auto-acknowledge incidents assigned to you while on-call
* Login to clusters directly from incidents, with automatic PagerDuty environment variables passed to ocm-container

Planned Features:

* View arbitrary incidents
* Assign incidents to any PagerDuty User ID
* Edit incident titles
* Merge incidents

## Requirements

* Go 1.26.3 or later

## Key Bindings

Press `h` at any time to toggle the in-app help overlay.

### Navigation

| Key | Action |
|-----|--------|
| `k` / `Up` | Move up |
| `j` / `Down` | Move down |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Enter` | View incident details |
| `h` | Toggle help |
| `Esc` | Back (close incident view, cancel selection, exit input mode) |

### Actions

| Key | Action |
|-----|--------|
| `a` | Acknowledge incident |
| `Ctrl+s` | Silence incident (requires `y/n` confirmation) |
| `Ctrl+e` | Re-escalate incident (requires `y/n` confirmation) |
| `n` | Add a note to the incident |
| `l` | Login to the cluster associated with the incident |
| `o` | Open the incident in the browser |
| `s` | Open the SOP link from the incident's alerts in the browser |
| `1`-`9` | Select a cluster when multiple clusters are available |

### Toggles

| Key | Action |
|-----|--------|
| `t` | Toggle between team and individual incident views |
| `r` | Refresh the incident list |
| `Ctrl+r` | Toggle auto-refresh |
| `Ctrl+a` | Toggle auto-acknowledge |
| `u` | Toggle urgency filter (show all vs. high-urgency only) |
| `Ctrl+l` | Toggle the action log |
| `i` / `:` | Enter input mode |

### Quit

| Key | Action |
|-----|--------|
| `Ctrl+q` | Quit |
| `Ctrl+c` | Quit |

## Configuration

SREPD uses the [Viper](https://github.com/spf13/viper) configuration setup, and will read required values from `~/.config/srepd/srepd.yaml`.

Configuration variables have the following precedence:

`command line arguments > environment variables > config file values`

### Required Values

* token: A PagerDuty Oauth Token
* teams: A list of PagerDuty team IDs to gather incidents for
* service_escalation_policies: A string map defining the default escalation policy, "silence" policy, and optional per-service "silence" polices. The "DEFAULT" and "SILENT_DEFAULT" keys are required. Optional keys are PagerDuty service IDs.  All values are PageDuty escalation policy IDs.

Example service_escalation_policies configuration:

```text
# In this example, the Default escalation policy in use by the team is P123456, and re-escalating alerts will be assigned to that policy.
# Alerts that are "silenced" will be re-assigned to escalation policy P654321.
# Any alerts for service PABC123 will be silenced by re-assigning to escalation policy PXYZ890 instead of the SILENT_DEFAULT policy.
service_escalation_policies:
  DEFAULT: P123456
  SILENT_DEFAULT: P654321
  PABC123: PXYZ890
```

### Optional Values

* ignoredusers: A list of PagerDuty user IDs to exclude from retrieved incident lists.
* editor: Your choice of editor.  Defaults to the `$EDITOR` environment variable.
* cluster_login_command: Command used to login to a cluster from SREPD.  Defaults to `/usr/local/bin/ocm backplane login`
* terminal: Your choice of terminal to use when launching external commands. Defaults to `/usr/bin/gnome-terminal`.

**NOTE:** The cluster_login_command and terminal accept a variable for `%%CLUSTER_ID%%` to stand in for the Cluster ID in the command. At least one of the two, most likely `cluster_login_command` MUST have the `%%CLUSTER_ID%%` placeholder set. See [AUTOMATIC LOGIN FEATURES](#automatic-login-features) for more details about config variables.

An example srepd.yaml file might look like so:

```yaml
---
# Editor will always be overridden by the ENV variable
# unless the ENV is not set for some reason
# type: string
editor: vim

# Cluster Login options
# Note the trailing `--` is necessary for gnome-terminal and may be necessary
# for other terminals as well
# type: string
terminal: /usr/bin/gnome-terminal --
# type: string
cluster_login_command: ocm-container --clusterid %%CLUSTER_ID%%

# Note that aliases, etc, are not sourced by the shell command when launching.
# This means, for example, that `ocm-container`, as normally setup using an
# alias, does not work, but calling the command or a symlink directly does.

# More complicated commands can be specified with space-separated strings
# terminal: "flatpak run org.contourterminal.Contour"

# PagerDutyOauthToken
# type: string
token: <pagerDuty Oauth Token>

# Teams are PagerDuty team IDs to retrieve incidents
# type: []string
teams:
  - <pagerDuty Team ID>

# Service Escalation Policies are a map of services to escalation policies, including the required "DEFAULT" and "SILENT_DEFAULT" keys.  Optional PagerDuty Service keys and PagerDuty Escalation Policies may be defined to customize how silenced serivces are assigned.
# type: map[string]string
service_escalation_policies:
  DEFAULT: P123456
  SILENT_DEFAULT: P654321
  PABC123: PXYZ890

# Ignore Users is a list of PagerDuty User IDs to ignore when gathering incidents
# type: []string
ignoredusers:
  - <pagerDuty User ID>
  - <pagerDuty User ID>
```

## Building

SREPD uses a Makefile for common development tasks. Run `make help` to see all available targets.

| Target | Description |
|--------|-------------|
| `make build` | Build the application using goreleaser (snapshot) |
| `make install` | Install the binary to `$GOPATH/bin` |
| `make install-local` | Build and install to `~/.local/bin` |
| `make test` | Run unit tests |
| `make lint` | Run golangci-lint |
| `make vet` | Run `go vet` to catch common mistakes |
| `make fmt` | Format the code with `gofmt -s` |
| `make fmt-check` | Check code formatting (CI-friendly, exits non-zero if unformatted) |
| `make coverage` | Generate test coverage report |
| `make test-all` | Run all checks: fmt-check, vet, lint, test |
| `make plan-check` | Check that a plan document exists for the current branch |
| `make tidy` | Tidy up go modules |
| `make clean` | Clean up build artifacts |
| `make run` | Run the application locally via `go run` |
| `make release` | Create a release using goreleaser |

## Try it out

The easiest way to get started with SREPD, after adding the required config file, is to just clone this repository and run `go build -o ${GOPATH}/bin/srepd .`

## Automatic Login Configuration

To enable automatic login directly from SREPD you will need to configure the `terminal` and `cluster_login_command` settings in the srepd config file.

### Linux

A typical linux configuration to launch a new terminal window may look something like what follows. Be sure to change to your preferred terminal or preferred ops environment (ocm-container, ocm-backplane session, osdctl session, etc)

```yaml
# Gnome-Terminal
# gnome-terminal requires the "--" separator between the terminal any any of its flags and the command to be run
# eg: gnome-terminal -- ocm backplane login %%CLUSTER_ID%%
terminal: gnome-terminal --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

Each terminal has its own way of accepting an argument for a command to run after launching. Some examples known to work:

```yaml
# Contour (in this example, via Flatpak)
# contour does not require any special arguments or separators
terminal: flatpak run org.contourterminal.Contour
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# BlackBox (via Flatpak)
terminal: flatpak run com.raggesilver.BlackBox --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Terminator
# terminator requires the "-x" or "--execute" flag as the separator between terminal arguments and the command to be run
terminal: terminator --execute
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Ptyxis (GNOME's newer terminal, available via Flatpak)
terminal: flatpak run app.devsuite.Ptyxis --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Konsole (KDE)
# konsole uses the "-e" flag to execute a command
terminal: konsole -e
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Kitty
# kitty accepts the command directly after the terminal name
terminal: kitty
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Alacritty
# alacritty uses the "-e" flag to execute a command
terminal: alacritty -e
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# WezTerm
# wezterm uses the "start --" subcommand to execute a command
terminal: wezterm start --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Foot (Wayland-native terminal)
# foot accepts the command directly after the terminal name
terminal: foot
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

### MacOS

Configuration for MacOS is not as straightforward because of the way that MacOS handles applications differently from Linux. We've provided a few separate ways to configure SREPD to handle automatic logins.

#### Default MacOS terminal

The following configuration will launch a new MacOS default terminal window to automatically login to a cluster. This example uses ocm-container but feel free to modify to fit your preferred workflow.

```yaml
# MacOS Terminal
terminal: osascript -e
cluster_login_command: tell application "Terminal" to do script "ocm-container -C %%CLUSTER_ID%%"
```

#### iTerm2 Support

iTerm2 is even more special in the fact that you can't tell it to 'do script' like you can with the default Terminal on MacOS. You will need to create a separate shell script to invoke from SREPD that will then run the osascript command needed to create a new iTerm2 window/tab and call the login script.

```bash
#!/bin/bash

osascript \
  -e 'tell application "iTerm" to tell current window to set newWindow to (create tab with default profile)' \
  -e "tell application \"iTerm\" to tell current session of newWindow to write text \"${*}\""
```

Then, you would add the following to your srepd config:

```yaml
# iTerm2
terminal: /Users/kbater/Projects/spikes/srepd
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

### TMUX Support

Alternatively to launching a whole new terminal window, if you're already using tmux you can use the following configuration to create new tmux-windows and auto-launch your environment from there:

```yaml
# TMUX
terminal: tmux new-window --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

### Automatic Login Features

The first feature of Automatic Login is the ability to replace certain strings with their cluster-specific details. When you pass `%%VARIABLE%%` in your `terminal` or `cluster_login_command` configuration strings they will dynamically be replaced with the alert-specific variable. This allows you to be able to put the specific details of these variables inside the command. The first argument of the `terminal` setting MUST NOT BE a replaceable value.

Supported Variables:

* `%%CLUSTER_ID%%` - used to identify the cluster to log into. (SEE NOTE BELOW)
* `%%INCIDENT_ID%%` - the PagerDuty Incident ID from which the cluster details have been taken.  You can, for example, use this to pass in compliance reasons, or to set a variable.

Note regarding `%%CLUSTER_ID%%`: 

It's also important to note that if `%%CLUSTER_ID%%` does NOT appear in the `cluster_login_command` config setting that the cluster ID will be appended to the end of the cluster login command. If the replaceable `%%CLUSTER_ID%%` string is present in the `cluster_login_command` setting, it will NOT be appended to the end.

Examples:

```text
## Assume the cluster ID for these examples is `abcdefg`

## effectively runs "ocm backplane login abcdefg --multi"
cluster_login_command: ocm backplane login %%CLUSTER_ID%% --multi

## effectively runs "ocm backplane login --multi abcdefg"
cluster_login_command: ocm backplane login --multi

## Logs into the cluster and sets the INCIDENT_ID env variable in ocm-container to the PagerDuty Incident ID
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID --launch-opts --env=INCIDENT_ID=%%INCIDENT_ID%%
```

### Automatic PagerDuty Environment Variables

When using `ocm-container` as your cluster login command, srepd automatically passes PagerDuty incident and alert information as environment variables to the container. This allows you to access incident details and alert data from within the ocm-container session without manual configuration.

The following environment variables are automatically set:

* `PAGERDUTY_INCIDENT` - The PagerDuty incident ID
* `ALERT_DETAILS` - Base64-encoded JSON containing the full incident object, all associated alerts, and incident notes

Example usage inside ocm-container:

```bash
# View the incident ID
echo $PAGERDUTY_INCIDENT

# Decode and view the full alert details
echo $ALERT_DETAILS | base64 -d | jq .

# Extract specific alert information
echo $ALERT_DETAILS | base64 -d | jq '.alerts[0].body.details.cluster_id'

# View incident notes
echo $ALERT_DETAILS | base64 -d | jq '.notes'
```

**Note:** The `ALERT_DETAILS` variable uses standard base64 encoding without padding to avoid parsing issues with `=` characters in ocm-container's env var handling.

These environment variables are automatically added when you use the login feature (press `l` on an incident). No additional configuration is required.
