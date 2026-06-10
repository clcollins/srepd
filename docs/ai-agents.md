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
| **LLM Watcher** | `:watcher` | LLM API (`llm_api` provider) | API prompt | Data analysis, pattern synthesis |
| **Ambient Watcher** | (automatic) | LLM API (`llm_api` provider) | API prompt | Cross-incident pattern detection |

Both surfaces share the same incident context via `buildWatcherContext` and display responses in the watcher pane with source-specific markers.

## Commands

### `:agent <query>`

Dispatches a query to the configured CLI agent subprocess. The agent command is configurable via `agent_cli_command` (default: `claude --print`).

```
:agent what's wrong with this cluster?
:agent suggest investigation steps for this alert
:agent what oc commands should I run?
```

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

## System Prompts

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
| Ambient synthesis | 30 seconds |
| Health check | 10 seconds |

A countdown timer is shown in the footer during active queries.

## Privacy

When using a remote provider (`anthropic`, `openai` pointed at a cloud endpoint), incident data including titles, service names, alert names, and cluster IDs is sent over the network. Use a local provider (`ollama`, `ramalama`) to keep all data on your machine.

See [LLM Providers](llm-providers.md) for provider setup details.
