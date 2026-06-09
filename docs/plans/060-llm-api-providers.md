# Plan: Configurable LLM API Provider System

**Issue:** #195
**Informed by:** #305
**Branch:** `srepd/llm-api-providers`

## Goal

Add a configurable, extensible LLM API provider interface (`pkg/ai`) to srepd
for AI-assisted incident analysis. Support Ollama, Anthropic, OpenAI-compatible,
and RamaLama providers via a unified `Provider` interface.

## Design Decisions

1. **ollama-container is not a separate provider** — same API as ollama, users
   point `endpoint` at their container. No lifecycle management in srepd.
2. **ramalama wraps openai-compatible** — `ramalama serve` exposes OpenAI `/v1`
   endpoint. Thin wrapper with ramalama-specific defaults.
3. **API keys are always optional** — supports unauthenticated local servers and
   authenticated cloud APIs uniformly.
4. **Provider defaults in registry** — `providerRegistry` map stores default
   endpoint and model per provider name.
5. **Health checks via optional interface** — `HealthChecker` interface, providers
   opt in via implementation.

## Files Changed

### New files (pkg/ai/)
- `provider.go` — Provider, HealthChecker interfaces, Config struct
- `factory.go` — NewProvider factory, ValidateConfig, providerRegistry
- `anthropic.go` — Anthropic SDK provider
- `ollama.go` — Direct HTTP to Ollama REST API
- `openai_compat.go` — HTTP to OpenAI-compatible /v1 endpoints
- `ramalama.go` — Wrapper around openaiCompatProvider
- `prompts.go` — SRE system prompt builder
- `mock.go` — MockProvider for testing
- `*_test.go` — Tests for each file

### Modified files
- `cmd/root.go` — Create AI provider from Viper config, pass to InitialModel
- `cmd/config.go` — Pass nil aiProvider in config mode
- `pkg/tui/model.go` — Add aiProvider field, update InitialModel signatures
- `README.md` — LLM Integration quick-start section
- `go.mod` / `go.sum` — Add anthropic-sdk-go dependency

### New documentation
- `docs/llm-providers.md` — Full provider reference

## Testing

- 40 tests in pkg/ai covering all providers
- HTTP providers tested with httptest.NewServer
- Race detection passes
- Full project test suite passes
- golangci-lint clean
