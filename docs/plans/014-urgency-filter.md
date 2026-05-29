# Add urgency filter toggle to show only high-urgency incidents

> Retroactive plan document for PR #154, created after merge.
> Fixes #5.

## Context

All incidents were shown regardless of urgency. Users wanted to
toggle between showing all incidents and showing only high-urgency
ones to reduce noise during active monitoring. PagerDuty incidents
have an `Urgency` field (string: "high" or "low") that is populated
on every incident from the API.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

1. Create pure function `filterByUrgency(incidents, showLow bool)`
2. Add `showLowUrgency bool` field to model (default: true)
3. Add 'u' key binding to toggle
4. Apply filter in `updatedIncidentListMsg` handler before building
   table rows
5. Show filter state in footer: "[high urgency only]"

## Files Modified

- `pkg/tui/commands.go` — `filterByUrgency` pure function
- `pkg/tui/commands_test.go` — 5 test cases
- `pkg/tui/model.go` — `showLowUrgency` field
- `pkg/tui/keymap.go` — `Urgency` key binding ('u')
- `pkg/tui/msgHandlers.go` — toggle handler
- `pkg/tui/tui.go` — filter applied in incident list handler
- `pkg/tui/views.go` — footer rendering
- `pkg/tui/views_test.go` — 3 new view test cases

## Verification

- `TestFilterByUrgency_HighOnly` returns only high urgency
- `TestFilterByUrgency_ShowAll` returns all incidents
- `TestRefreshArea` updated with urgency filter display cases
- `go test ./...` passes
