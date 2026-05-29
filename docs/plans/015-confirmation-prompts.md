# Add confirmation prompts for destructive incident actions

> Retroactive plan document for PR #155.
> Fixes #21.

## Context

Silence and re-escalate actions executed immediately with no
confirmation. A mispress could affect the wrong incident, especially
since auto-refresh can shift the highlighted row between keystrokes.

Acknowledge was intentionally excluded from confirmation because it
is the most common action and easily reversible.

Predecessors:
[013-normalize-key-handlers.md](013-normalize-key-handlers.md),
[007-preserve-autorefresh-selection.md](007-preserve-autorefresh-selection.md)

## Plan

1. Add `confirmActionState` struct (prompt string, action tea.Cmd)
   and `pendingConfirmation` field to model
2. Silence (ctrl+s) and re-escalate (ctrl+e) set
   `pendingConfirmation` instead of executing
3. Acknowledge ('a') executes immediately without confirmation
4. Render prompt in header status area: "Silence P1234567? [y/n]"
5. Accept 'y' (execute), 'n' or Escape (cancel)
6. Clear `pendingConfirmation` on view transitions

## Files Modified

- `pkg/tui/model.go` — `confirmActionState`, `pendingConfirmation`
- `pkg/tui/msgHandlers.go` — `handleConfirmationInput`, modified
  Silence/UnAck to set confirmation, Ack executes directly
- `pkg/tui/views.go` — confirmation prompt in `renderHeader`
- `pkg/tui/model_test.go` — 9 test functions
- `pkg/tui/msgHandlers_test.go` — updated Silence/UnAck tests

## Verification

- `TestConfirmAction_AckExecutesDirectly` confirms no prompt for ack
- `TestConfirmAction_SilenceShowsPrompt` confirms prompt for silence
- `TestConfirmAction_YesExecutes` executes on 'y'
- `TestConfirmAction_NoAborts` cancels on 'n'
- `go test ./...` passes

## Lessons Learned

- Originally included acknowledge in confirmation. Removed per user
  feedback: ack is the most frequent action, easily reversible, and
  the extra keypress adds friction to the most common workflow.
- Rebase conflict with PR #153 (normalized key handlers) required
  merging the sync pattern with the confirmation pattern. Both the
  sync and the confirmation are needed: sync first to resolve the
  incident, then set the confirmation prompt.
