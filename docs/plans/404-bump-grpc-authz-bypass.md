# 404: Bump google.golang.org/grpc for CVE-2026-33186

## Problem

GitHub flagged a **critical** Dependabot alert on `main`:

- **CVE-2026-33186** / **GHSA-p77j-4mvh-x3m3**
- `google.golang.org/grpc` v1.64.1, vulnerable range `< 1.79.3`
- "gRPC-Go has an authorization bypass via missing leading slash in `:path`"

The flaw: a gRPC-Go **server** accepted HTTP/2 requests whose `:path`
omitted the mandatory leading slash (`Service/Method` rather than
`/Service/Method`). The server routed them correctly, but authorization
interceptors evaluated the raw non-canonical string, so a "deny" rule
written against `/Service/Method` failed to match — and any fallback
"allow" rule let the request through.

## Is srepd exposed? No.

The advisory requires **both**:

1. Running a gRPC **server**, and
2. Path-based authorization interceptors (`grpc/authz`, or custom
   interceptors reading `info.FullMethod` / `grpc.Method(ctx)`).

srepd is a terminal client. It has neither. Three independent checks agree:

- `grep` for `grpc.NewServer|RegisterService|grpc/authz|UnaryInterceptor|net.Listen`
  across `cmd/` and `pkg/` — no matches.
- `go mod why google.golang.org/grpc` — arrives only as a client transport:
  `pkg/ai` → `anthropic-sdk-go/vertex` → `google.golang.org/api/option` → `grpc`.
- `govulncheck` — "Your code is affected by 0 vulnerabilities."

The "critical" rating is the advisory's severity for a vulnerable *server*,
not a measure of srepd's exposure. **v1.6.3 is not affected and does not
need re-cutting.**

## Approach

Bump anyway, for hygiene rather than urgency: a standing critical alert on
`main` trains us to ignore the alert list, and grpc 1.64.1 (via
`google.golang.org/api` v0.189.0) is well over a year stale.

`go get google.golang.org/grpc@v1.82.0` (current stable, well past the
1.79.3 patch floor) plus `go mod tidy`.

## Dependency changes (all indirect)

Per AGENTS.md, dependency changes are called out explicitly. Upgraded:

| Module | From | To |
|--------|------|-----|
| `google.golang.org/grpc` | v1.64.1 | v1.82.0 |
| `go.opentelemetry.io/otel` (+ `/metric`, `/trace`) | v1.24.0 | v1.43.0 |
| `google.golang.org/protobuf` | v1.34.2 | v1.36.11 |
| `google.golang.org/genproto/googleapis/rpc` | 20240722… | 20260414… |
| `golang.org/x/oauth2` | v0.30.0 | v0.36.0 |
| `cloud.google.com/go/compute/metadata` | v0.5.0 | v0.9.0 |
| `github.com/cespare/xxhash/v2` | v2.2.0 | v2.3.0 |
| `github.com/go-logr/logr` | v1.4.2 | v1.4.3 |
| `github.com/golang/glog` | v1.2.4 | v1.2.5 |

Added: `go.opentelemetry.io/auto/sdk` v1.2.1 (new transitive requirement of
otel 1.43.0). Pruned by tidy: `github.com/rogpeppe/go-internal`,
`gopkg.in/check.v1`.

No new **direct** dependencies. Every module above is an existing indirect
dep from an org-backed upstream (Google, OpenTelemetry, Go team) — no new
import paths from unfamiliar publishers, so no typosquat surface.

## Verification

Full local suite, all green: fmt-check, vet, lint (0 issues), test,
test-race, build.

`govulncheck` improved measurably:

- Before: 1 vulnerability in imported packages, 14 in required modules
- After: **0** in imported packages, 1 in required modules

(The remaining "1 in modules you require" is unreachable — `govulncheck`
reports 0 vulnerabilities that the code actually calls, both before and
after.)
