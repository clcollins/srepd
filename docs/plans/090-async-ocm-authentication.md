# Plan 090: Async OCM authentication (#279)

## Problem

When OCM tokens are expired, `ocm.NewClient()` calls `auth.InitiateAuthCode()`
which opens a browser and blocks indefinitely until the user completes auth.
This happens before the TUI starts — srepd appears hung while the user
authenticates. PagerDuty incidents are invisible until auth completes.

## Solution

Split `ocm.NewClient()` into composable functions so token validity can be
checked without blocking. If tokens are valid, connect synchronously (no UX
change). If expired, launch the TUI immediately and run browser auth in a
background goroutine that delivers the client via `p.Send()`.

This follows the existing `pdClientInitializedMsg` pattern where a client is
set on the model at runtime and follow-up commands are dispatched.

## Changes

| File | Change |
|------|--------|
| `pkg/ocm/client.go` | Split into `CheckTokens()`, `AuthenticateAsync()`, `ApplyAuthToken()`, `NewClientFromConfig()`, `applyConfigDefaults()`. Keep `NewClient()` as convenience wrapper |
| `pkg/ocm/client_test.go` | New tests for CheckTokens, ApplyAuthToken, NewClientFromConfig |
| `pkg/tui/commands.go` | Add exported `OCMClientReadyMsg` type |
| `pkg/tui/model.go` | Add `ocmAuthPending` field + `InitialModel()` parameter |
| `pkg/tui/tui.go` | Add `OCMClientReadyMsg` handler with retroactive enrichment |
| `pkg/tui/views.go` | Differentiate "authenticating" vs "not connected" in 3 tab renderers |
| `pkg/tui/ocm_enrichment_test.go` | 10 new tests for message handler and auth-pending views |
| `cmd/root.go` | Split `launchTUI()` to use fast-path / async-auth |
| `cmd/config.go` | Same split in `launchTUIWithConfig()` |

## Verification

- `make test-all` passes (fmt, vet, lint, unit tests, race detector)
- Fast path: valid tokens connect synchronously, journal shows `ocm.NewClientFromConfig`
- Async path: blanked tokens → TUI starts immediately, browser opens, auth completes → `OCM connected (async)` and enrichment fires
- Both paths manually verified against live PagerDuty + OCM
