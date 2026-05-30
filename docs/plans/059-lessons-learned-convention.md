# 059: Lessons Learned Convention for Plan Documents

## Context

Issue #221. When a PR fixes issues introduced by a previous PR, the
lessons learned are not recorded anywhere. This means the same
mistakes can repeat. For example, PR #190 fixed a broken test caused
by PRs #184 and #186 merging in the wrong order, but no lessons were
documented.

## Plan

Docs-only change: update CONVENTIONS.md and AGENTS.md to require
lessons-learned documentation when a PR fixes issues from a previous
PR.

### Changes

1. **CONVENTIONS.md** -- Add two bullets to the Plan Documents
   section requiring a "Lessons Learned" section in the fixing PR's
   plan doc, and a cross-reference update to the original plan doc.

2. **AGENTS.md** -- Add a bullet to Key Invariants requiring lesson
   documentation in both plan docs when fixing a bug from a previous
   PR.

## Files Modified

- `CONVENTIONS.md` -- Added lessons-learned requirements to Plan
  Documents section
- `AGENTS.md` -- Added lessons-learned invariant to Key Invariants
  section
- `docs/plans/059-lessons-learned-convention.md` -- This plan

## Verification

- All make checks pass (docs-only change, no code affected)
- New bullets are consistent with existing style and formatting
- Requirements match issue #221 specification
