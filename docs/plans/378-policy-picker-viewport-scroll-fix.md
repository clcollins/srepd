# 378: Policy picker cursor and team-scope fixes (OB-4 follow-up)

Issue: follow-up to #367 (OB-4 picker); the picker appeared to show only
the skip/manual escapes, and the fetched list missed the actual silent
policy
Branch: `srepd/ob4-policy-picker-optionsfunc`

## Problems

1. **Picker appeared to show only the escapes.** After the OB-4 picker
   shipped (#367), the policy Select never appeared to show the fetched
   escalation policies — only "Skip — configure later" and "Enter an ID
   manually…" — until the user pressed an arrow key. Four different
   `OptionsFunc` binding strategies all "failed" identically, which
   pointed away from the suspected binding-hash instability.
2. **The fetched list missed the real silent policy.** With team
   `PASPK4G` (Platform SRE) selected, the team-filtered query returned 6
   policies — none of them the intended silent policy `PCGXUDY`.

## Root causes

1. A deterministic message-pump test
   (`config_policy_picker_repro_test.go`) driving the real wizard form
   proved the async `updateOptionsMsg` was **accepted** by huh
   (`msg.hash == bindingsHash`, options installed). The policies were
   rendering all along — scrolled out of view: huh v1.0.0
   `Select.selectOption()` (field_select.go:191) moves the cursor to the
   option whose value equals the current bound value, then sets
   `viewport.YOffset = s.selected` **unclamped** (line 203;
   `updateViewportHeight` repeats the unclamped assignment on every
   Update, line 543). The picker's bound value starts as `""`, which was
   also the skip sentinel's value, so the cursor jumped to "Skip" below
   the policies and the viewport scrolled past every policy. The
   binding-hash mechanism was never the problem.
2. Verified against the live PagerDuty API (shim-mcp): `PCGXUDY`
   ("Silent Test - Non-Actionable") belongs to team `PRSN7UG`
   "Platform SRE - Non-actionable" — a companion team the user is a
   member of but never selects for incident filtering. A
   selected-teams-filtered `/escalation_policies` query can never return
   it. Querying across all 6 of the user's teams returned 18 policies
   including `PCGXUDY` and `PVBANNN` (the DMS silent-test policy from
   the user's custom mappings), with 8 SILENT-classified candidates.

## Approach

- `policyChoiceSkip` becomes a non-empty sentinel (`"__skip__"`), so the
  initial empty bound value matches **no** option and huh leaves the
  cursor at index 0 with the viewport at the top.
  `resolveSilentPolicyChoice` maps skip and the untouched empty choice to
  `""` — the saved format is unchanged.
- `buildPolicyOptions` keeps the original recommended-first ordering
  (SILENT-classified annotated, then the rest, then the escapes); the
  cursor now starts on the first recommended policy. When an existing
  config's silent policy is in the list, the cursor lands on it instead.
- `fetchPolicyOptions` drops its selected-teams parameter and fetches
  policies for **all** of the user's PagerDuty teams
  (`GetCurrentUserTeams` → `GetTeamEscalationPolicies`), since silent
  policies conventionally live on companion non-actionable teams. The
  `OptionsFunc` binding moves to the token input (the fetch no longer
  depends on team selection).
- Mock gains `RecordedListEscalationPoliciesOpts` so tests can assert
  the team scope actually sent.

### UX polish (from live testing)

- **Loading indicator**: huh natively renders a spinner + "Loading..."
  row while an async OptionsFunc fetch is in flight, but it only appears
  once a message arrives >25ms after dispatch — and srepd's Update loop
  intercepted every `spinner.TickMsg` and returned early, so the form
  received nothing and the list stayed blank during the fetch. The
  spinner case now forwards ticks to the config form; the loading row
  renders and animates, and is replaced when the list arrives.
- **Annotation**: "(recommended — no schedules notified)" →
  "(possible candidate — does not page)".
- **Ordering**: within the SILENT-classified candidates, policies with
  "silent" in the name (case-insensitive) sort first — the strongest
  signal of intent — so the cursor starts on the likeliest pick.
- **Height and scrolling**: huh defaults a dynamic-options Select to 10
  lines; title plus the 5-line description left only 4 visible options.
  Worse, any *explicit* field height routes huh v1.0.0 through an
  unclamped `viewport.YOffset = selected` on every update
  (updateViewportHeight, field_select.go:543), pinning the selected row
  to the top so arrow keys scrolled the list under a stationary cursor.
  The picker now sets `Height(0)` (after OptionsFunc, which
  force-defaults zero heights to 10): the field auto-sizes to the full
  list, arrows move the cursor naturally, and the group layout clamps
  the field back to scrolling mode only when the window is too short.

## Tests (TDD — written first)

- `config_policy_picker_repro_test.go`: deterministic bubbletea pump that
  drives the real form (token → teams → policy) with async fetch results
  arriving late; asserts fetched policies render, render *above* the
  escapes, and the cursor preselects the first recommended policy.
- `config_policy_picker_test.go`: ordering (silent-annotated first,
  escapes last, no empty-valued options), all-user-teams fetch scope,
  no-token escapes, classified error row, resolve mapping incl. the new
  skip sentinel and untouched-empty choice.

## Lessons learned

- "Options never render" had two plausible mechanisms (message dropped
  vs. rendered-but-hidden); four fix attempts targeted the wrong one
  because the diagnosis was never verified against huh's actual guard.
  Instrumenting the vendored dependency and asserting on the *rendered
  view* settled it in one run.
- Verify data-shape assumptions against the live API before shipping a
  fetch-driven UI: the "obvious" scope (selected teams) structurally
  excluded the one policy the feature exists to find.
- The srepd Update loop intercepts `spinner.TickMsg` (tui.go) and returns
  early, so huh's loading spinner never animates in config mode and the
  form receives almost no messages while a fetch is in flight.
- Upstream: huh v1.0.0 should clamp the viewport offset in
  `selectOption`/`updateViewportHeight` (`viewport.SetYOffset` clamps;
  direct assignment does not). Worth an upstream issue; v2 may already
  differ.
