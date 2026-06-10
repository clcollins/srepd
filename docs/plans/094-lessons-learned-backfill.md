# 094: Backfill Lessons Learned sections in originating plan documents

**Status:** Complete

## Problem

The Agentic SDLC convention (059-lessons-learned-convention) requires
that when a later PR fixes an issue from an earlier PR, the earlier
plan document gets a "Lessons Learned" section. Most originating plan
documents did not have this section, meaning lessons from fix PRs were
not recorded where future agents and developers would find them.

## Solution

Analyzed all 112 plan docs (plus PR #325's plan doc) to identify fix
relationships. A workflow read every plan in parallel batches, extracted
cross-references and fix/supersede relationships, then synthesized
lessons learned using the Agentic SDLC template categories (GENUINE
ERROR, PROCESS GAP).

Added Lessons Learned sections to 10 originating plan documents
covering 15 fix relationships:

| Originating Plan | Fixed By | Lesson |
|---|---|---|
| 003-ci-infrastructure | 021, 058 | Double CI triggers; silent Codecov failures |
| 013-normalize-key-handlers | 059-fix-actions | Normalization missed other focus modes |
| 032-async-log-writer | 042-fix-asyncwriter | Component silently abandoned by cobra init |
| 038-toolbox-detection | 044-toolbox-env-var | Env vars lost across flatpak-spawn boundary |
| 049-fix-broken-details | 059-lessons-learned | No convention for post-mortems |
| 051-agents-md-test-req | 059-agents-md-precommit | Vague instructions not actionable |
| 055-claude-cli | 061, 092, 093 | Hardcoded binary; key conflicts; fallthrough dispatch |
| 055-dev-mode | 059-fix-reorder, 066-fix-fixture | Map ordering; incomplete fixtures |
| 057-license-ai-note | 077-fix-license | Non-standard LICENSE broke tooling |
| 083-replace-ignoredusers | 084-remove-config | Deprecation left stale references |

## Files Modified

- `docs/plans/003-ci-infrastructure.md` -- added Lessons Learned (2 entries)
- `docs/plans/013-normalize-key-handlers.md` -- added Lessons Learned
- `docs/plans/032-async-log-writer.md` -- added Lessons Learned
- `docs/plans/038-toolbox-detection.md` -- appended to existing Lessons Learned
- `docs/plans/049-fix-broken-details-test.md` -- added Lessons Learned
- `docs/plans/051-agents-md-test-requirement.md` -- added Lessons Learned
- `docs/plans/055-claude-cli-integration.md` -- added Lessons Learned (3 entries)
- `docs/plans/055-dev-mode.md` -- added Lessons Learned (2 entries)
- `docs/plans/057-license-ai-note.md` -- added Lessons Learned
- `docs/plans/083-replace-ignoredusers.md` -- added Lessons Learned
- `docs/plans/094-lessons-learned-backfill.md` -- this plan

## Verification

- Docs-only change, no code affected
- `grep -c "## Lessons Learned" docs/plans/*.md` confirms 15 files now
  have the section (6 pre-existing + 9 new + 1 appended)
