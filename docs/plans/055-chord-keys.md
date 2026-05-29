# 055: Chord Keys (ctrl+x prefix)

**Issue**: #202 (depends on #22)
**Branch**: srepd/chord-keys
**Status**: In Progress

## Problem

Limited single-key bindings available. As srepd grows, adding new actions
risks collisions with existing keys, tmux prefixes, or terminal signals.

## Solution

Implement tmux-style chord commands using a ctrl+x prefix key followed by
a second key to trigger less-frequent actions. This provides a large
namespace for future commands without consuming single-key slots.

## Design

### State Machine

Two new fields on `model`:
- `chordPending bool` -- true while waiting for the second key
- `chordPrefix string` -- configurable prefix (default "ctrl+x")

### Chord interception order in keyMsgHandler

Before focus-mode dispatch and after cluster-select/confirmation guards:

1. If `chordPending` and key is Escape: cancel chord, clear status
2. If `chordPending`: resolve second key via `resolveChord()`
3. If key matches `chordPrefix`: enter chord mode, show status

### Chord action map (pkg/tui/chords.go)

New file containing:
- `chordAction` struct: Key, Description, Handler
- `chordActions` slice with initial entries: `?` (help), `d` (debug log)
- `resolveChord()` lookup function
- `chordHelpText()` for rendering chord section in help

### Disabled during modal states

Chords are skipped when any of:
- `pendingConfirmation != nil`
- `clusterSelectMode`
- `input.Focused()`
- `err != nil`

### Config integration

`chord_prefix` added to `defaultOptionalKeys` with default `ctrl+x`.

### Help integration

Chord section appended to FullHelp as an additional column.

## Files Changed

| File | Action |
|------|--------|
| `pkg/tui/chords.go` | New -- chord map, resolver, handlers, help text |
| `pkg/tui/chords_test.go` | New -- 6+ TDD tests |
| `pkg/tui/model.go` | Add chordPending, chordPrefix fields |
| `pkg/tui/msgHandlers.go` | Add chord interception in keyMsgHandler |
| `pkg/tui/keymap.go` | Add chord help section to FullHelp |
| `cmd/config.go` | Add chord_prefix optional key |
| `docs/plans/055-chord-keys.md` | This plan |

## Tests (TDD -- written first)

1. `TestChordPrefix_ActivatesChordMode` -- ctrl+x sets chordPending, shows status
2. `TestChord_EscapeCancels` -- Escape clears chordPending and status
3. `TestChord_ValidSecondKey` -- known chord executes handler
4. `TestChord_UnknownSecondKey` -- unknown key shows error status
5. `TestChord_DisabledDuringConfirmation` -- prefix ignored during confirmation
6. `TestChord_ConfigurablePrefix` -- different prefix works

## Related

- Issue #22 (original request for chord keys)
- Issue #203 (log viewer -- would use ctrl+x d chord)
