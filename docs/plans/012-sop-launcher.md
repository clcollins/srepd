# Add SOP launcher key binding to open alert SOP links directly

> Retroactive plan document for PR #152, created after merge.
> Fixes #53.

## Context

Users had to open the PagerDuty web UI to find SOP links from
alerts. Alert details contain a "link" field (or "runbook_url"
for Prometheus-convention alerts) with the SOP URL, but there was
no key binding to open it directly from srepd.

Predecessor:
[005-fix-unsafe-type-assertions.md](005-fix-unsafe-type-assertions.md)

## Plan

1. Create pure function `getSOPLink(alerts) (string, bool)` that
   checks both `"link"` and `"runbook_url"` fields (in priority
   order) using the safe `getDetailFieldFromAlert` accessor
2. Add 's' key binding in both table and incident focus modes
3. Open SOP via `xdg-open` (same pattern as existing 'o' browser
   open)
4. Show "no SOP link found" if no link exists

## Files Modified

- `pkg/tui/commands.go` — `getSOPLink`, `sopLinkFields`,
  `openSOPMsg`
- `pkg/tui/commands_test.go` — 7 tests (including `runbook_url`
  and priority ordering)
- `pkg/tui/keymap.go` — `SOP` key binding ('s')
- `pkg/tui/msgHandlers.go` — handlers in table and incident modes
- `pkg/tui/tui.go` — `openSOPMsg` handler

## Verification

- `TestGetSOPLink_HasLink` extracts from "link" field
- `TestGetSOPLink_RunbookURL` extracts from "runbook_url" field
- `TestGetSOPLink_LinkTakesPriorityOverRunbookURL` confirms priority
- `go test ./...` passes

## Lessons Learned

- Initial implementation only checked the `"link"` field. Review
  of managed-cluster-config alert rules revealed some alerts use
  `"runbook_url"` (Prometheus annotation convention) instead. Added
  `sopLinkFields` slice to check both in priority order. Always
  verify field names against actual alert data, not assumptions.
