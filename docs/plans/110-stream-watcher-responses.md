# Plan 110: Live-stream `:watcher` responses, gated on provider capability

**Branch:** `srepd/stream-watcher-responses`

## Problem

`:watcher` LLM queries called the blocking `provider.Query`, returning the whole
response at once and then *faking* a typing animation via the typewriter after the
fact. The user gets no feedback while the model is actually generating. The AI
providers already implement `StreamQuery`, but nothing in the TUI used it.

The user also noted that **not all providers support streaming reliably**, so
streaming must be **toggled on provider capability** (and user config), not assumed.

## Solution

### Capability gate (`pkg/ai`)

- New optional `StreamingProvider` interface (`SupportsStreaming() bool`) and a
  `SupportsStreaming(p Provider) bool` helper (false for nil / non-implementers /
  opt-outs), mirroring the existing optional `HealthChecker` pattern.
- All four shipped providers (anthropic, openai-compat, ollama, ramalama) advertise
  streaming; `MockProvider` gains a configurable `Streaming` field.

### Streaming command (`pkg/tui/stream.go`)

- `streamWatcherCmd` runs `provider.StreamQuery` in a background goroutine, forwarding
  each token (and a terminal done/err event) onto a channel. It returns
  `watcherStreamStartedMsg{ch, cancel}`.
- `readStreamCmd(ch)` is a channel-draining `tea.Cmd`: it blocks (off the Update loop)
  for the next event and yields `watcherStreamChunkMsg` or `watcherStreamDoneMsg`. The
  chunk handler re-issues it, draining one event per Update tick. **No `*tea.Program`
  handle needed** — this fits the existing single-threaded Update architecture and
  never blocks the loop.

### Update-loop wiring (`pkg/tui/tui.go`, `pkg/tui/model.go`)

- `watcherPromptMsg` branches: when `m.streamResponses && ai.SupportsStreaming(provider)`
  → `streamWatcherCmd`; otherwise fall back to the blocking `watcherQueryCmd` +
  typewriter.
- New Update cases: `watcherStreamStartedMsg` (cancel any prior stream, append a fresh
  buffer entry, start draining), `watcherStreamChunkMsg` (accumulate into
  `watcherStreamPartial`, `SetLast` in place, re-drain), `watcherStreamDoneMsg`
  (finalize / flash error). A new query cancels the in-flight stream.
- Model fields: `streamResponses`, `watcherStreamPartial`, `watcherStreamCancel`.

### Config

- New optional key `stream_responses` (default true) via `resolveStreamResponses()`
  (uses `viper.IsSet` so the default is true). Registered in
  DefaultOptionalKeys/OptionalKeys, the `--debug` safe-to-log allowlist, and the README.

## Files Modified

- `pkg/ai/provider.go`, `mock.go`, `anthropic.go`, `ollama.go`, `openai_compat.go`,
  `ramalama.go`, `provider_test.go` — capability interface + advertisements.
- `pkg/tui/stream.go`, `stream_test.go` — streaming command + drain.
- `pkg/tui/tui.go`, `model.go` — Update wiring + model fields + config resolution.
- `pkg/tui/watcher_integration_test.go` — Update-level streaming tests.
- `pkg/config/config.go`, `cmd/config.go`, `README.md` — `stream_responses` key.

## Tests (TDD, fully mocked)

- `TestSupportsStreaming` / `TestRealProviders_SupportStreaming` — capability gate.
- `TestStreamWatcherCmd_*` / `TestReadStreamCmd_*` — in-order accumulation, mid-stream
  error propagation, closed-channel done.
- `TestWatcherPromptMsg_StreamingProvider_UsesStreamPath` — full stream through
  `m.Update` accumulates into the buffer and reaches done.
- `TestWatcherPromptMsg_StreamingDisabled_FallsBackToBlocking` — config toggle.
- `TestWatcherStreamChunkMsg_AccumulatesInPlace`.

`make test-all` green (fmt, vet, lint, test, race, test-fixtures) on go1.26.5; race
detector clean over the streaming goroutines/channels.

## Scope note

This implements `:watcher` streaming (the provider path). `:agent` streaming is
deferred to the `ai.Provider`-SDK refactor (GH issue #352) — once `:agent` is on the
provider abstraction it reuses this exact machinery. Filed rather than done here to
keep this PR focused.

## Lessons Learned

- Streaming in Bubble Tea = a producer goroutine + a channel-draining `tea.Cmd` (one
  event per Update tick). This needs no program handle and never blocks the loop —
  cleaner than threading `p.Send` through the model.
- Gate streaming on an explicit capability interface (`SupportsStreaming`) plus a user
  config toggle, rather than assuming every provider streams.
