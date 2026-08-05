# Plan 413: Tool registry, policy engine, and approvals strip

**Branch:** `srepd/ai-p3-tools-policy`

## Problem

The watcher's AI investigation loop can only synthesize text from incident
data already fetched by the TUI. It cannot call PagerDuty or OCM APIs on its
own to gather additional context (alerts, notes, cluster info, service logs,
limited-support reasons). Without tool use, the model's analysis is limited
to whatever the TUI happened to pre-fetch, and deeper investigation requires
manual user action.

Adding tool use naively would be unsafe: the model could invoke any tool at
any time with no user visibility or control. A policy layer is needed to
gate tool execution by class and mode, and a TUI component is needed to
surface pending approval requests to the user.

## Solution

### 1. Read-only tool layer (`pkg/ai/tools/`)

A `Registry` holding `Tool` structs, each with a name, description, class,
JSON schema, and handler function. Each tool wraps an existing PagerDuty or
OCM accessor method — no new API calls are introduced.

Seven tools total:

| Tool | Source | Description |
|------|--------|-------------|
| `get_incident` | PagerDuty | Fetch incident details by ID |
| `get_alerts` | PagerDuty | Fetch alerts for an incident |
| `get_notes` | PagerDuty | Fetch notes for an incident |
| `list_queue` | PagerDuty | List current incident queue |
| `get_cluster_info` | OCM | Fetch cluster metadata |
| `get_service_logs` | OCM | Fetch service log entries |
| `get_limited_support` | OCM | Fetch limited-support reasons |

Key design decisions:

- **`get_recent_events` omitted.** `pkg/delta` does not exist — plan 412
  did not build it. The tool was dropped rather than stubbed.
- **String results, not structured types.** Tool handlers return plain text
  that the LLM consumes directly. This avoids a marshalling layer between
  tool output and model input.
- **Output truncation at 8192 bytes.** Prevents runaway tool output from
  consuming the model's context window.
- **`formatError` drops raw error text.** Errors returned to the model say
  only that the tool failed, never the internal error message. This
  preserves the plan-101 token-leak property (no internal details in
  model-visible output).
- **Single gated exposure mode.** `GatedBetaTools()` checks policy before
  executing each handler. (An ungated `AsBetaTools()` was originally planned
  but removed in review — see post-mortem below.)

#### `registry.go`

`Registry` struct with `Register`, `GatedBetaTools`, and `Execute` methods.
Tools are exposed as `anthropic.BetaTool` values for the Anthropic SDK.

#### `handlers.go`

Handler functions for each of the seven tools. Each handler takes JSON input,
extracts parameters, calls the underlying PagerDuty or OCM accessor, and
returns a string result or formatted error.

#### `verdict.go`

Verdict extraction from the model's fenced JSON output. Parses the model's
response to extract a structured verdict (silent, noteworthy, or actionable)
from a code-fenced JSON block.

### 2. Pure policy engine (`pkg/ai/policy/`)

A stateless, no-I/O function: `Decide(cfg, toolName, class, input) -> Decision`.

Enums:

- **Class:** `Read`, `WriteLocal`, `Exec`, `External`
- **Mode:** `Plan`, `Interactive`, `Auto`, `Custom`
- **Decision:** `Allow`, `Deny`, `Ask`

Rules:

- `ClassRead` is always `Allow` in every mode.
- Empty tool name produces `Deny`.
- Unknown mode produces `Deny`.
- All decisions derive from config alone — no I/O, no side effects.

### 3. Approvals strip (`pkg/tui/approvals.go`)

A TUI component rendered above the bottom status bar when Ask decisions are
pending. Operations:

- **Add** — enqueue a pending Ask with its associated action callback.
- **Accept** — invoke the action callback exactly once, then remove.
- **Dismiss** — remove without invoking.
- **Render** — compact single-line summary when asks are pending.
- **RenderExpanded** — full detail view for inspection.

The strip is only visible when there are pending asks; it occupies zero
vertical space otherwise.

### 4. Watcher investigation loop (`pkg/tui/investigation.go`)

Uses the Anthropic SDK's `BetaToolRunner.NextMessage` for per-turn control
of the tool-use loop, rather than `Run()` which provides no policy gate
between turns.

`watcherInvestigateCmd` is a `tea.Cmd` that runs a bounded, tool-using
investigation:

- **Max tool turns:** configurable, default 6.
- **Timeout:** configurable, default 90 seconds.
- The loop calls `NextMessage`, checks each tool call against policy (via
  `GatedBetaTools`), collects `Ask` decisions for diagnostic tracking, and
  continues until the model emits a final response or bounds are hit.
  (Tool permission asks are not surfaced to the approvals strip — deferred
  to plan 415.)
- The model's final response is parsed for a fenced JSON verdict block
  containing a classification (silent, noteworthy, or actionable).

### 5. Non-Anthropic graceful degradation

When the AI provider is not Anthropic-family, the tool investigation path is
skipped entirely. The watcher falls back to synthesis-only mode (the pre-413
behavior). A one-time log line is emitted explaining why tool investigation
was skipped.

### 6. Config additions

| Key | Default | Description |
|-----|---------|-------------|
| `watcher_max_tool_turns` | `"6"` | Maximum tool-use turns per investigation |
| `watcher_investigation_timeout` | `"90s"` | Timeout for a single investigation run |
| `ai_permission_mode` | `"interactive"` | Policy mode: plan, interactive, auto, custom |
| `ai_auto_allow_tools` | (empty) | Tools auto-allowed in auto/custom mode |
| `ai_allowed_command_prefixes` | (empty) | Command prefixes allowed for exec-class tools |

## Testing

### Headline criterion

A Deny-everything policy config produces ZERO tool handler invocations. This
is verified by `TestHeadline_DenyEverything_ZeroHandlerInvocations` in
`pkg/ai/tools/integration_test.go`.

### Policy engine (`pkg/ai/policy/policy_test.go`)

- ClassRead always allowed in every mode.
- Empty tool name always denied.
- Unknown mode always denied.
- Each Mode x Class combination produces the correct Decision.
- Pure function: no mocks, no I/O, no setup.

### Tool registry (`pkg/ai/tools/registry_test.go`)

- Registration and lookup.
- `GatedBetaTools` returns correct `anthropic.BetaTool` values and enforces
  policy before handler invocation.
- Output truncation at 8192 bytes.
- Error formatting drops raw error text.

### Tool handlers (`pkg/ai/tools/handlers_test.go`)

- Each of the seven handlers called with valid input returns expected output.
- Each handler called with invalid input returns a formatted error (no raw
  error text).
- Handlers use mock PagerDuty and OCM clients.

### Verdict extraction (`pkg/ai/tools/verdict_test.go`)

- Parses silent, noteworthy, and actionable verdicts from fenced JSON.
- Handles missing or malformed verdict blocks.

### Approvals strip (`pkg/tui/approvals_test.go`)

- Add/Accept/Dismiss lifecycle.
- Accept invokes action exactly once.
- Render returns empty string when no asks pending.
- RenderExpanded shows full detail.

### Investigation loop (`pkg/tui/investigation_test.go`)

- Bounded by max tool turns.
- Bounded by timeout.
- Policy deny stops tool execution.
- Verdict extracted from model response.

### Integration (`pkg/ai/tools/integration_test.go`)

- Headline test: Deny-everything config, zero handler invocations.
- End-to-end registry + policy + handler flow.

## Files created

| File | Purpose |
|------|---------|
| `pkg/ai/policy/policy.go` | Pure policy engine |
| `pkg/ai/policy/policy_test.go` | Policy decision tests |
| `pkg/ai/tools/registry.go` | Tool registry and schema exposure |
| `pkg/ai/tools/registry_test.go` | Registry tests |
| `pkg/ai/tools/handlers.go` | Tool handler implementations |
| `pkg/ai/tools/handlers_test.go` | Handler tests |
| `pkg/ai/tools/verdict.go` | Verdict extraction from model output |
| `pkg/ai/tools/verdict_test.go` | Verdict parsing tests |
| `pkg/ai/tools/integration_test.go` | Headline + integration tests |
| `pkg/tui/approvals.go` | Approvals strip TUI component |
| `pkg/tui/approvals_test.go` | Approvals strip tests |
| `pkg/tui/investigation.go` | Watcher investigation loop |
| `pkg/tui/investigation_test.go` | Investigation loop tests |

## Files modified

| File | Change |
|------|--------|
| `go.mod`, `go.sum` | SDK upgrade v1.57.0 to v1.61.0 |
| `pkg/ai/anthropic.go` | Added `BetaMessages()` method |
| `pkg/config/config.go` | New config keys and defaults |
| `pkg/tui/model.go` | Tool registry, approvals strip, investigation config fields; `initToolRegistryForModel` wiring |
| `pkg/tui/tui.go` | `investigationMsg` handler in Update loop |
| `pkg/tui/views.go` | Approvals strip rendering above status bar |
| `pkg/tui/watcher.go` | Tool investigation path in `runDetectors` |

## Deviations

- **`get_recent_events` omitted:** The original design included this tool,
  but `pkg/delta` was never built (plan 412 did not include it). Rather than
  stub a non-functional tool, it was dropped entirely. It can be added when
  `pkg/delta` lands.
- **`BetaToolRunner.NextMessage` instead of `Run()`:** The SDK's `Run()`
  method executes the entire tool-use loop with no hook between turns. This
  makes it impossible to check policy or surface Ask decisions mid-loop.
  `NextMessage` gives per-turn control at the cost of managing the loop
  manually.
- **Policy is pure (no I/O):** All decisions derive from the config struct
  passed in. There is no database, no file read, no network call in the
  policy engine. This makes it trivially testable and eliminates any
  possibility of policy decisions depending on external state that could
  drift.
- **String results from tool handlers:** Tool handlers return `string`, not
  structured Go types. The LLM consumes the text directly, so marshalling
  to a struct and back would add complexity with no benefit.
- **`formatError` drops raw error text entirely:** Rather than sanitizing
  error messages (which is fragile), the error formatter drops all internal
  detail. The model sees only that the tool failed, never why. This is
  stricter than necessary for read-only tools but maintains the plan-101
  invariant without case-by-case analysis of what each error might contain.

## Post-mortem / Lessons learned

### SDK beta surface

The Anthropic Go SDK's tool runner (`BetaToolRunner`, `BetaTool`) is a
beta API surface. The `Beta.Messages` field requires a pointer receiver
for `NewToolRunner` — `factory := client.Beta.Messages` fails silently at
compile time; must use `factory := &client.Beta.Messages`. Pin the SDK
version in go.mod and expect API churn.

### Truncate boundary panic

`Truncate(s, maxBytes)` panicked when `maxBytes` was smaller than the
`[truncated]` marker length. Any function that subtracts a fixed offset
from a user-controlled size must guard both zero and underflow. Caught
by a fuzz-style test, not by production use.

### formatError must not discard diagnostics entirely

The original `formatError` returned a bare message string and threw away
the error. This made production debugging impossible — no log, no class,
nothing. Fix: log the full error at Debug level and append a classified
error category (timeout, auth error, not found, network error, request
failed) to the user-visible string. The raw error text stays out of
tool results (plan-101 token-leak property), but operators can still
diagnose failures from debug logs.

### AsBetaTools ungated bypass

`AsBetaTools()` returned tools without policy gating — any caller using
it instead of `GatedBetaTools()` would bypass the entire policy engine.
With zero production callers it was dead code, but dead code that
*could* be called is a latent policy bypass. Deleted entirely. The
revert-check test now exercises `GatedBetaTools` with an Allow decision
instead.

### Production-path tests need real I/O boundaries

The headline test for `watcherInvestigateCmd` uses `httptest.NewServer`
to stand up a fake Anthropic API, exercising the real SDK HTTP client,
JSON serialization, and tool runner loop — not just the pure parsing
layer. Without this, the watcher's investigation path was tested only
at the parse level, and wiring bugs (wrong pointer receiver, missing
context propagation) would have shipped undetected.

### fullText field was set but never read

`investigationMsg.fullText` accumulated the raw model output but no
consumer ever read it. The verdict (parsed from that text) was the only
value used downstream. Removed the field to avoid confusion about
whether downstream code should be using it.

### AskToolPermission surfacing deferred to plan 415

The `toolAsks` → `AskToolPermission` loop in the `investigationMsg`
handler built asks with no `Action` callback. `approvalsStrip.Accept`
returned `nil` when `Action == nil`, so accepting a tool permission ask
silently did nothing. The one-shot-allow flow needed to make these
useful was never built. Rather than ship a button that lies (honest-flags
principle, 410a §3f), the surfacing was removed entirely. The `onAsk`
collector in `investigation.go` still tracks ask-class decisions for
diagnostic purposes and future use by plan 415.

### Hardcoded model fallback broke Bedrock users

`watcherInvestigateCmd` received `""` as the model argument from
`runDetectors` and silently substituted `"claude-sonnet-4-6"` — a bare
model ID that cannot be invoked on Bedrock (which requires the
inference-profile ID `us.anthropic.claude-sonnet-4-6`). Fixed by adding
a `ModelReporter` optional interface to `ai.Provider`, plumbing the
resolved model through `runDetectors`, and returning an error on empty
model instead of guessing.
