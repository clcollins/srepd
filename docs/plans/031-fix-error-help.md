# Fix error popup help message

> Fixes #17.

## Context

The error view displayed the full `errorViewKeyMap` help text via the
bubbles help renderer, which produced multi-line keymap output that
was confusing in context.  Users seeing an error only need to know
how to dismiss it.

Additionally, dismissing the error with ESC left the app in a stale
state because no incident list refresh was triggered, so the user had
to manually refresh to recover.

## Plan

1. Replace the `help.New().View(errorViewKeyMap)` call in the error
   view rendering with a simple "Press ESC to dismiss" string
2. In `switchErrorFocusMode`, return an `updateIncidentListMsg`
   command when the error is dismissed so the app recovers gracefully
3. Add tests for both behaviors

## Files Modified

- `pkg/tui/views.go` -- replaced help renderer with plain dismiss text
- `pkg/tui/msgHandlers.go` -- `switchErrorFocusMode` now triggers
  incident list refresh on error dismiss
- `pkg/tui/msgHandlers_test.go` -- added `TestErrorView_BackClearsError`
  and `TestErrorView_BackTriggersRefresh`

## Verification

- `TestErrorView_BackClearsError` confirms ESC clears `m.err`
- `TestErrorView_BackTriggersRefresh` confirms ESC returns a command
  that produces an `updateIncidentListMsg`
- `make fmt-check` passes
- `make vet` passes
- `make test` passes (all tests green)
