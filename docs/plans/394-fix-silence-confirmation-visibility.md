# Fix: ctrl+s confirmation prompt hidden by layout overflow

## Context

Justin Downie reported (2026-07-14) that pressing ctrl+s to silence an
incident in srepd v1.6.1 "froze" the TUI — he had to ctrl+c out three
times. The TUI never actually froze: the confirmation prompt renders only
in the header (row 1 of the frame), and when the terminal is narrower
than the 8-tab tab bar (~100+ cols intrinsic width), the tab rows wrap
into extra terminal rows that View()'s newline-counting height math
doesn't account for. The frame exceeds the terminal height, the terminal
scrolls, and the header (with the prompt) is pushed off-screen. With an
invisible prompt and all keys swallowed except y/n/esc/quit, the user
perceives a hard freeze.

Drive-by found during review: errMsgHandler never resets apiInProgress,
so a failed API call leaves the header spinner running indefinitely.

## Plan

### 1. Reusable centered modal overlay (pkg/tui/modal.go)

New file with `Modal` struct (Title, Body, Hint, Variant), `ModalVariant`
enum (ModalWarning, ModalError, ModalInfo), and `renderModal()` pure
function. Uses lipgloss.Place for full-frame centering. The component is
designed to be reusable — a follow-up PR will migrate the error screens
(the `m.err != nil` branch in View()) onto this modal.

### 2. View() integration (pkg/tui/views.go)

When `pendingConfirmation != nil`, View() returns renderModal() before
any other rendering — the modal takes over the full screen, guaranteeing
visibility at any terminal size. The old header prompt branch in
renderHeader() is removed; the header always renders statusArea now.

### 3. Clamp tab bars (pkg/tui/views.go, pkg/tui/docs.go)

New `clampLineWidth(s, maxWidth)` helper using
`github.com/charmbracelet/x/ansi.Truncate` (already an indirect dep,
promoted to direct). Applied to renderTabBar and renderDocsTabBar return
values so tabs never exceed terminal width.

### 4. Reset spinner on error (pkg/tui/msgHandlers.go)

Added `m.apiInProgress = false` in errMsgHandler.

## Files modified

- `pkg/tui/modal.go` — NEW: Modal types + renderModal
- `pkg/tui/modal_test.go` — NEW: modal unit tests
- `pkg/tui/views.go` — clampLineWidth, View() modal check, header cleanup, tab bar clamp
- `pkg/tui/views_test.go` — clampLineWidth + tab bar clamp tests
- `pkg/tui/docs.go` — docs tab bar clamp
- `pkg/tui/docs_test.go` — docs tab bar clamp test
- `pkg/tui/msgHandlers.go` — errMsgHandler resets apiInProgress
- `pkg/tui/model_test.go` — errMsg test, renamed confirmation test
- `pkg/tui/view_render_test.go` — modal integration tests
- `go.mod` — ansi promoted from indirect to direct

## Verification

- `make test-all` passes (fmt-check, vet, lint, test, test-race, fixtures)
- Manual: build, run `--dev` at ~50 cols, open incident, ctrl+s → centered modal visible; y/n/esc work; tab bar does not overflow

## Lessons learned

(Post-merge)
