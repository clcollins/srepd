# Plan 087: Fix table height calculation (#288)

## Problem

The incident table compresses to 0 visible rows on 24-line terminals because
`windowSizeMsgHandler` uses hardcoded overhead values (`estimatedExtraLinesFromComponents = 7`,
`additionalSpacing = 15`, plus `rowCount` subtracted from height) that consume more
lines than a normal terminal provides.

Additionally, toggling help does not recalculate table height.

## Solution

Extract `recalculateTableHeight()` on `*model` that computes table height from
the actual rendered help view line count and known fixed-component overhead
(header, footer, input, bottom status, container border). Call it from
`windowSizeMsgHandler`, `toggleHelp()`, and `chordShowHelp()`.

## Changes

| File | Change |
|------|--------|
| `pkg/tui/model.go` | Add `recalculateTableHeight()`, modify `toggleHelp()`, add `strings` import |
| `pkg/tui/chords.go` | Call `recalculateTableHeight()` in `chordShowHelp()` |
| `pkg/tui/msgHandlers.go` | Remove hardcoded overhead, call `recalculateTableHeight()` |
| `pkg/tui/msgHandlers_test.go` | 5 new tests for table height calculation |

## Testing

- `make test-all` passes (fmt, vet, lint, race, tests)
- Manual verification: 24-line terminal shows ~13 data rows with compact help

## Post-mortem / Lessons Learned

(To be filled after merge if issues arise)
