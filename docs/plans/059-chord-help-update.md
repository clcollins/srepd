# 059: Update Chord Help Display

**Issue**: #223
**Branch**: srepd/chord-help-update
**Status**: In Progress

## Problem

The chord key help (ctrl+x ?) displays its content in the status bar at the
top of the screen. This is inconsistent with how regular help works -- the
standard help message renders at the bottom of the window. The chord help
should temporarily replace (or integrate with) the standard help display.

## Solution

Add a `chordHelpActive` boolean to the model. When the chord help action
fires (ctrl+x ?), set this flag and expand help to full mode. In `View()`,
when `chordHelpActive` is true, render a chord-specific help keymap instead
of the regular one. The flag is cleared on any subsequent keypress.

## Design

### Model changes

New field on `model`:
- `chordHelpActive bool` -- when true, the help section at the bottom shows
  chord bindings instead of the regular keymap

### chordShowHelp handler

Instead of calling `setStatus()`, the handler:
1. Sets `m.chordHelpActive = true`
2. Sets `m.help.ShowAll = true` (ensures full help is visible)
3. Does NOT set status (keeps status bar clean)

### View() integration

The help keymap selection in `View()` gains a third condition:

```
if m.chordHelpActive {
    helpKeyMap = chordHelpKeyMap
} else if m.input.Focused() {
    helpKeyMap = inputModeKeyMap
} else {
    helpKeyMap = defaultKeyMap
}
```

### Clearing chordHelpActive

The flag is cleared at the top of `keyMsgHandler` before any other
processing, so a single keypress dismisses the chord help and returns to
normal help display.

### chordHelpKeyMap

New keymap type `chordKeymap` implementing `help.KeyMap`. Its `FullHelp()`
returns chord bindings as the sole column. Its `ShortHelp()` shows a
dismissal hint.

## Files Changed

| File | Action |
|------|--------|
| `pkg/tui/model.go` | Add `chordHelpActive` field |
| `pkg/tui/chords.go` | Update `chordShowHelp`, add `chordKeymap` type |
| `pkg/tui/views.go` | Add `chordHelpActive` branch in help keymap selection |
| `pkg/tui/msgHandlers.go` | Clear `chordHelpActive` on keypress |
| `pkg/tui/chords_test.go` | Update and add tests for new behavior |
| `docs/plans/059-chord-help-update.md` | This plan |

## Tests (TDD -- written first)

1. `TestChordShowHelp_SetsChordHelpActive` -- pressing ctrl+x ? sets chordHelpActive
2. `TestChordShowHelp_DoesNotSetStatus` -- chord help no longer uses status bar
3. `TestChordHelp_ClearedOnKeypress` -- any key clears chordHelpActive
4. `TestChordHelp_ViewRendersChordKeymap` -- View() uses chord keymap when active
5. Update existing `TestChord_ValidSecondKey` to reflect new behavior
