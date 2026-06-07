# 085: Default silent policy discovery via --pick-teams

Issue: #269, Phase 4
Branch: `srepd/auto-discover-policies`

## Problem

The `service_escalation_policies` config required manually looking up
PagerDuty policy IDs. Initial auto-discovery attempts fetched all ~500
team services at startup, causing 30s delays.

## Key insight

Re-escalation uses the incident's own policy — no config needed. Only
silencing needs a target policy. One "default silent policy" covers most
cases, with rare per-service overrides.

## Solution

Replace `service_escalation_policies` with:
- `default_silent_escalation_policy`: single policy ID, auto-discovered
  via `--pick-teams` or set manually. 1 API call at startup.
- `custom_service_escalation_policies`: optional per-service overrides.

### --pick-teams flow

After team selection: fetch team's escalation policies via
`ListEscalationPoliciesWithContext` (~1 API call), classify as
REAL/SILENT, present SILENT policies for selection via `huh.Select`,
write chosen ID to config.

### Runtime

1 API call for silent policy + 0-3 for custom overrides. Instant startup.

## Changes

- Added `ListEscalationPoliciesWithContext` to PD interface + wrappers
- Added `GetTeamEscalationPolicies()` for paginated policy fetch by team
- Updated `NewConfigWithClient` with three paths: old (deprecated),
  new (default_silent + custom overrides), empty (silencing disabled)
- Extended `--pick-teams` with silent policy discovery and selection
- Deprecated `service_escalation_policies`
- Backward compatible: old config still works with deprecation warning
