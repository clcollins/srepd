# 397: Render errors as a full-page SREPD-styled view

## Problem

PR #399 (plan 396) migrated the error screen from a hard-coded box to the
centered `renderModal` overlay. Reviewing it live, Chris asked for the
error to look like part of SREPD instead of a floating box: keep the full
main-window border and the footer info, so it reads as "the page content
was replaced by the error" — sized like the incident table area when the
watcher pane is collapsed.

Requested layout:

- Header: the standard status header (`> showing N/N incidents… /
  Showing assigned to You`) replaced by a bold `Error`, centered.
- Content: full-window bordered container; a second bold `Error` title
  above the error message, the block vertically centered, text in white.
- Footer: the standard help line (`h help • esc back • ctrl+q/ctrl+c
  quit`) and the bottom status line, including the update banner — which
  renders only when an update is actually available.

## Approach

The `m.err != nil` branch flows through the normal `View()` layout
instead of early-returning:

- `renderErrorHeader()` renders the centered bold `Error` title in the
  header slot.
- `renderErrorContent()` renders the error inside a
  `TableContainer`-styled box that fills the space between the header
  and the help + bottom-status lines. Height is computed dynamically
  from the window size, lines already written, and the rendered help
  line count, so the page fits any terminal and adapts when full help
  is expanded. Content is vertically centered via `AlignVertical`;
  title and body use `theme.Highlight` (white).
- The `View()` tail's keymap selection returns `errorViewKeyMap` in
  error mode, so the standard help/bottom-status rendering does the
  rest — no duplicated footer logic.

## Key design decisions

- **Honest quit help.** `errorViewKeyMap` previously advertised
  `q/ctrl+c quit`, but `q` never quit — the global quit matcher in
  `keyMsgHandler` listens for ctrl+q/ctrl+c only. The keymap now mirrors
  those keys, and gains `Help` (h) so its ShortHelp matches the default
  view footer exactly.
- **`h` toggles full help in error mode** (new `Help` case in
  `switchErrorFocusMode`); the error container shrinks to fit the
  expanded help. `TestKeymapCompleteness` enforces the binding/help
  pairing automatically.
- **`renderModal` stays** for the confirmation prompt. The `ModalError`
  variant (and the `theme.Error` color) are no longer used by the error
  screen but remain part of the modal component API.
- Known cosmetic quirk, accepted: expanded help shows the chord-command
  column even though chords are disabled in error mode
  (`keymap.FullHelp` appends chord bindings unconditionally).

## Verification

- TDD: view tests assert the two Error titles, vertical centering,
  footer bindings, window border, no header leak, and update-banner
  gating; dimension tests assert every line fits 120x40 and 40x20;
  a handler test covers the help toggle.
- Golden `TestGolden_ErrorMode` regenerated; no other golden changed.
- Live-verified in `--dev` mode (pressing `o` without `xdg-open`
  produces a real errMsg): page renders per spec, `h` expands help,
  `esc` returns to the table.

## Files changed

| File | Change |
|------|--------|
| `pkg/tui/views.go` | Error branch renders full-page view; `renderErrorHeader`/`renderErrorContent` helpers; error keymap in help selection |
| `pkg/tui/keymap.go` | `errorViewKeyMap` gains Help, quit keys corrected to ctrl+q/ctrl+c |
| `pkg/tui/msgHandlers.go` | `switchErrorFocusMode` handles Help toggle |
| `pkg/tui/view_render_test.go` | Full-page layout assertions, update-banner tests |
| `pkg/tui/msgHandlers_test.go` | Help-toggle handler test |
| `pkg/tui/testdata/TestGolden_ErrorMode.golden` | Regenerated |
