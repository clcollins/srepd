# Fix CI workflow running twice on PR branches

## Context

The GitHub Actions CI workflow triggers on both `push` to
`srepd/**` branches AND `pull_request` to `main`. When a commit
is pushed to a branch that has an open PR, both triggers fire,
causing every CI job to run twice. This doubles runner usage for
no benefit.

Predecessor: [003-ci-infrastructure.md](003-ci-infrastructure.md)

## Plan

Remove `srepd/**` from the `push` trigger. The `push` to `main`
trigger stays (catches direct merges and post-merge validation).
The `pull_request` trigger covers all PR branches regardless of
naming convention.

## Files Modified

- `.github/workflows/go-ci.yml` — remove `srepd/**` from push
  branches

## Verification

- Push to a PR branch triggers only ONE CI run (pull_request)
- Push directly to main still triggers CI (push)
- All CI jobs still run on PRs
