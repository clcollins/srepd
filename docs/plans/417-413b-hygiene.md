# 417 — 413b Hygiene PR

## Problem

After plan 413 merged (commit `bccb0fb`), eight defects were identified
in the AI paths through manual audit. Two are safety-critical (B1, B4),
and the remainder are correctness/robustness issues that would surface
under normal usage.

## Defects and Fixes

| Bug | Summary | Files Changed |
|-----|---------|---------------|
| B1 | Approval writes to wrong incident — `buildAskFromVerdict` captures live `m.selectedIncident`; user can switch incidents between creation and acceptance | `model.go`, `approvals.go`, `commands.go`, `ask_wiring_test.go` |
| B4 | Terminal injection via AI output — ANSI CSI, OSC-52 clipboard writes, and C0 control chars pass through to the terminal | `watcher.go`, `watcher_test.go` |
| B3 | `readAgentSessionCmd` drops final Result ~50% of the time — single `select` randomly picks `Done()` over `Events()` when both are ready | `claude.go`, `claude_test.go` |
| B2+B8 | Spawn deadlock + pipe leak — detection `select` has no timeout/ctx case; retry-as-resume leaks stdin/stdout pipes | `session.go`, `session_test.go` |
| B5 | Stale stream clobbers successor — Done/chunk messages carry no channel identity, so a superseded stream nils the new stream's cancel func | `stream.go`, `claude.go`, `tui.go`, `model.go`, `claude_test.go`, `watcher_integration_test.go` |
| B6 | No in-flight guard on `:agent` — submitting while a query is active issues concurrent readers on the session channel | `claude.go`, `claude_test.go` |
| B7 | Untested security gates — `ClaudeArgs`, `ValidateUserFlags`, `extractToolRunnerFactory`, `askKindLabel` had no direct unit tests | `session_test.go`, `investigation_test.go`, `approvals_test.go` |
| B9 | Bedrock region not validated — `newBedrockProvider` doesn't check for a discoverable region, unlike Vertex | `bedrock.go`, `bedrock_test.go`, `factory_test.go`, `provider_test.go` |

## Cleanup

- `index.load`: warning said "truncated" (wrong), logged raw payload (no customer data in logs) — now says "ignored" and logs only byte length
- UTF-8 truncation: four call sites used byte-level `s[:N]` which splits multi-byte runes — now rune-aware
- `inferAskKind`: `"oc "` substring false-positived on `"doc "`, `"adhoc "` — now requires word boundary
- Deleted write-only `Session.err` field and dead `LastUsed` from session index entry
- Added `TODO(phase-2)` comment on `PermissionAsk` handler

## Approach

- B1 and B4 committed first (safety items)
- TDD: failing test committed before each fix where possible; revert checks for B1, B4, B3, B7
- Each fix is a separate commit with traceability to the bug ID

## Lessons (for 413)

Closures over live model state in deferred-action patterns (approvals,
typed commands) are a recurring source of identity races. Snapshot the
identity at the point of creation, not at the point of execution. The
same principle applies to stream messages — every message must carry
enough identity to be routed correctly even when superseded.
