# AI Agents

SREPD integrates AI assistance for incident analysis through two complementary agents and an ambient watcher system.

## Quickstart

1. Configure an LLM provider in `~/.config/srepd/srepd.yaml`:

```yaml
llm_api:
  provider: ollama
  model: llama3.1:8b
```

2. Launch srepd, select an incident, and press `:` then type:

```
:watcher what's wrong with this cluster?
```

3. The response appears in the watcher pane below the incident table.

For the CLI agent (requires [Claude Code](https://claude.ai/download) or another CLI agent):

```
:agent investigate this alert
```

## Architecture

SREPD provides two distinct AI surfaces:

| Surface | Command | Backend | Context | Use Case |
|---------|---------|---------|---------|----------|
| **CLI Agent** | `:agent` | Subprocess (`agent_cli_command`) | Stdin + env vars | Interactive investigation with tool access |
| **LLM Watcher** | `:watcher` | LLM API (`llm_api` provider) | API prompt | Data analysis, pattern synthesis; tool investigation with Anthropic providers |
| **Ambient Watcher** | (automatic) | LLM API (`llm_api` provider) | API prompt | Cross-incident pattern detection; tool investigation with Anthropic providers |

Both surfaces share the same incident context via `buildWatcherContext` and display responses in the watcher pane with source-specific markers.

### Persistent sessions (Claude Code)

When `agent_session_enabled: true` and the configured `agent_cli_command`
resolves to `claude`, `:agent` uses a persistent per-incident session
instead of one-shot subprocess invocation. This means:

- **Conversational context** carries across `:agent` queries for the same
  incident. The agent remembers what you asked before.
- **Tool activity is visible** during execution. Lines like `⚙ Bash ls -la`
  and `⚙ Read /var/log/...` appear in the watcher pane as the agent works.
- **LRU process management** keeps at most `agent_max_sessions` (default 3)
  Claude Code processes alive. When you switch incidents, the oldest session
  is suspended and resumed transparently via `--resume` on next access.
- **Markdown rendering** via glamour is applied to the final agent response.

**Default: `true`.** Session metadata (incident ID, session UUID, timestamps)
is stored locally in `~/.config/srepd/sessions/index.jsonl` (XDG-aware). No
query content or responses are persisted by srepd — only the mapping needed to
resume sessions. Set `agent_session_enabled: false` to revert to one-shot mode.

Non-Claude CLIs fall back to the one-shot blocking path automatically.

## Commands

### `:agent <query>` / `:agent` (chat mode)

Dispatches a query to the configured CLI agent subprocess. The agent command is configurable via `agent_cli_command` (default: `claude --print`).

```
:agent what's wrong with this cluster?
:agent suggest investigation steps for this alert
:agent what oc commands should I run?
```

`:agent <query>` dispatches the query and returns to the incident queue
immediately (fire-and-return). The response appears in the watcher pane.

Bare `:agent` (no query) opens **chat mode**: a focused pane for interactive
conversation with the agent. Type messages and press `Enter` to send; `Esc`
returns to the incident queue.

When a background session produces output while you are not in chat mode, the
status bar shows `[agent has new output]`. Entering chat mode clears the badge.

The query, system prompt, and full incident context are piped to the subprocess via stdin. PagerDuty environment variables are also set on the process.

### `:watcher <query>`

Queries the configured LLM API provider directly with rich incident context. Requires `llm_api` to be configured and healthy.

```
:watcher analyze the service logs for this cluster
:watcher is this a known issue?
:watcher what's the relationship between these incidents?
```

The watcher sends the selected incident's full context including alerts, cluster info from OCM, service logs, limited support history, notes, and the complete incident queue.

### Ambient Analysis

When the watcher pane is active and an LLM provider is configured and healthy, SREPD automatically analyzes the incident queue for patterns. Heuristic detectors identify:

| Detector | Threshold | Description |
|----------|-----------|-------------|
| Service storm | 3+ incidents | Multiple incidents on the same PagerDuty service |
| Cluster storm | 2+ incidents | Multiple incidents involving the same cluster ID |
| Urgency shift | 3+ high | Majority of incidents are high urgency |

When a pattern is detected, the observation is sent to the LLM for natural-language synthesis. If the LLM is unavailable, the raw heuristic text is shown instead. Observations are deduplicated with a 5-minute cooldown.

### Tool-Assisted Investigation

When the configured LLM provider is an Anthropic-family provider (`anthropic`, `anthropic-bedrock`, `anthropic-vertex`), the watcher uses tool-assisted investigation instead of plain synthesis. During investigation, the watcher can call read-only tools to fetch live PagerDuty and OCM data:

| Source | Available Tools |
|--------|----------------|
| PagerDuty | `get_incident`, `get_alerts`, `get_notes`, `list_queue` |
| OCM | `get_cluster_info`, `get_service_logs`, `get_limited_support` |

Investigation is bounded by `watcher_max_tool_turns` (default 6) and `watcher_investigation_timeout` (default 90s). Each investigation round may invoke multiple tools before producing a final synthesis.

The policy engine controls which tools can execute. The `ai_permission_mode` setting determines the mode:

| Mode | Behavior |
|------|----------|
| `plan` | Read-only tools only |
| `interactive` | Read tools allowed automatically; write tools are not executed (deferred to plan 415) |
| `auto` | Tools on the `ai_auto_allow_tools` allowlist execute without prompting |
| `custom` | Fully user-defined allowlist via `ai_auto_allow_tools` |

Non-Anthropic providers (`ollama`, `openai`, `ramalama`) do not support tool use and fall back to synthesis-only mode automatically. See [LLM Providers](llm-providers.md) for provider-level tool support details.

## Watcher Pane

The watcher pane appears below the incident table. Toggle visibility with `w`.

- **Layout**: Dynamically splits vertical space — minimum 10 rows for the table, minimum 5 for the watcher, 2/3 table and 1/3 watcher when space allows
- **Scrolling**: Mouse wheel scrolls the watcher pane content
- **Markers**: Each line is prefixed with a source marker (see Markers below)
- **Typewriter**: Responses display word-by-word for a live typing effect
- **Word wrap**: Long lines wrap at the pane width

### Footer Status

When the watcher pane is visible, the footer shows provider status:

```
Watching for updates...                    [AI Watcher] | ollama | healthy | idle
```

During a query, the status shows a countdown timer:

```
Watching for updates...                    [AI Watcher] | ollama | healthy | ⠋ analyzing... 57s
```

## Context

Both `:agent` and `:watcher` receive the same incident context:

| Data Source | Included |
|-------------|----------|
| Selected incident | Title, ID, service, status, urgency |
| Alerts | Alert names, SOP/runbook links, cluster IDs |
| OCM cluster info | Display name, state, region, cloud provider, version |
| Service logs | Up to 5 recent logs with severity and summary |
| Limited support | All LS reasons with summaries |
| Notes | Up to 5 recent notes (truncated to 300 chars) |
| Incident queue | All incidents with ID, title, service, urgency |

Context is pulled from the incident cache (populated by the OCM enrichment pipeline), not from the manually-loaded selected incident data.

## Configuration

### Agent session keys

| Key | Default | Description |
|-----|---------|-------------|
| `agent_session_enabled` | `true` | Use persistent per-incident sessions (Claude Code only). Session metadata is stored locally in `~/.config/srepd/sessions/index.jsonl` (XDG-aware). Set to `false` for one-shot mode. |
| `agent_max_sessions` | `3` | Max live Claude Code processes before LRU eviction |
| `agent_allowed_tools` | (empty) | Comma-separated Claude Code tool allowlist (e.g. `Bash,Read`) |
| `agent_permission_mode` | (empty) | Passed as `--permission-mode` to Claude Code |

To disable persistent sessions and use one-shot mode:

```yaml
agent_session_enabled: false
```

### Tool investigation keys

| Key | Default | Description |
|-----|---------|-------------|
| `watcher_max_tool_turns` | `6` | Maximum tool-use turns per watcher investigation. Values ≤ 0 are clamped to 6. |
| `watcher_investigation_timeout` | `90s` | Timeout for a single watcher investigation |
| `ai_permission_mode` | `interactive` | AI tool policy mode: plan (read-only), interactive (reads allowed, writes ask), auto (per allowlist), custom |
| `ai_auto_allow_tools` | (empty) | Tool names auto-allowed in auto/custom mode |
| `ai_allowed_command_prefixes` | (empty) | Command prefixes allowed in auto mode (reserved for phase 415) |

### System Prompts

Both agents have configurable system prompts:

```yaml
agent_system_prompt: "You are in read-only investigation mode for SRE PagerDuty incident triage. Suggest commands for the user to run if changes are needed. Do not modify cluster state."

watcher_system_prompt: "You are an SRE assistant with access to PagerDuty incident data and OpenShift cluster information. Provide concise, actionable analysis. Do not suggest destructive commands."
```

## Markers

Responses are prefixed with source markers on every non-blank line:

| Source | Emoji (`emoji: true`) | Text (`emoji: false`) |
|--------|----------------------|----------------------|
| CLI Agent | 🤖 | ☻ |
| LLM Watcher | 📡 | ☺ |
| Flags | 🚩 | \|► |

Set `emoji: false` in config for terminals without emoji support.

## Health Checks

The LLM provider health is checked every 60 seconds via the provider's health endpoint:

| Provider | Health Endpoint |
|----------|----------------|
| `ollama` | `GET /api/tags` |
| `openai` | `GET /v1/models` |
| `ramalama` | `GET /v1/models` |
| `anthropic` | (no health check) |

Health status is shown in the watcher footer. The `:watcher` command will show an error flash if the provider is offline.

## Timeouts

| Operation | Timeout |
|-----------|---------|
| CLI agent query | 60 seconds |
| Watcher LLM query | 60 seconds |
| Watcher tool investigation | 90 seconds (configurable via `watcher_investigation_timeout`) |
| Ambient synthesis | 30 seconds |
| Health check | 10 seconds |

A countdown timer is shown in the footer during active queries.

## Privacy

When using a remote provider (`anthropic`, `openai` pointed at a cloud endpoint), incident data including titles, service names, alert names, and cluster IDs is sent over the network. Use a local provider (`ollama`, `ramalama`) to keep all data on your machine.

With persistent sessions enabled (`agent_session_enabled: true`, the default),
srepd writes a session index to `~/.config/srepd/sessions/index.jsonl`
(XDG_CONFIG_HOME-aware). Each line records an incident ID, session UUID, and
timestamps — no query content or responses. This file is created `0600` inside
a `0700` directory. Delete it to clear session history; srepd will recreate it
on next use. Claude Code additionally maintains its own session state in
`~/.claude/`, which may contain incident context from prior queries. Both
stores are local to your machine but persist across srepd restarts. Claude Code
sessions are tied to your Anthropic account.

See [LLM Providers](llm-providers.md) for provider setup details.
