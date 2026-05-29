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
