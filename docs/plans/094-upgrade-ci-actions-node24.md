# Plan: Upgrade GitHub Actions to Node.js 24-compatible versions (#311)

**Status:** Complete
**Issue:** #311
**PR:** TBD

## Problem

GitHub Actions CI emits deprecation warnings for Node.js 20 actions.
After June 16, 2026, actions will be forced to run with Node.js 24 and
may break.

## Solution

Bump all action version pins to Node.js 24-compatible releases:

- `actions/checkout` v4 → v6
- `actions/setup-go` v5 → v6
- `actions/cache` v4 → v5
- `codecov/codecov-action` v5 → v6

## Files changed

- `.github/workflows/go-ci.yml` — 14 version pin updates
- `.github/actions/setup-go-from-mod/action.yml` — 1 version pin update

## Post-mortem / lessons learned

None — straightforward version bumps with no behavioral changes.
