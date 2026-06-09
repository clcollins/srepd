# Plan 091: Flag conditions for incidents (#280)

## Problem

SREs need to visually mark incidents matching specific criteria (cluster ID,
customer org) to track incidents of interest during an on-call shift. No
mechanism exists for this.

## Solution

Add session-only flag conditions that prepend a configurable marker to matching
incident summaries in the table view, with a "Flag Conditions" section in the
Details tab. Conditions match against cluster ID (internal, external, or raw
alert ID) and organization name (with glob patterns). Conditions can optionally
be saved/loaded to JSON for persistence.

## Changes

| File | Change |
|------|--------|
| `pkg/tui/flags.go` | New: FlagCondition types, matchGlob, evaluateFlags, matchClusterID, matchOrgName, renderFlagConditionsSection, formatFlagsList, rebuildFlagMatchCache |
| `pkg/tui/flags_test.go` | New: 50+ tests for glob matching, flag evaluation, message handlers, display |
| `pkg/tui/flag_commands.go` | New: Slash command parsing (/flag, /flags, /unflag), save/load, dispatch |
| `pkg/tui/flag_commands_test.go` | New: 23 tests for command parsing and isFlagCommand |
| `pkg/tui/model.go` | Add flagConditions, flagNextID, flagMarker, flagMatchCache fields |
| `pkg/tui/tui.go` | Add message handlers, row annotation with flag marker, cache rebuild triggers |
| `pkg/tui/msgHandlers.go` | Add f key handler, slash command routing in input mode |
| `pkg/tui/keymap.go` | Add Flag key binding on f |
| `pkg/tui/views.go` | Add flag conditions section to renderDetailsTab |
| `pkg/config/config.go` | Add flag_marker optional config key |
| `README.md` | Document flag feature, f key, flag_marker config |
| `docs/flag-conditions.md` | Full feature documentation with save file spec |

## Verification

- `make test-all` passes
- Core logic (matchGlob, evaluateFlags) at 100% coverage
- Command parsing at 77-100% coverage
- SUPPORTEX/OHSS deferred to #303
