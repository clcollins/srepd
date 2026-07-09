# Plan 101: Harden AI providers (no token leak, request timeouts)

**Branch:** `srepd/ai-provider-hardening`

## Problem

1. **Token leak vector.** On non-200 responses, `openai_compat.go` and `ollama.go`
   read the response body and interpolated it into the returned error
   (`server returned %d: %s`). A proxy/gateway that echoes the `Authorization: Bearer
   <token>` header back in its error body would leak the API token into srepd's error
   output and logs.
2. **No request-timeout defense.** The providers share a single `http.Client` with no
   `Timeout`. Callers pass a deadline context today, but there was no defense-in-depth
   if a caller ever passed a deadline-less context.

## Solution

1. **Drop the body from error paths.** All four `Query`/`StreamQuery` non-200 branches
   now return the status code only (`server returned %d`), matching the existing
   `Healthy` methods. The token is never in the request/response body — only the
   header — so status-only removes the leak regardless of what the server echoes.
2. **Per-request default timeout for non-streaming calls.** Added
   `defaultRequestTimeout` (60s) and `ensureTimeout(ctx, d)` in `provider.go`:
   returns ctx unchanged if it already has a deadline, else derives a bounded one.
   Applied in both providers' `Query` and `Healthy`. **Not** applied to `StreamQuery`
   (a whole-request timeout would truncate long token streams) and **not** set on the
   shared `http.Client.Timeout` (same reason — it would hit the stream path).

## Files Modified

- `pkg/ai/provider.go` — `defaultRequestTimeout`, `ensureTimeout`.
- `pkg/ai/openai_compat.go` / `ollama.go` — drop body from 4 error sites; add
  `requestTimeout` field; `ensureTimeout` in `Query`/`Healthy`; remove now-unused `io`.
- `pkg/ai/openai_compat_test.go` / `ollama_test.go` — leak + timeout tests.

## Tests (TDD)

Written first, seen red, then green:
- `TestOpenAIQuery_DoesNotLeakTokenInError` / `_StreamQuery...` /
  `TestOllamaQuery_DoesNotLeakHeadersInError` — server echoes the Authorization header
  in a 401 body; the error must contain `401` but never the token/marker (red on the
  old body-echoing error).
- `TestOpenAIQuery_AppliesDefaultTimeout` — a hung server + `context.Background()`
  (no deadline) returns promptly via the provider's default timeout.

Existing `TestOpenAIQuery_ServerError` / `TestOllamaQuery_ServerError` assert
`Contains("500")` — still true (status code retained), so unchanged.

## Verification

`make test-all` green (fmt, vet, lint, test, race, test-fixtures).

## Lessons Learned

- Never interpolate an upstream response body into an error surfaced to logs when the
  request carries a secret header — a reflecting proxy turns it into a credential leak.
  Prefer status-code-only (as the `Healthy` methods already did).
- A client-level `http.Client.Timeout` is a whole-request deadline and will truncate
  streaming responses; bound non-streaming calls via the request context instead.
