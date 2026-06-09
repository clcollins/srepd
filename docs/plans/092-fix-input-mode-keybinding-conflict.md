# Plan: Fix input mode key binding conflict (#308)

**Status:** Complete
**Issue:** #308
**PR:** #310

## Problem

Global key handlers in `keyMsgHandler()` intercept keypresses before
they reach `switchInputFocusMode()`. Keys like `u` (urgency toggle),
`i`/`:` (input focus), `ctrl+r` (auto-refresh), and `ctrl+a` (auto-ack)
are consumed by global handlers, preventing users from typing those
characters in the input field.

## Solution

Add an `m.input.Focused()` early return after chord handling (which
already correctly gates on `!m.input.Focused()`) and before global key
handlers. When input is focused, only Quit passes through globally;
everything else routes to `switchInputFocusMode()`.

## Files changed

- `pkg/tui/msgHandlers.go` — 7-line guard added at line 132
- `pkg/tui/msgHandlers_test.go` — 8 new tests covering all intercepted
  keys plus preserved Escape/Enter/Ctrl+C behavior

## Post-mortem / lessons learned

_(to be completed after merge)_
