# 365: Classified API errors in the config wizard (OB-3)

Issue: #353 (OB-3), part of the onboarding overhaul (#353, #324)
Branch: `srepd/classified-api-errors`

## Problem

When the wizard's live PagerDuty calls failed, the raw Go error was dumped at
the user: the token step said `invalid token: HTTP response with status code
401...` and the team multiselect rendered `Error: <err>` as a *selectable
option*, with no distinction between an auth problem (fix your token), a rate
limit (wait), or a network problem (check VPN).

## Approach

- **`pd.ClassifyAPIError(err) string`** (pkg/pd/classify.go): unwraps
  `pagerduty.APIError` — 401 → "invalid or expired token — create one at
  <token help path>", 403 → permissions + help path, 429 → rate limited;
  `context.DeadlineExceeded` / `net.Error` → network/VPN guidance; unknown
  errors pass through; nil → "". Works on wrapped errors via
  `errors.As`/`errors.Is`. The token help path (PagerDuty web → My Profile →
  User Settings → API Access) now lives in one const.
- **Wizard closures extracted into testable functions** (pkg/tui/tui.go):
  - `validateTokenInput(clientFactory, input, existingToken)` — token step
    validation, classified errors.
  - `fetchTeamOptions(clientFactory, tokenInput, existingToken,
    existingTeamSet)` — team options; placeholder/error states return a
    single empty-valued option carrying the classified message.
  - `validateTeamValues(values)` — rejects empty selection AND the
    empty-valued placeholder/error options, with recovery guidance
    ("shift+tab to go back").
  `buildConfigForm`'s closures now delegate to these.

Timeouts: all `pd.*` helpers (including `GetCurrentUserTeams`) already run
through `contextWithTimeout()`, so a hung network cannot freeze the form's
synchronous `Validate` indefinitely — no additional timeout plumbing needed.

## Tests (TDD — written first)

- `pkg/pd/classify_test.go`: table for nil / 401 / 403 / 429 / wrapped API
  error / deadline / wrapped deadline / net.Error / unknown; 401 must include
  the token acquisition path.
- `pkg/tui/config_token_validation_test.go`: token validation via mock client
  factory (blank-with-existing OK, blank-without required, valid OK, 401
  classified); team options (no-token prompt, success with preselection,
  classified error option with empty value); team-values validation rejects
  the placeholder and names the recovery key.
