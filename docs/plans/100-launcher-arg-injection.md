# Plan 100: Harden launcher against argument injection

**Branch:** `srepd/launcher-arg-injection`

## Problem

`replaceVars` (`pkg/launcher/launcher.go`) substituted template variables
(`%%CLUSTER_ID%%`, `%%INCIDENT_ID%%`) by joining all args on a single space,
running `strings.ReplaceAll`, then splitting back on spaces. Any substituted value
containing a space was **re-tokenized into extra argv elements**.

The values come from PagerDuty alert data (`getDetailFieldFromAlert("cluster_id", …)`)
and are not validated. Because the built command is passed to `exec.Command` (no
shell), this is not shell RCE — but a `cluster_id` like `x --evil-flag y` could inject
extra command-line arguments into `ocm backplane login` / `rosa-boundary`.

## Solution — two complementary layers

1. **Per-arg substitution.** `replaceVars` now iterates each arg and substitutes in
   place, never joining/splitting. A substituted value with spaces stays within its
   single argv slot.
2. **Validate at the TUI boundary.** Added exported `ocm.ValidClusterID(id)` (wrapping
   the existing `clusterIDPattern = ^[a-zA-Z0-9_-]+$`). `getUniqueClusters`
   (`pkg/tui/commands.go`) now skips any cluster_id that fails validation (and logs a
   warning), so malformed values never reach the launcher `vars` map.

## Files Modified

- `pkg/launcher/launcher.go` — `replaceVars` rewritten per-arg.
- `pkg/launcher/launcher_test.go` — `TestReplaceVars_PreservesArgBoundaries`.
- `pkg/ocm/client.go` — exported `ValidClusterID`.
- `pkg/ocm/ocm_test.go` — `TestValidClusterID`.
- `pkg/tui/commands.go` — `getUniqueClusters` validates cluster IDs.
- `pkg/tui/commands_test.go` — `TestGetUniqueClusters_RejectsMalformedClusterID`.

## Tests (TDD)

Written first, seen red, then green:
- `TestReplaceVars_PreservesArgBoundaries` — a value with spaces stays one argv
  element (fails on the old join/split); simple/no-space cases and nil inputs unchanged.
  **Asserts on the `[]string` slice**, not a joined string, so it cannot pass if arg
  boundaries regress (the existing `TestLoginCommandBuild` compares via `strings.Join`,
  which is boundary-blind — this test closes that gap).
- `TestValidClusterID` — accepts UUID/underscore IDs, rejects empty / spaces /
  injected flags / shell metacharacters.
- `TestGetUniqueClusters_RejectsMalformedClusterID` — malformed IDs filtered, good ID
  kept.

Existing `TestLoginCommandBuild` and all `TestGetUniqueClusters_*` remain green.

## Verification

`make test-all` green. Manual: an alert with a space-bearing `cluster_id` no longer
injects extra args into the launched login command (value is dropped at
`getUniqueClusters` and, defensively, would stay one argv slot in `replaceVars`).

## Lessons Learned

- Join-then-split on spaces silently destroys argv boundaries; substitute per-arg.
- Tests that compare argv via `strings.Join` cannot detect boundary regressions —
  assert on the slice.
- Attacker-influenceable data (PagerDuty alert fields) must be validated at the trust
  boundary before it reaches `exec.Command`, even without a shell.
