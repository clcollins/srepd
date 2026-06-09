# Plan 090: Multi-issue Bug Fixes (#281, #282, #289, #290, #292)

## Summary

Five bug fixes in a single PR addressing crash, rendering, navigation, config migration, and bulk silence issues.

## Issues Fixed

### #289 — Panic in rebuildMergeTable
- **Root cause**: `SetRows()` called before `SetColumns()` on a fresh mergeTable, causing index-out-of-range in renderRow
- **Fix**: Swap call order in `pkg/tui/merge.go` so columns exist before rows are rendered

### #290 — Multi-paragraph note blockquoting
- **Root cause**: Template used `> {{ .Note.Content }}` which only blockquotes the first line
- **Fix**: Add `blockquote` template function that prefixes every line with `> `, returning `template.HTML` to avoid html/template escaping

### #281 — Arrow key tab navigation in incident view
- **Root cause**: Left/right arrow keys not bound to TabPrev/TabNext
- **Fix**: Add `"left"` and `"right"` keys to TabPrev/TabNext bindings in keymap

### #292 — Auto-comment old config format on startup
- **Root cause**: `CommentOutOldPolicies()` only ran during `srepd config`, not normal startup
- **Fix**: Add `autoCommentOldPolicies()` in PreRun that detects both old and new format coexistence and comments out the old block

### #282 — Bulk silence adjustments
- **Root cause**: No UI trigger for bulk silence; hardcoded default policy ignoring per-service mappings
- **Fix**: Add `ctrl+x s` chord, multi-select form (following teamSelectForm pattern), per-service policy lookup via `getEscalationPolicyKey()`, and confirmation prompt

## Files Changed

- `pkg/tui/merge.go` — swap SetColumns/SetRows order
- `pkg/tui/views.go` — add blockquote function, bulk silence form view
- `pkg/tui/keymap.go` — add left/right to tab bindings
- `pkg/tui/chords.go` — add `s` chord for bulk silence
- `pkg/tui/model.go` — add bulk silence state fields
- `pkg/tui/msgHandlers.go` — add switchBulkSilenceFocusMode, dispatch case
- `pkg/tui/tui.go` — add enterBulkSilenceMsg/bulkSilenceConfirmedMsg handlers, fix silenceIncidentsMsg per-service lookup
- `pkg/tui/commands.go` — add message types
- `cmd/root.go` — add autoCommentOldPolicies in PreRun

## Test Plan

- [x] TDD: all tests written before implementation
- [x] `make test-all` passes (fmt-check, vet, lint, test, test-race)
- [x] CI passes
- [ ] Manual: press `m` on incident — no panic
- [ ] Manual: view multi-paragraph note — all paragraphs blockquoted
- [ ] Manual: left/right arrows switch tabs in incident view
- [ ] Manual: `ctrl+x s` opens bulk silence multi-select
