# Normalize table key handler patterns and fix Open key bug

> Retroactive plan document for PR #153, created after merge.

## Context

The `switchTableFocusMode` function had four different patterns for
resolving which incident to act on:

1. Ack/Silence/UnAck: check `SelectedRow()` → send message → handler
   resolves later
2. Login: `doIfIncidentSelected()` helper
3. Note (after PR #149 fix): `SelectedRow()` → sync → check
4. Open: bare `selectedIncident == nil` check (buggy — same bug as
   Note had)

The Open key ('o') would fail with "no incident selected" on first
load without navigation.

Predecessors: [009-fix-add-note.md](009-fix-add-note.md)

## Plan

1. Research all key handler patterns and evaluate tradeoffs
2. Normalize Ack, Silence, UnAck, Note, Open to consistent 3-step
   pattern: check `SelectedRow()` → `syncSelectedIncidentToHighlightedRow()`
   → check `selectedIncident`
3. Document Login as intentional exception (`doIfIncidentSelected`
   for async wait-then-act flow)
4. Fix Open key bug in the process

## Files Modified

- `pkg/tui/msgHandlers.go` — normalized all 5 handlers
- `pkg/tui/msgHandlers_test.go` — tests for Open, Silence, UnAck
  with no-selectedIncident scenario

## Verification

- `TestTableMode_OpenKeyWithNoSelectedIncident` passes
- `TestTableMode_SilenceKeyWithNoSelectedIncident` passes
- `go test ./...` passes
