# Add rate limiting and retry with exponential backoff to PD API client

> Retroactive plan document for PR #151, created after merge.
> Fixes #97.

## Context

No rate limiting on PD API calls. When 800+ alerts triggered, the
app fired hundreds of API calls in rapid succession, received 429
responses from PagerDuty, and became unusable. There was no retry
logic for transient failures.

Predecessors: [006-add-api-timeouts.md](006-add-api-timeouts.md),
[008-enhanced-mock-client.md](008-enhanced-mock-client.md)

## Plan

1. Create `pkg/pd/ratelimit.go` with `RateLimitedClient` wrapping
   `PagerDutyClientInterface`
2. Token bucket rate limiter via `golang.org/x/time/rate` (10 req/s,
   burst 20)
3. Exponential backoff on 429 responses (1s initial, 30s max,
   3 retries)
4. Respect `Retry-After` header when present
5. Integrate by wrapping client in `NewConfig`

## Files Modified

- `pkg/pd/ratelimit.go` — new, `RateLimitedClient` wrapper
- `pkg/pd/ratelimit_test.go` — new, 6 tests
- `pkg/pd/pd.go` — wrap client creation
- `go.mod`, `go.sum` — `golang.org/x/time/rate` dependency

## Verification

- `TestRateLimiter_PassthroughBelowLimit` passes
- `TestRetry_429Response` retries with backoff
- `TestRetry_MaxRetriesExhausted` errors after 3 retries
- `go test ./...` passes
