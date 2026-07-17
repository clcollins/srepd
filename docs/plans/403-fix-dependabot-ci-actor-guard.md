# 403: Fix Dependabot CI exemption using the wrong actor field

## Problem

PR #398 (`Bump golang.org/x/crypto from 0.51.0 to 0.52.0`) was authored by
`app/dependabot`, but its **Plan Document** check failed — even though
AGENTS.md states that only *non-Dependabot* PRs must pass these checks, and
the workflow appeared to exempt Dependabot:

```yaml
if: github.event_name == 'pull_request' && github.actor != 'dependabot[bot]'
```

The bug is `github.actor`. It is **not** the PR author — it is whoever
triggered the most recent event on the workflow run. Inspecting the failing
run confirmed it:

```
actor:            clcollins
triggering_actor: clcollins
head_branch:      dependabot/go_modules/golang.org/x/crypto-0.52.0
```

Any interaction with a Dependabot PR — rerunning checks, pushing a rebase,
closing and reopening — restamps the actor as a human and permanently
un-exempts the PR for that run. The guard fails *open*.

This affected all three guarded jobs (plan-doc, readme-check,
quickstart-check), not just plan-doc. The other two only *appeared* healthy
because their make targets no-op on a dependency bump: `readme-check` only
fires when `keymap.go` / `root.go` / `commands.go` change, so it passes
trivially on a go.mod bump regardless of whether the guard worked.
`plan-check` is simply the only one strict enough to expose the bug —
it hard-fails whenever no `docs/plans/*.md` is in the diff.

## Approach

Use the PR author, which is a stable property of the pull request and does
not change based on who triggered the run:

```yaml
if: github.event_name == 'pull_request' && github.event.pull_request.user.login != 'dependabot[bot]'
```

Applied to all three jobs. Also adds a `skip-plan` label bypass to plan-doc,
mirroring the existing `skip-readme` / `skip-quickstart` escape hatches —
plan-doc previously had no manual override at all, so a legitimately
plan-free PR had no way past it.

Verified no `github.actor` guards remain anywhere in `.github/workflows/`.

## Notes / lessons

- **`github.actor` is an event property, not a PR property.** For any
  "is this PR from a bot?" decision, use
  `github.event.pull_request.user.login`. `github.actor` answers a different
  question ("who most recently poked this?") and silently gives the right
  answer until a human touches the PR — which is exactly when the check is
  least likely to be scrutinized.
- **A passing check is not evidence a guard works.** readme-check and
  quickstart-check passed on #398 while sharing the identical broken guard;
  they pass on dependency bumps either way. When verifying a conditional,
  confirm the condition evaluated as intended, not just that the job was
  green.
