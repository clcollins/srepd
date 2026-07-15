# 396: Migrate error screens to renderModal

## Problem

PR #395 introduced a reusable centered modal component (`pkg/tui/modal.go`)
to fix the invisible ctrl+s confirmation prompt, with the explicit intent
that `renderModal` be reused for ALL overlay messages. The error screen was
the next migration target, but still used a hard-coded rendering path in
`View()` with several problems:

- The error box was **not centered** — it rendered at column 0 after the
  header.
- It had a **fixed 64-char width** (66 with borders) that overflowed
  narrow terminals instead of adapting to the window size.
- The **header leaked inside the error box**: `View()` writes the header
  into the string builder before the focus-mode switch, and the error
  branch appended to that same builder, so the `>` prompt and "Showing
  assigned to You" text appeared inside the error border.
- The help text was rendered **inside** the error box via
  `help.New().View(errorViewKeyMap)`, duplicating help rendering instead
  of using the modal hint convention.

## Approach

Replace the `m.err != nil` branch in `View()` with a `renderModal` call
using the existing `ModalError` variant — the same pattern the
confirmation prompt uses:

```go
case m.err != nil:
    return renderModal(windowSize.Width, windowSize.Height, m.styles, m.theme, Modal{
        Title:   "Error",
        Body:    m.err.Error(),
        Hint:    "esc: back  q/ctrl+c: quit",
        Variant: ModalError,
    })
```

The early return happens before the header content in the string builder
is used, so the modal replaces the full screen — matching the
confirmation modal behavior. `ModalError` already selects `theme.Error`
for the border and `theme.Highlight` for text; no changes to
`pkg/tui/modal.go` were needed.

## Key design decisions

- **`styles.Error` removed as dead code.** After the migration the only
  references to `m.styles.Error` were the migrated `View()` branch and a
  `NotNil` assertion in `theme_test.go`. The style (fixed 64-char rounded
  box) was removed from the `Styles` struct and `BuildStyles()`, and the
  test assertion dropped. The `theme.Error` *color* is still used by the
  `ModalError` variant.
- **Key handling unchanged.** `switchErrorFocusMode` in
  `msgHandlers.go` still clears `m.err` on `esc` via `defaultKeyMap.Back`
  and is untouched.
- **`errorViewKeyMap` kept, but now test-only.** Its help rendering was
  the only production use (`switchErrorFocusMode` matches against
  `defaultKeyMap`, not `errorViewKeyMap`). It is still referenced by
  `keymap_test.go`, so it stays for now; removing it and its test
  coverage is a candidate follow-up cleanup.
- **TDD.** Tests were updated/added first and confirmed failing against
  the old implementation (the 66-char box exceeding a 40-column
  terminal reproduced the exact reported defect), then the
  implementation made them pass:
  - `TestView_ErrorModeRendersError`: asserts modal title, error text,
    dismiss hint, and that the header does NOT leak into the view.
  - `TestView_ErrorModeNarrowTerminal`: error and hint visible at 40x20.
  - `TestView_ErrorModalDimensions`: at 120x40 and 40x20, every rendered
    line fits the terminal width and the line count fits the height.
- **Golden snapshot regenerated.** `TestGolden_ErrorMode.golden` now
  shows a centered, full-width-adaptive modal instead of the
  header-in-a-box rendering.

## Files changed

| File | Change |
|------|--------|
| `pkg/tui/views.go` | Error branch replaced with `renderModal` call |
| `pkg/tui/theme.go` | Dead `Error` style removed from `Styles`/`BuildStyles()` |
| `pkg/tui/theme_test.go` | Dropped `styles.Error` assertion |
| `pkg/tui/view_render_test.go` | Updated error-mode tests, added narrow-terminal and dimension tests |
| `pkg/tui/testdata/TestGolden_ErrorMode.golden` | Regenerated |
