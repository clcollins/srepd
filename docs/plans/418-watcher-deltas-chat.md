# 418 — Watcher Deltas + Chat Sessions

## Problem

The watcher re-investigates every incident on every PagerDuty refresh,
even when nothing has changed. This wastes AI tokens, floods the user
with duplicate assessments, and makes the investigation log noisy.
Additionally, investigations are scoped to the UI-selected incident
(`m.selectedIncident`) rather than the incident that actually triggered
the observation, creating identity mismatches.

## Approach

Incident-state diffing as the primary investigation gate, scoped
investigations, and the Chat interface abstraction.

### Key decisions

- **In-memory only.** No TurnStore, JSONL persistence, or file I/O.
  Restarting srepd = fresh start. Keeps Diff pure and storage-agnostic.
- **Design choice (c) for context:** triggering incidents are
  foregrounded in the observation context; sibling alerts appear as
  background in a queue summary.
- **Delta gate is primary, cooldown is secondary.** If incident state
  hasn't changed (`len(changes) == 0`), detectors don't run at all.
  Cooldown-based dedup remains as a secondary rate limiter for cases
  where an unrelated field changes.
- **First-sighting semantics:** a new incident with no prior snapshot
  counts as changed (IncidentNew), so the first poll always triggers
  investigation.

## Deliverables

| ID | Summary | Files | Commit |
|----|---------|-------|--------|
| D1a | Pure `Diff(prev, curr)` function, `Narrate`, `Snapshot` types | `pkg/delta/delta.go`, `pkg/delta/delta_test.go` | `df22984` |
| D1b | Wire delta into watcher, gate investigation on changes | `pkg/tui/watcher.go`, `pkg/tui/tui.go`, `pkg/tui/model.go`, `pkg/tui/watcher_integration_test.go` | `5883b99` |
| D2 | Scope investigations to triggering incident, not UI selection | `pkg/tui/watcher.go`, `pkg/tui/investigation.go`, `pkg/tui/model.go`, `pkg/tui/tui.go`, `pkg/tui/watcher_test.go`, `pkg/tui/investigation_test.go`, `pkg/tui/ask_wiring_test.go`, `pkg/tui/approvals_update_test.go` | `e0c41c2` |
| D3 | Feed delta changes into investigation seed context | `pkg/tui/watcher.go` (via `delta.Narrate`) | `5883b99` |
| D4 | Land Chat interface in pkg/ai (optional-interface pattern) | `pkg/ai/provider.go`, `pkg/ai/provider_test.go` | `4c56dc0` |
| D5 | Add `get_recent_events` tool as ClassRead | `pkg/ai/tools/handlers.go`, `pkg/ai/tools/handlers_test.go`, `pkg/tui/model.go` | `47046e9` |

## Delta change kinds

| Kind | When |
|------|------|
| IncidentNew | ID exists in current but not previous |
| IncidentResolved | ID exists in previous but not current |
| StatusChanged | Status field differs |
| UrgencyChanged | Urgency field differs |
| Escalated | Escalation level increased |
| NoteAdded | Note count increased |
| AlertAdded | Alert count increased |

## Chat interface (D4)

Follows the optional-interface pattern established by `StreamingProvider`:

```go
type Chat interface {
    Send(ctx context.Context, userMsg string) (string, error)
    History() []Turn
}
```

Helper functions `SupportsChat(p Provider)` and `AsChat(p Provider)`
use type assertions — no provider is forced to implement Chat.

## Test coverage

- `pkg/delta`: 16 table-driven subtests covering all change kinds,
  first sighting, reordering, empty inputs, escalation decrease
  (no-change), and Narrate formatting
- `pkg/tui`: `TestDeltaGate_IdenticalRefreshesProduceOneInvestigation`,
  `TestBuildObservationContext_ScopedToTriggeringIncident`,
  `TestBuildObservationContext_MultipleTriggering`,
  `TestBuildObservationContext_EmptyIncidentIDs`,
  `TestBuildAskFromVerdict_UsesOriginatingIncident`
- `pkg/ai`: `TestSupportsChat`, `TestAsChat`
- `pkg/ai/tools`: `TestGetRecentEvents_{HappyPath,EmptyChanges,WithLimit,InvalidInput,IsClassRead}`

## Revert-check properties

1. **D1 delta gate:** identical refreshes produce zero changes →
   `runDetectors` returns nil (test: `TestDeltaGate_IdenticalRefreshesProduceOneInvestigation`)
2. **D2 scoping:** observation context foregrounds triggering incidents,
   not UI selection (test: `TestBuildObservationContext_ScopedToTriggeringIncident`)
3. **D5 get_recent_events:** tool is ClassRead and returns expected
   JSON (test: `TestGetRecentEvents_IsClassRead`)

## Constraints

- No new Go modules added
- No `//nolint` directives
- No `replace` directives or vendoring
- PagerDuty `Incident.EscalationPolicy` is `APIObject` (no `NumLoops`),
  so escalation level defaults to 0 in snapshots
- Pre-existing `cmd/` test failure (missing config keys) is not caused
  by these changes
