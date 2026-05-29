# 055: Fix Incident View Formatting

GitHub Issue: #200

## Problem

The incident detail view has several formatting problems that make it
hard to read:

1. Incident ID is not bold in the heading
2. Glamour renderer has no style configured (root cause of most visual issues)
3. Double border from both container and viewport styles
4. Notes not indented with 2-space prefix
5. Alert detail lines not indented with 2-space prefix
6. No spacing between multiple alerts
7. Bold acknowledged username not rendering (consequence of #2)
8. URLs showing both text and raw link (consequence of #2)
9. No space between ID line and title (already has newline, but confirmed)

## Changes

### pkg/tui/model.go
- Add `glamour.WithStylePath("dark")` to `glamour.NewTermRenderer()` call
- Remove `vp.Style = incidentViewerStyle` from `newIncidentViewer()`

### pkg/tui/views.go
- Bold the incident ID: `**{{ .ID }}**`
- Indent notes with 2-space prefix on blockquote and attribution lines
- Indent alert detail bullet points with 2-space prefix
- Add blank line between alert iterations (already has `{{ end }}` with
  newline, but ensure explicit spacing)
- Remove `incidentViewerStyle` variable definition (border style)

## Testing

- Template output tests: verify bold ID markers, indented notes,
  indented alert details, spacing between alerts
- Glamour renderer test: verify `WithStylePath("dark")` produces
  styled output (bold renders as ANSI sequences)
- Viewport border test: verify `newIncidentViewer()` has no border style

## Files Modified

- `pkg/tui/model.go`
- `pkg/tui/views.go`
