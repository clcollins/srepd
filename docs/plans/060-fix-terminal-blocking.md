# Fix terminal launch blocking srepd until window closed

## Context

Pressing 'l' to login froze srepd until the terminal window (ptyxis)
was closed. The login() function used stdout/stderr pipes and
cmd.Wait() which blocked the tea.Cmd goroutine until the child
process exited.

## Plan

Replace the blocking pipe+wait pattern with cmd.Start() and a
background goroutine for process reaping. Return loginFinishedMsg
immediately after successful start.

## Files Modified

- `pkg/tui/commands.go` — removed stdout/stderr pipes, io.ReadAll,
  cmd.Wait blocking. Added background goroutine for zombie reaping.
  Removed unused `io` import.

## Verification

- Press 'l' to login — srepd returns to normal immediately
- Terminal window opens and functions independently
- No zombie processes after terminal closes
