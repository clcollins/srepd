# 059: LLM API Integration

**Issue**: #195
**Branch**: `srepd/llm-api-integration`
**Status**: In Progress

## Problem

Some queries are pure data analysis (summarize notes, classify alerts,
suggest investigation steps). Direct API calls are better than spawning
a CLI process for these use cases. Currently srepd has no LLM API
integration.

## Solution

Add a new `pkg/ai/` package with a Provider interface, an Anthropic SDK
implementation, a system prompt builder, and config integration. The
package is configurable via `srepd.yaml` with an optional `llm_api`
section. When unconfigured, AI features are disabled gracefully.

## Changes

### pkg/ai/client.go
- Define `Provider` interface with `Query`, `StreamQuery`, and `Name`
- Define `Config` struct for provider configuration
- Implement `NewProvider` factory that dispatches by provider name
- Validate config: check api_key_env resolves to a non-empty value

### pkg/ai/anthropic.go
- Implement `anthropicProvider` struct satisfying `Provider`
- Use `github.com/anthropic-ai/anthropic-sdk-go` Messages API
- `Query`: send system + user prompt, return concatenated text blocks
- `StreamQuery`: stream response tokens via channel
- `Name`: return "anthropic"

### pkg/ai/prompts.go
- `BuildSystemPrompt`: format incident context (title, ID, service,
  status, urgency, cluster ID, alert count, alert names) into a
  system prompt for SRE analysis
- Read-only safety instruction: do not suggest destructive commands

### pkg/ai/client_test.go
- `TestNewProvider_Anthropic` - creates Anthropic provider with valid config
- `TestNewProvider_Unknown` - returns error for unknown provider
- `TestNewProvider_MissingAPIKey` - returns error when env var is empty
- `TestBuildSystemPrompt` - includes incident context fields
- `TestBuildSystemPrompt_NilIncident` - handles nil incident gracefully
- `TestBuildSystemPrompt_EmptyAlerts` - handles empty alert slice
- `TestConfig_Validation` - validates required fields

### cmd/config.go
- Add `llm_api` to optional keys with validation
- Validate provider name is known, api_key_env resolves

### go.mod
- Add `github.com/anthropic-ai/anthropic-sdk-go` dependency

## Lessons Learned from Prior Plans

- Plan 046 (alert normalization): TDD approach with table-driven
  tests was effective; follow the same pattern here
- Plan 055 (Claude CLI): system prompt safety is critical; reuse the
  read-only investigation constraint
- Plan 032 (async writer): interface-based design enables clean
  testing with mocks

## Verification

1. `make fmt-check` passes
2. `make vet` passes
3. `make test` passes (all new tests green)
4. `make test-race` passes
5. `make lint` passes
6. `make plan-check` passes
7. `make readme-check` passes
