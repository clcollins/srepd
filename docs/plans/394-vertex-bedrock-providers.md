# Plan 394: Add Vertex AI and AWS Bedrock providers for `:watcher`

**Branch:** `srepd/vertex-bedrock-providers`

## Problem

The `:watcher` LLM integration uses `ai.Provider` with four supported backends
(anthropic, ollama, openai, ramalama), but the `anthropic` provider only supports
direct API key auth. Users running Claude via Vertex AI (Google Cloud) or AWS
Bedrock cannot use `:watcher` — even though the `anthropic-sdk-go` library
already ships `vertex` and `bedrock` subpackages as transparent middleware.

## Solution

### Reuse `anthropicProvider` entirely

Both Vertex and Bedrock middleware produce a standard `anthropic.Client` — the
`Query`, `StreamQuery`, `SupportsStreaming`, and `Name` methods are identical.
Only the constructor differs (auth mechanism + URL rewriting). Added a `name`
field to `anthropicProvider` so each variant reports its own name.

### `anthropic-vertex` provider (`pkg/ai/vertex.go`)

- `vertex.WithGoogleAuth(ctx, region, projectID)` — uses Google ADC
- Auto-detects region from `CLOUD_ML_REGION` / `VERTEXAI_LOCATION`
- Auto-detects project from `ANTHROPIC_VERTEX_PROJECT_ID` / `VERTEXAI_PROJECT` /
  `GOOGLE_CLOUD_PROJECT`
- Config fields override env vars; missing both is an error
- Deferred recover converts SDK panics to errors

### `anthropic-bedrock` provider (`pkg/ai/bedrock.go`)

- `bedrock.WithLoadDefaultConfig(ctx)` — uses AWS default credential chain
- Region auto-detected via AWS SDK (env vars, shared config, IMDS)
- Bedrock model IDs use a different format
  (`anthropic.claude-sonnet-4-6-20250514-v1:0`)
- Deferred recover converts SDK panics to errors

### Config additions

- `llm_api.region` and `llm_api.project_id` fields in `ai.Config`
- Both read from viper in `cmd/root.go`

### Human-readable provider errors (`pkg/ai/classify.go`)

Provider API failures previously surfaced as the SDK's raw `Error()` string —
method, URL, and the full JSON response body — in a 4-second status flash.
Following the `pd.ClassifyAPIError` pattern:

- `ai.ClassifyProviderError(err)` extracts the HTTP status and the
  human-readable `error.message` from the response body. Handles the
  Anthropic (`{"error":{"message":...}}`), Google Cloud (same, sometimes
  wrapped in a single-element array), and AWS (`{"message":...}`) shapes,
  plus status-specific hints (401 auth, 403 permission, 404 model, 429 rate
  limit) and network/timeout classification.
- User-initiated `:watcher`/`:agent` query failures now route through
  `errMsg` → the full-screen error view (esc to dismiss) instead of a
  transient flash, so long actionable messages (e.g. Google's "enable this
  API at <url>") are actually readable.
- A stream superseded by a newer query ends with `context.Canceled`; that is
  now silently ignored instead of surfacing as an error.
- Ambient watcher synthesis errors keep their existing graceful degradation
  (raw observation appended to the pane) — no unprompted full-screen takeover.

Field-testing follow-ups (journal review):

- `errMsgHandler` no longer copies the error into the transient status line —
  the 15s incident poll overwrote it within a second; the error view (m.err)
  is the surface, logged once at ERROR.
- `View()` no longer logs the error on every render — at ~13 renders/second
  the full multi-line error was spammed to the journal hundreds of times
  while the error view was up.
- The provider's raw stream/query error (full JSON response body) is logged
  at DEBUG instead of WARN, so the journal carries one classified ERROR line
  per failure instead of the same failure twice (raw then classified).

### Honest provider health (tri-state)

The status line previously showed "healthy" for anthropic-family providers
that had never been verified: they don't implement `ai.HealthChecker`, and the
periodic check fabricated `healthy: true` for them — strictly false when every
query 403s. Replaced `aiHealthy bool` with a tri-state `aiHealth`:

- `unverified` — no probe endpoint and no completed query yet
- `healthy` — a probe passed or the last query succeeded
- `error` — a probe failed or the last query failed

Mechanics:

- New `ai.SupportsHealthCheck` helper (mirrors `SupportsStreaming`); the 60s
  probe job is only scheduled for probe-capable providers.
- Watcher query/stream/synthesis outcomes update `aiHealth` reactively — a
  403 flips the line to `error`, the next success flips it back.
- Status label is now `healthy`/`error`/`unverified` ("offline" was dropped:
  it was also untrue for a 403, where the service is up but the config is
  wrong).
- Query gating: probe-capable providers verified down still block queries;
  probe-less providers always allow retries (a successful query is the only
  signal that can recover them).
- Health flips to healthy on the FIRST streamed token, not at end of stream —
  the first token is proof the provider answered.

### Startup-only stream timeouts (watchdog pattern)

The watcher stream previously passed a 60s `WithTimeout` context to
`StreamQuery`, truncating any response that streamed longer than 60s. Both
stream paths now use a cancelable context plus a `time.AfterFunc` watchdog
that fires only if NO token arrives before the deadline and is disarmed by
the first token:

- Watcher (`stream.go`): watchdog cancellation is distinguished from
  consumer cancellation via an atomic flag, so a genuine startup timeout
  surfaces as "no response from provider after 60s" instead of being
  swallowed as the superseded-stream `context.Canceled` case.
- Agent CLI (`claude.go`): fixes a latent flaw — the old between-lines
  context check could never fire for a CLI that produced zero output (the
  scanner blocks forever). The watchdog kills the process directly.
  `cmd.WaitDelay` (2s) force-closes the pipes so grandchild processes that
  inherited stdout cannot keep the stream alive after the kill; a deferred
  `cmd.Wait` reaps the process on all exit paths (no zombies on supersede).
- The "analyzing... Xs" countdown reflects only the startup watchdog; once
  elapsed (stream is responding, no deadline) the countdown is hidden
  instead of showing a hung-looking "0s".

## Files

- `pkg/ai/provider.go` — `Region`, `ProjectID` fields on `Config`
- `pkg/ai/anthropic.go` — `name` field, dynamic `Name()`
- `pkg/ai/vertex.go` (new) — Vertex provider constructor
- `pkg/ai/vertex_test.go` (new) — env resolution + error tests
- `pkg/ai/bedrock.go` (new) — Bedrock provider constructor
- `pkg/ai/bedrock_test.go` (new) — panic recovery + default model tests
- `pkg/ai/factory.go` — registry entries + switch cases
- `pkg/ai/factory_test.go` — validation + registry tests
- `pkg/ai/classify.go` (new) — provider error classification
- `pkg/ai/classify_test.go` (new) — shape extraction + classification tests
- `pkg/tui/tui.go` — watcher/agent error routing to the error view
- `pkg/tui/claude.go` — non-streaming agent error routing
- `cmd/root.go` — Region/ProjectID in llmCfg
- `README.md` — provider table + config keys

## New dependencies (transitive)

Importing the SDK's `vertex` and `bedrock` subpackages pulls in Google OAuth2
and AWS SDK v2 modules as indirect deps. These are org-backed, well-known
libraries already declared in `anthropic-sdk-go`'s go.mod.

## Tests

- `TestResolveVertexRegion_*` — config precedence, env fallback, empty
- `TestResolveVertexProjectID_*` — config precedence, three env fallbacks, empty
- `TestNewVertexProvider_MissingRegion` / `_MissingProjectID` — error messages
- `TestNewVertexProvider_AuthPanicRecovery` — SDK panic → error
- `TestNewBedrockProvider_AuthPanicRecovery` — SDK panic → error
- `TestValidateConfig_VertexValid` / `_BedrockValid` — registry presence
- `TestProviderRegistry_Defaults` — default models for both

`make test-all` green.
