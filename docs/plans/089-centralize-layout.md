# Plan 089: Centralize TUI layout calculations (#291)

## Problem

Magic numbers for height/width calculations scattered across the TUI. Each mode
independently guesses header/footer/help sizes (`fixedLines := 6`,
`reservedLines := 7`, `windowSize.Height - 4`). These can drift out of sync and
resizing during team select, cluster select, or merge modes was not handled.

## Solution

Create a `Layout` struct with named constants computed via a pure `computeLayout()`
function. Store on model, replace scattered arithmetic. Absorb
`recalculateTableHeight()` into `recomputeLayout()`. Forward resize events to all
active components (forms, merge table, etc.).

## Changes

| File | Change |
|------|--------|
| `pkg/tui/layout.go` | NEW: Layout struct, named constants, computeLayout, recomputeLayout |
| `pkg/tui/layout_test.go` | NEW: 14 tests |
| `pkg/tui/model.go` | Add `layout` field, remove `recalculateTableHeight`, update toggleHelp |
| `pkg/tui/msgHandlers.go` | Simplify windowSizeMsgHandler, resize all active components |
| `pkg/tui/tui.go` | Forms and cluster select use m.layout dimensions |
| `pkg/tui/chords.go` | chordShowHelp calls recomputeLayout |
| `pkg/tui/msgHandlers_test.go` | Update recalculateTableHeight calls to recomputeLayout |

## Testing

- `make test-all` passes
- 14 new layout tests including regression test matching old behavior
- Resize tests for config mode, team select mode, and incident view mode

## Post-mortem / Lessons Learned

(To be filled after merge if issues arise)
