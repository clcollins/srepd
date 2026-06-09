# LLM Provider Configuration

SREPD supports pluggable LLM providers for AI-assisted incident analysis. The `llm_api` configuration block is entirely optional — when absent, all AI API features are disabled and SREPD functions normally.

## Configuration

Add an `llm_api` section to `~/.config/srepd/srepd.yaml`:

```yaml
llm_api:
  provider: ollama
  endpoint: http://localhost:11434
  model: llama3.1:8b
  api_key_env: ""
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | `string` | Yes | Provider name. One of: `ollama`, `anthropic`, `openai`, `ramalama`. |
| `endpoint` | `string` | No | API endpoint URL. Each provider has a default (see below). |
| `model` | `string` | No | Model identifier. Each provider has a default (see below). |
| `api_key_env` | `string` | No | Name of the environment variable containing the API key. The key itself is never stored in config. Optional for all providers. |

When `api_key_env` is set, SREPD reads the API key from that environment variable at startup. When empty or unset, no authentication header is sent — suitable for local, unauthenticated servers.

## Providers

### ollama

Local Ollama daemon. Privacy-preserving: all data stays on your machine.

| Setting | Default |
|---------|---------|
| Endpoint | `http://localhost:11434` |
| Model | `llama3.1:8b` |
| API Key | Not required |
| Protocol | Ollama REST API (`/api/chat`) |
| Health Check | `GET /api/tags` |

```yaml
llm_api:
  provider: ollama
```

**Setup:**

1. Install Ollama: https://ollama.com
2. Pull a model: `ollama pull llama3.1:8b`
3. Start the server: `ollama serve` (or let systemd manage it)

Alternatively, use [ollama-container](https://github.com/clcollins/ollama-container) to run Ollama in a pod via `podman kube play`. Point `endpoint` at wherever the container is listening (default `http://localhost:11434`).

**Remote Ollama:** To use an Ollama instance on another host, set `endpoint` to the remote URL:

```yaml
llm_api:
  provider: ollama
  endpoint: http://remote-host:11434
  model: mistral:7b
```

### anthropic

Anthropic Messages API via the official Go SDK.

| Setting | Default |
|---------|---------|
| Endpoint | SDK default (https://api.anthropic.com) |
| Model | `claude-sonnet-4-6` |
| API Key | Set via `api_key_env` |
| Protocol | Anthropic SDK (Messages API) |

```yaml
llm_api:
  provider: anthropic
  api_key_env: ANTHROPIC_API_KEY
```

**Setup:**

1. Get an API key from https://console.anthropic.com
2. Set the environment variable: `export ANTHROPIC_API_KEY=sk-ant-...`

**Custom endpoint:** For Anthropic API proxies or compatible services:

```yaml
llm_api:
  provider: anthropic
  endpoint: https://custom-proxy.internal.example.com
  api_key_env: ANTHROPIC_API_KEY
```

### openai

Any OpenAI-compatible `/v1/chat/completions` endpoint. This is the extensibility hook — use it for vLLM, text-generation-inference, internal model gateways, or any service that speaks the OpenAI chat completions protocol.

| Setting | Default |
|---------|---------|
| Endpoint | (none — must be specified) |
| Model | (none — must be specified) |
| API Key | Optional, sent as `Authorization: Bearer <key>` when set |
| Protocol | OpenAI `/v1/chat/completions` |
| Health Check | `GET /v1/models` |

```yaml
llm_api:
  provider: openai
  endpoint: https://api.openai.com
  model: gpt-4o
  api_key_env: OPENAI_API_KEY
```

**For unauthenticated local servers** (vLLM, TGI, etc.):

```yaml
llm_api:
  provider: openai
  endpoint: http://localhost:8000
  model: my-local-model
```

### ramalama

Red Hat RamaLama AI tool in server mode. Exposes an OpenAI-compatible endpoint.

| Setting | Default |
|---------|---------|
| Endpoint | `http://localhost:8080` |
| Model | (none — depends on your ramalama model) |
| API Key | Not required |
| Protocol | OpenAI-compatible `/v1/chat/completions` |
| Health Check | `GET /v1/models` |

```yaml
llm_api:
  provider: ramalama
  model: granite-code:8b
```

**Setup:**

1. Install RamaLama: https://github.com/containers/ramalama
2. Start the server: `ramalama serve --port 8080 granite-code:8b`

## Privacy Considerations

When using a remote provider (`anthropic`, `openai` pointed at a cloud endpoint), incident data is sent over the network. This includes:

- Incident title, ID, status, urgency
- Service name
- Alert names
- Cluster IDs (from alert details)

When using a local provider (`ollama`, `ramalama`, `openai` pointed at localhost), all data stays on your machine.

Choose your provider based on your organization's data handling requirements.

## Validation and Error Handling

- Invalid `provider` value: logged as a warning at startup, AI features disabled
- Missing `endpoint` for `openai` provider: provider creation fails, logged as warning
- Missing `api_key_env` environment variable: provider still created (key is optional), but authenticated endpoints will return 401 errors at query time
- Provider health check failure: does not block startup; health is checked on demand

## Watcher Integration

The configured LLM provider is used by the ambient watcher system for:

- **`:watcher` queries**: direct LLM analysis of incidents with full context
- **Pattern synthesis**: when heuristic detectors identify cross-incident patterns, the LLM synthesizes a natural-language observation
- **Health monitoring**: provider connectivity checked every 60 seconds, status shown in the footer

The watcher system prompt is configurable via `watcher_system_prompt` in the config file. See [AI Agents](ai-agents.md) for full usage documentation.

## Adding New Providers

The provider system is extensible. To add a new provider:

1. Implement the `ai.Provider` interface in a new file under `pkg/ai/`
2. Add the provider to the `providerRegistry` in `pkg/ai/factory.go`
3. Add a case to the `switch` in `NewProvider()`
4. Write tests using `httptest.NewServer` for HTTP-based providers
