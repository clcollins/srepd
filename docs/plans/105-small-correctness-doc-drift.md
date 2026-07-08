# Plan 105: Small correctness fixes and documentation drift

**Branch:** `srepd/small-correctness-docs`

## Problem

A batch of small, localized correctness issues and stale docs surfaced in review.

## Changes

### Correctness (with tests)

- **`pkg/tui/claude.go` — panic on whitespace-only agent command.**
  `handleClaudePrompt` did `strings.Fields(agentCmd)[0]`. A whitespace-only
  `agent_cli_command` is non-empty (skips the `""` default) but `strings.Fields`
  returns an empty slice → index panic. Added a `len(fields)==0` guard (mirrors
  `agentQuery`).
- **`pkg/pd/pd.go` — nil-pointer panics.** `ReassignIncidents` and
  `ReEscalateIncidents` dereferenced `user.Email` (and `policy.ID`) with no nil check.
  `ReEscalateIncidents` receives `p.CurrentUser` with no guard at the call site. Added
  nil guards returning errors (never panic in library code).
- **`pkg/backplane/client.go` — unchecked type assertion.**
  `http.DefaultTransport.(*http.Transport)` panics if a dependency reassigns the
  mutable global. Added comma-ok with a fresh-transport fallback.
- **`pkg/ocm/client.go` — missing timeouts.** `GetBackplaneURL` used `.Send()` and
  `GetAccessToken` used `.Tokens()`, both without a context. Switched to
  `SendContext`/`TokensContext` with a 30s `ocmRequestTimeout`.

### Documentation drift

- `CONVENTIONS.md`: Go `1.26.3`→`1.26.4`; `test-all` list now includes `test-race`
  and `test-fixtures`; corrected the build-output note (`make install` →
  `$GOPATH/bin`, `install-local` → `~/.local/bin`; no `/tmp`).
- `AGENTS.md`: `test-all` list updated; Key Files table adds `cmd/update.go` and
  `cmd/asyncwriter.go`, and clarifies `cmd/config.go`.

## Tests (TDD)

Written first, seen red (panics), then green:
- `TestHandleClaudePrompt_WhitespaceOnlyCommand` — no panic, flashes an error, does
  not start a query.
- `TestReassignIncidents_NilUser`, `TestReEscalateIncidents_NilUser` — return an error,
  not a panic.

Existing reassign/reescalate/claude tests remain green.

## Verification

`make test-all` green (fmt, vet, lint, test, race, test-fixtures).

## Lessons Learned

- `strings.Fields(s)[0]` is only safe after a `len()==0` check — whitespace-only
  strings are non-empty but tokenize to nothing.
- Library functions that dereference pointer params must nil-check and return an error,
  never panic. Guard even "internal" callers (`p.CurrentUser`).
- `http.DefaultTransport` is a mutable global — always comma-ok the assertion.
- Docs that hardcode a version or a target list drift; keep them matched to the
  Makefile/go.mod source of truth.
