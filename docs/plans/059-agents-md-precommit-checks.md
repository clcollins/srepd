# Require full parallel CI suite in AGENTS.md pre-commit checks

## Context

Agents and developers were skipping CI checks before pushing,
causing repeated CI failures on PRs (lint, format, README check).
The AGENTS.md instructions said "run make test-all" but that was
vague and didn't cover all checks.

## Plan

Replace the vague instruction with an explicit parallel checklist
of all 7 CI checks that must pass before every commit/push.

## Files Modified

- `AGENTS.md` — replaced Key Invariants and PR Workflow sections
  with explicit pre-commit check commands

## Verification

- All 7 checks pass locally before pushing
