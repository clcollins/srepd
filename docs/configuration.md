# Configuration Reference

## CLI Arguments

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--debug` | `-d` | bool | `false` | Enable debug logging |
| `--dev` | `-D` | bool | `false` | Run with fixture data (no PagerDuty connection) |
| `--fixtures-dir` | `-F` | string | `testdata/fixtures` | Path to fixture data directory for dev mode |
| `--version` | | | | Print version and git SHA |

### Commands

| Command | Description |
|---------|-------------|
| `srepd` | Start the TUI |
| `srepd config` | Interactive configuration wizard |
| `srepd update` | Update to the latest release in place |

## Config File Reference

SREPD reads configuration from `~/.config/srepd/srepd.yaml` and supports the `SREPD_` environment variable prefix (e.g., `SREPD_TOKEN`). Run `srepd config` to create or update your config interactively.

### Required Keys

| Key | Type | Description |
|-----|------|-------------|
| `token` | `string` | PagerDuty API OAuth token |
| `teams` | `[]string` | PagerDuty team IDs to filter incidents |

### Optional Keys

#### General

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `editor` | `string` | `vim` | Editor for incident notes |
| `terminal` | `string` | `gnome-terminal --` | Terminal emulator for cluster login |
| `cluster_login_command` | `string` | `ocm backplane login %%CLUSTER_ID%%` | Cluster login command. Supports `%%CLUSTER_ID%%` and `%%INCIDENT_ID%%` placeholders. |
| `toolbox_mode` | `string` | `auto` | Toolbox detection: `auto`, `true`, or `false` |
| `chord_prefix` | `string` | `ctrl+x` | Prefix key for chord commands |
| `emoji` | `bool` | `true` | Use emoji markers (🚩 🤖 📡) or text fallbacks (\|► ☻ ☺) |

#### Escalation Policies

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `default_silent_escalation_policy` | `string` | (none) | Silent escalation policy ID for silencing incidents. Set via `srepd config`. |
| `custom_service_escalation_policies` | `map[string]string` | (none) | Per-service silent policy overrides (service ID to policy ID) |

#### Flag Conditions

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `flag_marker` | `string` | `🚩 ` | Prefix marker for flagged incidents (alt: \|►). Overridden by `emoji` setting. |

See [Flag Conditions](flag-conditions.md) for usage.

#### AI Agents

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `agent_cli_command` | `string` | `claude --print` | CLI command for `:agent` queries. Parsed with `strings.Fields`. |
| `agent_system_prompt` | `string` | (read-only investigation prompt) | System prompt piped to the CLI agent via stdin. |
| `watcher_system_prompt` | `string` | (SRE assistant prompt) | System prompt for `:watcher` LLM queries and ambient analysis. |

See [AI Agents](ai-agents.md) for usage.

#### LLM API

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `llm_api.provider` | `string` | (none) | LLM provider: `ollama`, `anthropic`, `openai`, `ramalama` |
| `llm_api.endpoint` | `string` | (provider default) | API endpoint URL |
| `llm_api.model` | `string` | (provider default) | Model identifier |
| `llm_api.api_key_env` | `string` | (none) | Name of env var containing API key (optional for all providers) |

See [LLM Providers](llm-providers.md) for provider-specific setup.

#### Colors

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `colors` | `map[string]string` | (defaults) | Custom color scheme with hex values |

Available color keys: `text`, `border`, `highlight`, `selected`, `warning`, `error`, `muted`, `tab`.

```yaml
colors:
  text: "#778da9"
  border: "#415a77"
  highlight: "#ffffff"
  selected: "#415a77"
  warning: "#a4133c"
  error: "#0d1b2a"
  muted: "#5C5C5C"
```

### Example Config

```yaml
token: <PagerDuty API token>
teams:
  - <team ID>

# Terminal and login
terminal: ptyxis
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%
toolbox_mode: auto

# Escalation policies
default_silent_escalation_policy: PXXXXXX
custom_service_escalation_policies:
  SVCID1: POLID1

# AI integration
agent_cli_command: toolbox run -c devtools claude --print
emoji: true

llm_api:
  provider: ollama
  endpoint: http://zaphod.collins.is:11434
  model: llama3.1
```

### Environment Variables

All config keys can be set via environment variables with the `SREPD_` prefix:

```bash
SREPD_TOKEN=<token> srepd
SREPD_DEBUG=true srepd
```

### Logging

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `log_to_journal` | `bool` | `true` | Log to systemd journal on Linux (falls back to file) |

Log destinations by platform:

| Platform | With journal | Without journal |
|----------|-------------|----------------|
| Linux | `journalctl --user -t srepd` | `/var/log/srepd.log` |
| macOS | n/a | `~/Library/Logs/srepd.log` |
| Other | n/a | stderr |
