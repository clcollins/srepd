# Plan 410: Fix govulncheck CVEs (GO-2026-5970, GO-2026-6061)

## Problem

CI vulnerability scan (`make test-vuln` / `govulncheck`) fails with two
symbol-level CVEs:

1. **GO-2026-5970** — Infinite loop on invalid input in `golang.org/x/text`
   normalization (`norm.Form.*`). Reachable via `http.Client.Do` in the AI
   provider health check and `fmt.Fprintln` in the OCM client.

2. **GO-2026-6061** — Vulnerabilities in the xDS RBAC authorization engine
   and HTTP/2 transport server in `google.golang.org/grpc`. Reachable via
   `transport.*` through the AI streaming client and backplane HTTP client.

Dependabot PR #413 bumped grpc but couldn't pass CI because the x/text
CVE was already present. This PR fixes both together.

## Approach

Dependency-only change — bump three indirect modules:

| Module | From | To | Fixes |
|--------|------|----|-------|
| `golang.org/x/text` | v0.37.0 | v0.39.0 | GO-2026-5970 |
| `google.golang.org/grpc` | v1.82.0 | v1.82.1 | GO-2026-6061 |
| `golang.org/x/sync` | v0.20.0 | v0.21.0 | transitive (x/text) |

## Verification

- `govulncheck ./...` reports 0 symbol-level vulnerabilities
- All unit tests pass
- Race detection clean
- Lint clean
- No code changes, only go.mod/go.sum

## Supersedes

Dependabot PR #413 (`google.golang.org/grpc` 1.82.0 → 1.82.1) — this PR
includes that bump plus the x/text fix. #413 can be closed after merge.
