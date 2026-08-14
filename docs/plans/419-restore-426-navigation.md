# Plan 419: Restore PR #426 — g/G navigation in incident viewer

## Problem

PR #426 added g/G (top/bottom) keyboard navigation to the incident viewer
and fixed the `getlint` Makefile target to use `$(BIN_DIR)/golangci-lint`
instead of `which golangci-lint`. It merged successfully, but a subsequent
rebase of PR #419 resolved a conflict by keeping only the #419 side,
silently dropping all three #426 changes. The loss was undetectable by CI
because both the feature code and its test were removed together, leaving
a consistent green tree.

## What was lost

1. **`pkg/tui/msgHandlers.go`** — two `case` blocks in
   `switchIncidentFocusMode` handling `defaultKeyMap.Top` and
   `defaultKeyMap.Bottom` to call `GotoTop()`/`GotoBottom()` on the
   incident viewer viewport.
2. **`pkg/tui/model_test.go`** — `TestIncidentViewer_TopBottom`, the test
   covering the above navigation.
3. **`Makefile`** — line 71 of the `getlint` target changed from
   `@which golangci-lint` to `$(BIN_DIR)/golangci-lint`, ensuring the
   project-local binary is used.

## Approach

Surgical restoration of the exact changes from PR #426, verified against
`gh pr diff 426 --repo openshift-online/srepd`. No modifications, no adjacent
changes.

## Revert check

Deleting the two `case` blocks causes `TestIncidentViewer_TopBottom` to
fail, confirming the test is wired to the feature code.
