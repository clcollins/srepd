# Terminal Configuration

SREPD opens a new terminal window when you press `Enter` on an incident
to log into a cluster. This document explains how terminal detection,
profiles, and environment variable passing work across Linux and macOS.

## Quick Start

Set the `terminal` key in `~/.config/srepd/srepd.yaml` to the name of
your terminal emulator. SREPD handles the rest automatically:

```yaml
terminal: kitty
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

The config wizard (`srepd config`) detects installed terminals and
offers them as choices — you don't need to know the exact name.

## Supported Terminals

### Linux

| Terminal | Config value | Profile | Argument style |
|----------|-------------|---------|----------------|
| GNOME Terminal | `gnome-terminal` | separator | `gnome-terminal -- <cmd>` |
| Ptyxis | `ptyxis` | separator | `ptyxis -- <cmd>` |
| WezTerm | `wezterm` | separator | `wezterm -- <cmd>` |
| BlackBox | `blackbox` | separator | `blackbox -- <cmd>` |
| Konsole | `konsole` | flag | `konsole -e <cmd>` |
| Alacritty | `alacritty` | flag | `alacritty -e <cmd>` |
| Ghostty | `ghostty` | flag | `ghostty -e <cmd>` |
| Terminator | `terminator` | flag | `terminator --execute <cmd>` |
| kitty | `kitty` | direct | `kitty <cmd>` |
| foot | `foot` | direct | `foot <cmd>` |
| Contour | `contour` | direct | `contour <cmd>` |
| tmux | `tmux` | generic | `tmux <cmd>` (only inside a tmux session) |

All Linux terminals are detected by probing `$PATH` for the executable.

### Flatpak

Flatpak-installed terminals are supported using their application ID:

```yaml
terminal: org.kde.konsole
```

SREPD prepends `flatpak run` automatically. Recognized Flatpak app IDs:
`org.gnome.Terminal`, `org.gnome.Ptyxis`, `org.wezfurlong.wezterm`,
`com.raggesilver.BlackBox`, `org.kde.konsole`, `org.codeberg.dnkl.foot`,
`net.kovidgoyal.kitty`, `io.github.AtomsDevs.Contour`.

### macOS

| Terminal | Config value | Profile | How it launches |
|----------|-------------|---------|-----------------|
| Terminal.app | `terminal` | AppleScript | `osascript -e 'tell application "Terminal" to do script "<cmd>"'` |
| iTerm2 | `iterm2` | AppleScript | `osascript -e 'tell application "iTerm2" to create window with default profile command "<cmd>"'` |
| kitty | full binary path | direct | `/Applications/kitty.app/Contents/MacOS/kitty <cmd>` |
| Alacritty | full binary path | flag | `/Applications/Alacritty.app/Contents/MacOS/alacritty -e <cmd>` |
| WezTerm | full binary path | separator | `/Applications/WezTerm.app/Contents/MacOS/wezterm -- <cmd>` |

**Detection:**

- **Terminal.app** is always available (built into macOS).
- **iTerm2** is offered only when `iTerm.app` exists in `/Applications/`
  or `~/Applications/`.
- **kitty, Alacritty, WezTerm** are detected by checking for their
  `.app` bundle in `/Applications/` and `~/Applications/` (in that
  order; `/Applications/` takes precedence). When found, the config
  value is set to the full binary path inside the bundle (e.g.,
  `/Applications/kitty.app/Contents/MacOS/kitty`) so that
  `exec.Command` can find the binary even when it's not on `$PATH`.
  If the terminal IS on `$PATH` (e.g., via Homebrew), the bare name
  is used instead.
- **Ghostty** is not currently supported for bundle detection. Its
  macOS CLI cannot reliably launch terminal windows. Ghostty >=1.3
  supports AppleScript, which a future PR will add as a dedicated
  profile. If Ghostty is on `$PATH`, it works via the flag profile.

**TCC (Transparency, Consent, and Control):**

macOS TCC may block AppleScript automation on the first use. When this
happens, srepd shows an actionable error message explaining how to
grant permission in System Settings > Privacy & Security > Automation.
If the permission toggle is missing or stuck, toggling it off/on or
running `tccutil reset AppleEvents` in a terminal can help.

### Fedora Toolbox

When running inside a Fedora Toolbox container, SREPD automatically
prefixes terminal commands with `flatpak-spawn --host` so the terminal
launches on the host system. This is controlled by the `toolbox_mode`
config key (`auto`, `true`, or `false`).

## How Profiles Work

You only set the terminal name in your config. SREPD auto-detects the
correct "profile" — the argument style your terminal expects — from
the executable name (using `filepath.Base`, so full paths work).

There are five profile types:

- **Separator** (`--`): Terminal args, then `--`, then the login command.
- **Flag** (`-e` or `--execute`): Terminal args, then a flag, then the
  login command.
- **Direct**: Terminal args followed directly by the login command (no
  separator or flag).
- **AppleScript**: Constructs an `osascript -e '...'` command that tells
  the macOS application to run the login command.
- **Generic**: Fallback — concatenates terminal args and login command.

If you already have the separator or flag in your terminal config string
(e.g., `gnome-terminal --`), SREPD detects this and doesn't double it.

## Environment Variables

When you log into a cluster, SREPD passes PagerDuty context as
environment variables so your shell or tooling can use them:

| Variable | Example |
|----------|---------|
| `PAGERDUTY_INCIDENT_ID` | `P1234ABC` |
| `PAGERDUTY_INCIDENT_TITLE` | `ClusterOperatorDown CRITICAL` |
| `PAGERDUTY_INCIDENT_URL` | `https://...` |
| `PAGERDUTY_INCIDENT_SERVICE` | `osd-mycluster...` |
| `PAGERDUTY_INCIDENT_URGENCY` | `high` |
| `PAGERDUTY_INCIDENT_STATUS` | `triggered` |
| `PAGERDUTY_CLUSTER_ID` | `abc-123-def` |
| `PAGERDUTY_ALERT_COUNT` | `2` |
| `PAGERDUTY_ALERT_NAMES` | `ClusterOperatorDown,KubePodCrashLooping` |
| `PAGERDUTY_ALERT_LINKS` | `https://...sop...` |
| `PAGERDUTY_NOTES_EXIST` | `true` |
| `PAGERDUTY_NOTE_COUNT` | `3` |
| `PAGERDUTY_CLAUDE_AVAILABLE` | `true` (when `claude` CLI is on PATH) |
| `REASON` | Same as `PAGERDUTY_INCIDENT_URL` |

How these variables reach the terminal session depends on the
environment:

| Scenario | Mechanism |
|----------|-----------|
| **ocm-container** (non-AppleScript) | `-e KEY=VAL` flags spliced after `ocm-container` in the login command |
| **AppleScript + ocm-container** | `-e` flags in the exec line inside a wrapper script |
| **AppleScript + no ocm-container** | `export KEY=VAL` lines in a wrapper script |
| **Toolbox** (non-ocm) | `--env=KEY=VAL` flags on `flatpak-spawn` |
| **Direct** (none of the above) | `exec.Cmd.Env` on the child process |

### Wrapper Scripts (macOS AppleScript terminals)

AppleScript terminals (Terminal.app, iTerm2) cannot receive environment
variables through `exec.Cmd.Env` — AppleEvents don't carry process
environment. SREPD solves this by writing a short wrapper script to
`~/.cache/srepd/launch/login-<random>.sh` that exports the variables
and exec's the login command:

```sh
#!/bin/sh
export PAGERDUTY_INCIDENT_ID='P1234ABC'
export PAGERDUTY_INCIDENT_TITLE='CPU high on node-42'
# ... more exports ...
exec 'ocm' 'backplane' 'login' 'abc-123-def'
```

The AppleScript then runs this script instead of the raw login command.
Scripts older than 1 hour are cleaned up automatically at startup.

**`SREPD_TEST_WRAPPER_DIR`:** By default, wrapper scripts are written
to `~/.cache/srepd/launch/`. The `SREPD_TEST_WRAPPER_DIR` environment
variable overrides this location for test isolation — it exists so that
unit tests don't write to the real cache directory.

When `cluster_login_command` contains `ocm-container`, the `-e` flags
are spliced into the exec line and the wrapper script does NOT also
emit `export` lines — the variables are passed only once.

## Variable Substitution

The `terminal` and `cluster_login_command` strings support placeholder
variables that are replaced at launch time:

| Placeholder | Replaced with |
|-------------|---------------|
| `%%CLUSTER_ID%%` | The cluster ID from the incident's alert data |
| `%%INCIDENT_ID%%` | The PagerDuty incident ID |

At least one of `terminal` or `cluster_login_command` must contain
`%%CLUSTER_ID%%`.

## Examples

### Linux with GNOME Terminal and ocm-container

```yaml
terminal: gnome-terminal
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%
```

Produces: `gnome-terminal -- ocm-container -e PAGERDUTY_INCIDENT_ID=P123 ... --cluster-id abc-123`

### Linux with kitty and ocm backplane

```yaml
terminal: kitty
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

Produces: `kitty ocm backplane login abc-123` (env vars set via
`exec.Cmd.Env`)

### macOS with iTerm2 and ocm-container

```yaml
terminal: iterm2
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%
```

Produces: a wrapper script with `-e` flags in the exec line, launched
via `osascript -e 'tell application "iTerm2" to create window with
default profile command "/path/to/wrapper.sh"'`

### macOS with bundle-installed kitty

```yaml
terminal: /Applications/kitty.app/Contents/MacOS/kitty
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

Produces: `/Applications/kitty.app/Contents/MacOS/kitty ocm backplane
login abc-123` (env vars set via `exec.Cmd.Env`)

### Flatpak konsole inside Toolbox (non-ocm login)

```yaml
terminal: org.kde.konsole
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
toolbox_mode: auto
```

Produces: `flatpak-spawn --host --env=PAGERDUTY_INCIDENT_ID=P123 ...
flatpak run org.kde.konsole -e ocm backplane login abc-123`

## Known Limitations and Future Work

- **Ghostty macOS bundle:** Ghostty >=1.3 supports AppleScript, which
  will be added as a dedicated profile in a future PR. Until then,
  Ghostty only works on macOS when installed on `$PATH`.
