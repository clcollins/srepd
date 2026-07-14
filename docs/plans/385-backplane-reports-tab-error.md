# 385: Fix misleading backplane error on Reports tab

Branch: `srepd/fix-backplane-reports-tab-error`

## What

The Reports tab shows "Backplane not enabled; ensure your config.json exists..."
whenever `backplaneClient` is nil, regardless of the actual cause. This is
misleading when the config loaded fine but the client hasn't been created yet
(OCM auth pending, URL resolution failed, etc.).

Replace the single nil check with a cascade that shows the real state:
no config, auth pending, waiting for OCM, init error with details, or
a fallback pointing to logs.

Also store `backplaneInitErr` on the model so the deferred-init failure
in `OCMClientReadyMsg` surfaces the actual error to the user.

## Files

- `pkg/tui/model.go` — add `backplaneInitErr` field
- `pkg/tui/tui.go` — set/clear `backplaneInitErr` in OCMClientReadyMsg handler
- `pkg/tui/views.go` — replace single nil check with cascading status messages
- `pkg/tui/views_render_test.go` — six tests covering all branches
