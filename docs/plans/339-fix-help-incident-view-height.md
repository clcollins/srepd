# 339: Fix help window pushing incident view off-screen

## Problem

When pressing `h`/`?` in incident view to toggle the help panel, the
help text was appended below the incident viewer without shrinking it,
pushing the top of the view (tabs, header) off the terminal. In table
mode this worked correctly because `helpLines` was subtracted from the
available height.

## Root cause

`computeLayout` in `layout.go` calculated the incident viewer height as:
```go
incidentViewerHeight := ws.Height - incidentViewerFixedOverhead
```

This did not subtract `helpLines`, unlike the table height calculation
which did:
```go
availableRows := ws.Height - ... - helpLines
```

Additionally, `recomputeLayout()` (called by `toggleHelp()`) only updated
`m.table.SetHeight` and the watcher viewport — it never applied the new
`IncidentViewerHeight` to `m.incidentViewer` or `m.logViewer`.

## Fix

1. Subtract `helpLines` from the incident viewer height calculation so
   the viewport shrinks when help expands.
2. Update `recomputeLayout()` to apply the new dimensions to both
   `m.incidentViewer` and `m.logViewer` on every recompute.

## Files changed

| File | Change |
|------|--------|
| `pkg/tui/layout.go` | Subtract helpLines from incident viewer height; apply viewer dimensions in recomputeLayout |
| `pkg/tui/layout_test.go` | Two new tests: help reduces incident viewer height, recomputeLayout updates viewer dimensions |
