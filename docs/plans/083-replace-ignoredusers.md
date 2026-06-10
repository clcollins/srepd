# 083: Replace ignoredusers with auto-discovery from escalation policies

Issue: #269
Branch: `srepd/replace-ignoredusers`

## Problem

The `ignoredusers` config key requires manually looking up PagerDuty
user IDs for bot/silent users. This information is already present in
the escalation policies: silent policies target only `user_reference`
bot users and never route to on-call schedules.

## Solution

Auto-discover ignored users by classifying escalation policies:

- **REAL policies** have at least one `schedule_reference` target
  (incidents reach on-call humans)
- **SILENT policies** have only `user_reference` targets
  (incidents go to bots only)

Extract all `user_reference` target IDs from SILENT policies as the
ignored user set.

## Changes

### New functions in `pkg/pd/pd.go`

- `ClassifyEscalationPolicy(policy) string` — returns `"REAL"` or
  `"SILENT"` based on whether any target is a `schedule_reference`
- `ExtractSilentPolicyUsers(policies) []string` — collects deduplicated,
  sorted user IDs from all SILENT policies

### Modified `NewConfigWithClient`

- If `ignoredUsers` is non-empty: use manual list + log deprecation
- If empty/nil: auto-discover via `ExtractSilentPolicyUsers`
- Auto-discovery fetch failures are warnings (bot accounts may be
  deleted), manual fetch failures are errors

### Mock enhancement

- Added `EscalationPolicyResponses` map to `MockPagerDutyClient` for
  returning policies with `EscalationRules` in tests

### Deprecation

- Added `ignoredusers` to `pkg/deprecation/deprecation.go`

## Verification

- `make test-all` passes
- Remove `ignoredusers` from config → auto-discovery produces same list
- Keep `ignoredusers` → deprecation warning, still works
- No SILENT policies → `IgnoredUsers` is empty

## Future work

- Phase 3: Escalation level filtering (`ctrl+x e` chord)
- Phase 4: Service discovery via `ListServicesWithContext`
- Phase 5: Full `ignoredusers` removal after deprecation period

## Lessons Learned

**GENUINE ERROR — deprecation left stale references in templates and help**
(Fixed by: [084-remove-ignoredusers-config.md](084-remove-ignoredusers-config.md))

This plan added auto-discovery to replace `ignoredusers` but left
behind references to the deprecated key in the `config --create`
template, the optional keys help text, and the README config table.
Users following the config template would still see and potentially
configure the deprecated key.

Why it wasn't caught: the deprecation focused on the runtime behavior
(auto-discovery vs manual list) without auditing all user-facing
references to the key. No checklist existed for deprecation cleanup.

Prevention: when deprecating any user-facing configuration, grep the
entire codebase for the key name and remove/update all references
atomically in the same PR — templates, help text, README, examples,
and comments.
