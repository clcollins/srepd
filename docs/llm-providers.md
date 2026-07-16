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
| `provider` | `string` | Yes | Provider name. One of: `ollama`, `anthropic`, `anthropic-vertex`, `anthropic-bedrock`, `openai`, `ramalama`. |
| `endpoint` | `string` | No | API endpoint URL. Each provider has a default (see below). |
| `model` | `string` | No | Model identifier. Each provider has a default (see below). |
| `api_key_env` | `string` | No | Name of the environment variable containing the API key. The key itself is never stored in config. Optional for all providers. Ignored by `anthropic-vertex` and `anthropic-bedrock` (they authenticate via the cloud provider's SDK). |
| `region` | `string` | No | Cloud region. Used by `anthropic-vertex` only. For `anthropic-bedrock`, the region comes from the AWS SDK environment (`AWS_REGION` / `~/.aws/config`), **not** from this field. |
| `project_id` | `string` | No | Google Cloud project ID. Used by `anthropic-vertex` only. |

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

### anthropic-bedrock

Anthropic Claude models via AWS Bedrock, using the official Go SDK's Bedrock
transport.

| Setting | Default |
|---------|---------|
| Endpoint | AWS SDK default (Bedrock runtime for the resolved region) |
| Model | `us.anthropic.claude-sonnet-4-6` |
| Credentials | AWS SDK default credential chain (see below) |
| Region | AWS SDK default (`AWS_REGION` / `~/.aws/config`) |
| Protocol | Anthropic SDK (Messages API) over Bedrock |

```yaml
llm_api:
  provider: anthropic-bedrock
  model: us.anthropic.claude-sonnet-4-6
```

Or, to accept the default model, just:

```yaml
llm_api:
  provider: anthropic-bedrock
```

**Credentials.** SREPD puts **no** AWS credentials in its config. The Bedrock
provider authenticates entirely through the **AWS SDK default credential
chain** — the same sources every AWS tool uses, in order:

1. Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
   `AWS_SESSION_TOKEN` (for temporary/STS credentials), `AWS_PROFILE`
2. Shared config/credentials files: `~/.aws/credentials`, `~/.aws/config`
3. SSO, assumed roles, and EC2/ECS instance metadata (IMDS)

You must also have a **region** set (`AWS_REGION` / `AWS_DEFAULT_REGION`, or a
profile default in `~/.aws/config`) — `llm_api.region` is *not* read for
Bedrock. In practice, export your AWS environment before launching SREPD:

```bash
export AWS_PROFILE=your-profile
export AWS_REGION=us-east-2
srepd
```

Verify your credentials and region work before configuring SREPD:

```bash
aws bedrock list-foundation-models --query 'modelSummaries[0].modelId'
```

**One-time model enablement.** Before Anthropic models can be invoked in a
given AWS account, you must enable them once. Sign in to the AWS Console,
open the Bedrock model catalog for Anthropic —
`https://console.aws.amazon.com/bedrock/home#/model-catalog?providerName=anthropic`
— and submit the one-time enablement/enrollment request for the Claude
model(s) you intend to use. Until this is done, invocations fail with an
access error even though credentials are valid.

**Model IDs — use an inference profile, not a bare model ID.** Current
Anthropic Claude models on Bedrock are *inference-profile-only*: they do not
support on-demand throughput by their bare foundation-model ID (e.g.
`anthropic.claude-sonnet-4-6-20250514-v1:0`). You must use a cross-region
inference profile ID, which is region-prefixed — `us.` for US regions,
`global.` for cross-region routing:

```
us.anthropic.claude-sonnet-4-6
```

SREPD's default is already an inference-profile ID
(`us.anthropic.claude-sonnet-4-6`), so the minimal config above works
out of the box in US regions. If you set `model:` yourself, use a profile ID.

To list the inference profiles available in your account:

```bash
aws bedrock list-inference-profiles \
  --query 'inferenceProfileSummaries[?contains(inferenceProfileId, `anthropic`)].inferenceProfileId' \
  --output table
```

To check which models (if any) support **on-demand** invocation by their bare
ID — i.e. those that do *not* require an inference profile:

```bash
aws bedrock list-foundation-models --by-inference-type ON_DEMAND \
  --query 'modelSummaries[].modelId' --output table
```

Note: at the time of writing, no Anthropic Claude models appear in the
`ON_DEMAND` list — they are all inference-profile-only — which is exactly why
the default model is a `us.`-prefixed profile ID.

### anthropic-vertex

Anthropic Claude models via Google Vertex AI, using the official Go SDK's
Vertex transport.

| Setting | Default |
|---------|---------|
| Endpoint | Google SDK default (Vertex for the resolved region) |
| Model | `claude-sonnet-4-6` |
| Credentials | Google application default credentials (ADC) |
| Region | `llm_api.region`, or `CLOUD_ML_REGION` / `VERTEXAI_LOCATION` |
| Project | `llm_api.project_id`, or `ANTHROPIC_VERTEX_PROJECT_ID` / `VERTEXAI_PROJECT` / `GOOGLE_CLOUD_PROJECT` |
| Protocol | Anthropic SDK (Messages API) over Vertex |

```yaml
llm_api:
  provider: anthropic-vertex
  region: us-east5
  project_id: my-gcp-project
```

Unlike Bedrock, Vertex **does** read `region` and `project_id` from config
(falling back to the environment variables listed above). Provider creation
fails with a clear error if neither config nor environment supplies a region
and project ID. Credentials come from Google application default credentials
(e.g. `gcloud auth application-default login`, or a service-account key
referenced by `GOOGLE_APPLICATION_CREDENTIALS`).

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
