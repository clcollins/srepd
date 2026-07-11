# Team Presets

A preset is a YAML file containing team policy decisions that new SREs
inherit instead of hunting for PagerDuty IDs. It pre-seeds the config
wizard — every value is still shown to and confirmed by the user.

## Usage

```bash
# From a local file
srepd config --preset /path/to/team-preset.yaml

# From an HTTPS URL (e.g. a GitHub gist)
srepd config --preset https://gist.githubusercontent.com/.../preset.yaml
```

The wizard opens with preset values pre-filled. The summary step tags
preset-sourced values with "(from preset: <source>)".

## Example preset

```yaml
# SREP Team Preset
# Only include keys your team standardizes on — everything else
# is discovered by the wizard or left at the user's preference.

# Team ID(s) to monitor
teams:
  - PASPK4G

# Silent escalation policy for silencing incidents
default_silent_escalation_policy: PCGXUDY

# Per-service overrides (service ID → silent policy ID)
custom_service_escalation_policies:
  P5LAB5Y: PVBANNN

# Cluster login command (ocm backplane or ocm-container)
cluster_login_command: ocm backplane login %%CLUSTER_ID%%

# Optional: terminal and editor preferences
# terminal: gnome-terminal --
# editor: vim
```

## Allowed keys

| Key | Purpose |
|-----|---------|
| `teams` | PagerDuty team IDs to monitor |
| `default_silent_escalation_policy` | Silent escalation policy ID |
| `custom_service_escalation_policies` | Per-service silent policy overrides |
| `cluster_login_command` | Cluster login command template |
| `terminal` | Terminal emulator |
| `editor` | Editor for incident notes |

Any other key is **rejected loudly** — a typo in a team preset should fail
review, not be silently ignored. In particular, `token` and `llm_api` are
never accepted: presets carry team policy, not credentials.

## How presets interact with existing config

- **Existing values always win.** A preset only fills values the user hasn't
  configured. If your config already has a `terminal`, the preset's
  `terminal` is ignored.
- **Presets don't bypass the wizard.** Every value is shown in the wizard for
  confirmation. The summary tags preset-sourced values.
- **`--preset` is explicit.** Presets are never auto-fetched. The user must
  pass the flag.

## Security

- **HTTPS only** for URL fetches — HTTP is rejected
- **64KB size cap** — a preset is a handful of IDs, not a megabyte document
- **No credentials** — `token` and `llm_api` keys are always rejected
- **No redirects to insecure hosts** — Go's `http.Client` follows redirects
  but the initial URL must be HTTPS

## Publishing a preset for your team

1. Create a YAML file with your team's policy decisions (see example above)
2. Host it somewhere your team can access:
   - A GitHub/GitLab gist (use the "raw" URL)
   - Your team's internal docs repo
   - Any HTTPS-accessible URL
3. Add one line to your team's onboarding docs:
   ```
   srepd config --preset https://your-url/srep-preset.yaml
   ```
