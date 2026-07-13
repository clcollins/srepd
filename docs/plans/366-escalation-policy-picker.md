# 366: Escalation policy picker (OB-4)

Issue: #353 (OB-4), #324 item 1; part of the onboarding overhaul
Branch: `srepd/escalation-policy-picker`

## Problem

The wizard's silent-policy step was a free-text ID input with "find the ID in
the PagerDuty URL" instructions — the exact scavenger hunt this overhaul
exists to eliminate. The plumbing to do better already existed:
`GetTeamEscalationPolicies` (team-filtered, paginated) and
`ClassifyEscalationPolicy` (SILENT = no schedule targets) in pkg/pd.

## Approach

- Replace the free-text group with a `huh.Select[string]` whose `OptionsFunc`
  is keyed on `&configState.SelectedTeams` (re-fires when the team selection
  changes; falls back to existing teams when keeping):
  - `fetchPolicyOptions(clientFactory, tokenInput, existingToken, teams)` —
    fetches team policies; API errors render as a classified (OB-3),
    skip-valued row so selecting them is harmless.
  - `buildPolicyOptions(policies)` — SILENT-classified first with
    "(recommended — no schedules notified)" annotation, then the rest, then
    always-present escapes: "Skip — configure later" → `""` and "Enter an ID
    manually…" → `policyChoiceManual`.
- The free-text input group is retained for the weird cases, revealed via
  `WithHideFunc` only when the picker choice is `policyChoiceManual`.
- `resolveSilentPolicyChoice(choice, manualInput)` maps the pair to the final
  bare policy ID at completion (both `WizardInputs` construction sites) — the
  save format and runtime consumption are byte-identical to the free-text
  flow, so no changes to WriteConfig or pd.NewConfigWithClient.
- Picker pre-selects the existing policy ID when present (both
  `SilentPolicyChoice` and the manual field seed from
  `existing.SilentPolicy`). If the existing ID belongs to a team outside the
  selection it simply won't highlight — skip/manual still available.
- Mock: added `ListEscalationPoliciesErr` to `MockPagerDutyClient`.
- Deliberately NO service picker: `custom_service_escalation_policies` stays
  free text (plan 085's bulk service fetch warning; mappings are moving to
  the advanced path later in this overhaul).

## Tests (TDD — written first)

`pkg/tui/config_policy_picker_test.go`:
- `buildPolicyOptions`: SILENT ordering + recommendation annotation; escapes
  always present (even with zero policies)
- `fetchPolicyOptions`: team-filtered fetch via mock; no-teams → escapes
  only; API error → classified, skip-valued row + escapes
- `resolveSilentPolicyChoice`: picker wins / skip empty / manual trimmed /
  manual blank
