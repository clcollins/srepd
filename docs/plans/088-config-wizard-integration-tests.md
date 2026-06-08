# Plan 088: Config wizard TUI integration tests (#296)

## Problem

The config wizard runs inside the srepd TUI as an inline huh form but is only
tested via structural source-scan tests and manual smoke testing. We need
automated tests covering form construction, state transitions, config writes,
and view rendering.

## Solution

1. Extract `buildConfigForm()` from inline 143-line form construction for testability
2. Inject `pdClientFactory` to avoid real PD API calls in form closures
3. Inject `configFS` to mock filesystem for write tests
4. Add 25 tests across 5 tiers: form construction, keystroke flows, state
   transitions, write integration, view rendering

## Changes

| File | Change |
|------|--------|
| `pkg/tui/tui.go` | Extract `buildConfigForm()`, use `pdClientFactory`, pass `configFS` |
| `pkg/tui/model.go` | Add `pdClientFactory` and `configFS` fields |
| `pkg/tui/commands.go` | Add `configFS` parameter to `writeConfigCmd` |
| `pkg/tui/config_mode_test.go` | 25 new tests + test helpers |

## Testing

- `make test-all` passes
- All 25 new tests pass across 5 tiers

## Post-mortem / Lessons Learned

(To be filled after merge if issues arise)
