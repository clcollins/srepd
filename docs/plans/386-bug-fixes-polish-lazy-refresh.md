# 386: Bug fixes, Update-loop polish, and lazy-enricher staleness refresh

Branch: `srepd/bug-fixes-polish-lazy-refresh`

## What

A full-codebase review (carried over from an unpushed session, re-verified
against current main) found six bugs, several Update-loop inefficiencies, and
a data-freshness gap:

**Bug fixes**

- `UserIsOnCall` sent `time.Now().String()` as `Since`/`Until` — not the
  ISO8601/RFC3339 the PagerDuty API accepts — so the on-call check that gates
  auto-acknowledge queried with invalid time bounds. It also returned "not
  on call" on the first malformed on-call entry, hiding valid shifts, and
  called `time.Now()` four times per invocation.
- `removeCommentsFromBytes` dropped newlines (multi-line notes collapsed to
  one line when posted to PagerDuty) and wrote each kept line once per
  non-matching prefix when given multiple prefixes.
- `login()` and `openBrowserCmd()` ran `exec.Command(...).Start()` at
  command-construction time — inside the Bubble Tea Update loop — instead of
  inside the returned `tea.Cmd`. A slow terminal or browser launch froze the
  UI. All four launch paths (regular, cluster-selected, and both
  rosa-boundary variants) route through `login()` since PR #386, so one fix
  covers them all. `openBrowserCmd` also gained a `go c.Wait()` reaper.
- `loginMsg`, `rosaBoundaryLoginMsg`, and `waitForSelectedIncidentThenDoMsg`
  re-queued themselves immediately while waiting for data, spinning the
  Update loop at full speed. They now requeue via `requeueAfterDelay()`
  (`tea.Tick`, 250ms).
- `silenceIncidentsMsg` unconditionally appended the selected incident to
  the list being silenced, double-silencing it when already present.
- `initialScheduledJobs` was a package-level slice of pointers; the job
  structs (whose `lastRun` mutates as jobs fire) were shared across model
  instances. Replaced with a `defaultScheduledJobs()` factory.

**Polish**

- The per-message debug-log preamble in `Update()` ran `reflect.TypeOf`
  comparisons on every message regardless of log level; it is now gated
  behind `log.GetLevel()` and uses a type switch.
- Cache invalidation on list refresh was O(n²); now a single pass over the
  cache with an ID map.
- The error-help widget was constructed on every `View()` render; now only
  on the error path.
- New `view_render_test.go` smoke-tests the top-level `View()` output per
  focus mode (table, error, incident tabs, log, cluster select) and the
  Enter/Esc/Down key flows.

**Lazy-enricher staleness refresh**

Since list-wide prefetch was removed (plan 055), the lazy enricher fetches
each incident once and never refreshes it — notes and alerts added after the
first fetch don't appear until a status change invalidates the cache. The
enricher now re-fetches incidents whose cached data is older than five
minutes, at the same pace (one incident per 3s `lazyEnrichMsg` tick,
cursor-spiral priority).

## Key design decisions

- **Extend the existing enricher instead of adding a prefetch queue.** An
  earlier draft added a parallel per-tick prefetch engine; it was rejected
  because background alerts responses cascade into OCM/backplane/prior-alert
  fetches via the `selectedIncident == nil` branch of `gotIncidentAlertsMsg`,
  and 6 req/s of background traffic consumes most of the client rate
  limiter's 10 req/s budget. Re-using the 1-per-3s enricher keeps the
  cascade and pacing semantics exactly as they are today — stale incidents
  simply become eligible again.
- **`lastFetched` now means "time of last data write".** It was only set by
  the details handler, so alerts/notes-only entries looked infinitely stale.
  All three `gotIncident*Msg` handlers now stamp it, which also makes a
  stale re-fetch self-limiting: the first response to land resets the TTL.
- **A dispatch-cooldown map is required, not optional.** Failed notes/alerts
  fetches never write the cache, so an incident with a persistently failing
  fetch stays "incomplete" forever; without `enrichDispatchedAt` (30s
  cooldown per incident) it would be re-dispatched every 3 seconds. A
  single-slot guard is insufficient — two failing incidents would
  round-robin around it.

## Files

- `pkg/tui/commands.go` — `UserIsOnCall`, `removeCommentsFromBytes`,
  `login`, `openBrowserCmd`, `requeueAfterDelay`, `needsEnrichment`,
  `pickNextEnrichment`, TTL/cooldown constants
- `pkg/tui/tui.go` — requeue sites, silence guard, debug preamble, cache
  invalidation single pass + dispatch-record pruning, `lastFetched` stamps
  in notes/alerts handlers
- `pkg/tui/model.go` — `defaultScheduledJobs()` factory,
  `enrichDispatchedAt` field + constructor init
- `pkg/tui/views.go` — error-help construction moved to error branch
- `pkg/tui/view_render_test.go` — new focus-mode render smoke tests
- `pkg/tui/lazy_enrich_test.go`, `pkg/tui/commands_test.go`,
  `pkg/tui/update_test.go` — new and updated tests
- `README.md` — background data freshness feature bullet

## Post-mortem / Lessons Learned

(To be filled after merge)
