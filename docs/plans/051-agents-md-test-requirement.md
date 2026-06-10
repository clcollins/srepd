# 051: Require Local Test Verification in AGENTS.md

## Status: In Progress

## Problem

AGENTS.md did not explicitly require that all tests pass locally before
committing or pushing code. Developers (human or AI) could create commits
and PRs with failing tests, wasting CI resources and review cycles.

## Solution

Add explicit requirements to AGENTS.md:

1. **Key Invariants** -- new bullet requiring `make test-all` (or at minimum
   `go test ./... -count=1`) to pass with zero failures before any commit or PR.

2. **PR Workflow** -- update step 4 to emphasize that all tests **must** pass,
   add a new step 5 requiring fixes before proceeding, and renumber subsequent
   steps accordingly.

## Changes

- `AGENTS.md` -- Key Invariants section and PR Workflow section updated.

## Verification

- `go test ./... -count=1` passes with zero failures.
- AGENTS.md content reviewed for correctness and clarity.

## Lessons Learned

**GENUINE ERROR — vague test requirement was not actionable**
(Fixed by: [059-agents-md-precommit-checks.md](059-agents-md-precommit-checks.md))

The instruction to "run `make test-all` before committing" was
incomplete — `make test-all` did not cover all CI checks (lint, format,
README check), and the requirement was vague enough that agents and
developers continued to skip checks, causing repeated CI failures.

Why it wasn't caught: the plan focused on adding *a* requirement rather
than verifying that the requirement covered *everything* CI checks.
`make test-all` sounded comprehensive but actually missed several gates.

Prevention: process documentation must be explicit and exhaustive —
list every command, not a summary target that may not cover everything.
The fix replaced the vague instruction with an explicit parallel
checklist of all 7 CI checks.
