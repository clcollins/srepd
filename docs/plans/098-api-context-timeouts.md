# Plan 098: Add context timeouts to raw PagerDuty client calls

**Branch:** `srepd/api-context-timeouts`

## Problem

Three `tea.Cmd` bodies in `pkg/tui/commands.go` called the PagerDuty client
directly with a bare `context.Background()` instead of a timeout-bounded context,
bypassing the `pd` package's `contextWithTimeout()` convention:

- `getIncident` — `p.Client.GetIncidentWithContext(context.Background(), id)`
- `reassignIncidents` — `GetCurrentUserWithContext(context.Background(), ...)`
- `addNoteToIncident` — `GetCurrentUserWithContext(context.Background(), ...)`

The client is wrapped by `RateLimitedClient`, which retries with backoff and passes
the context straight through. With no deadline, a persistently failing/slow endpoint
can block (and retry) indefinitely.

## Solution

Route all three through timeout-wrapped `pd` helpers:

- `getIncident` now calls the existing `pd.GetIncident(p.Client, id)` (pd.go:258),
  which applies `contextWithTimeout()` and wraps the error with a `pd.GetIncident():`
  prefix.
- Added one new exported wrapper `pd.GetCurrentUser(client)` (mirroring
  `pd.GetCurrentUserTeams`) that applies the default timeout. Both
  `reassignIncidents` and `addNoteToIncident` now call it.

## Files Modified

- `pkg/pd/pd.go` — new `GetCurrentUser` wrapper.
- `pkg/pd/pd_test.go` — `TestGetCurrentUser_Success` / `_Error`.
- `pkg/tui/commands.go` — three call sites routed through the wrappers.
- `pkg/tui/commands_test.go` — error-path assertion updated for the wrapped error.

## Tests (TDD)

Written first, seen red (`undefined: GetCurrentUser`), then green:
- `TestGetCurrentUser_Success` — returns the user and calls
  `GetCurrentUserWithContext` (proving it routes through the timeout wrapper).
- `TestGetCurrentUser_Error` — wraps the error with the `pd.GetCurrentUser()` prefix.

Existing tests act as the regression net (`TestReassignIncidents_*`,
`TestAddNoteToIncident_*`, `TestReassignIncidents_GetCurrentUserError` — the last uses
`errors.Is`, unaffected by wrapping).

### Deliberate existing-test change (documented per convention)

`TestGetIncident`'s error subtest asserted exact equality against the bare
`pd.ErrMockError`. Because `getIncident` now routes through `pd.GetIncident`, the
error carries a `pd.GetIncident():` prefix (matching the pre-existing
`GetAlerts`/`GetNotes` wrappers). That subtest was replaced by
`TestGetIncident_ErrorIsWrapped`, which asserts the message contains both the wrapper
prefix and the original mock error text — strictly more context than before, no
behavior regression for callers that surface the error string.

## Verification

`make test-all` green (fmt, vet, lint, test, test-race, test-fixtures).

## Lessons Learned

- Any raw `p.Client.*WithContext(context.Background(), ...)` call in the tui package
  bypasses the `pd` timeout convention. Prefer the `pd.*` wrappers; they centralize
  `contextWithTimeout()` and consistent error wrapping.
- The `pd` wrappers wrap with `%v`, not `%w`, so `errors.Is` against the sentinel does
  not survive. Tests on wrapped errors should assert on the message substring (as the
  sibling `GetAlerts`/`GetNotes` tests do), not sentinel identity.
