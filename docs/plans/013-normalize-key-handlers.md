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

## Lessons Learned

**GENUINE ERROR — normalization missed handlers in other focus modes**
(Fixed by: [059-fix-actions-no-selection.md](059-fix-actions-no-selection.md))

This plan normalized key handlers in `switchTableFocusMode` but did
not audit `switchIncidentFocusMode` for the same bug class. The Enter
key in table mode, the SOP key, and multiple handlers in
`switchIncidentFocusMode` (Ack, UnAck, Silence, Note, Login, Open,
SOP, Refresh) were all missing nil guards for `selectedIncident`.

Why it wasn't caught: the plan scoped its fix to `switchTableFocusMode`
because that's where the original bug report (009) pointed. No audit
of other focus mode handlers was performed.

Prevention: when fixing a pattern bug, audit every function that
handles the same type of input dispatch — not just the function where
the bug was first identified. Grep for the same anti-pattern across
all focus modes.
