# Fix watcher pane missing right border

## Problem

The watcher pane (toggled with `w`) is missing its right border. The
incident table above it renders all four borders correctly, but the
watcher box's right border is clipped off-screen at any terminal width.

## Root cause

In `layout.go`, `watcherWidth` was computed using `containerHOverhead`
derived from `TableContainer` (which has no padding). The watcher
content is rendered inside `WatcherContainer`, which has
`Padding(0, 1)` -- 2 extra horizontal characters. This made the
viewport 2 chars too wide, pushing the right border past the terminal
edge.

## Fix

Add a separate `watcherHOverhead` variable computed from
`WatcherContainer`'s own margins, padding, and border size, and use
it for `watcherWidth` instead of reusing `containerHOverhead`.

## Files changed

- `pkg/tui/layout.go` -- compute `watcherHOverhead` from
  `WatcherContainer` style; use it for `watcherWidth`
- `pkg/tui/layout_test.go` -- add
  `TestComputeLayout_WatcherWidthAccountsForPadding` verifying the
  watcher width fits within the terminal at multiple widths
