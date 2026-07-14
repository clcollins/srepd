# 383: Unify rosa-boundary with the cluster login path

Branch: `srepd/fix-rosa-boundary-direct-execution` (PR #386)

## Problem

rosa-boundary (github.com/openshift-online/rosa-boundary) launches an
interactive session into a protected cluster **exactly like ocm-container**
— it is deliberately importing ocm-container's learnings and will
eventually replace `cluster_login_command`. Its srepd behavior must
therefore be identical to the cluster login: session in a new terminal
window, srepd keeps running, multiple concurrent sessions.

The pre-existing `rosaBoundaryLogin()` got the window behavior right but
was a hand-rolled copy of `login()`'s mechanics with one real bug: it
never passed the `PAGERDUTY_*` environment context that ocm-container
sessions receive.

## History (how this PR got here)

This PR went through three designs; the record matters because the first
two came from misunderstandings:

1. **Original draft** (agent-authored): claimed the new terminal window
   was "unnecessary" and made rosa-boundary execute directly — but kept
   fire-and-forget `Start()` mechanics, which cannot host an interactive
   session while the TUI owns the terminal. The premise was wrong: the
   agent review misunderstood what rosa-boundary is.
2. **First fix**: made direct execution actually work via
   `tea.ExecProcess` (TUI suspends). Mechanically correct, but the wrong
   model — it serializes the SRE to one session at a time, the opposite
   of the ocm-container workflow rosa-boundary is built to match.
3. **Final design (this)**: no rosa-boundary-specific execution path at
   all.

**Lesson**: before "fixing" launch behavior for an external tool, verify
what the tool actually is; the launch model (new window vs current
terminal vs suspend) follows from the tool's interaction model, not from
aesthetics. An agent-review claim like "this window is unnecessary" needs
the same adversarial verification as any other bug report.

## Solution

Delete the special path; share `login()` wholesale:

- Both rosa-boundary call sites (single-cluster and multi-cluster select
  in `pkg/tui/tui.go`) call
  `login(vars, m.rosaBoundaryLauncher, incident, alerts, notes)` — the
  same function as `cluster_login_command`.
- `rosaBoundaryLogin()` and the rosa-specific launcher method are
  removed. rosa-boundary automatically inherits: the terminal wrapper and
  profiles, toolbox `flatpak-spawn --host` handling, `PAGERDUTY_*` env
  injection (all three mechanisms), fire-and-forget launch with
  `loginFinishedMsg`, and any future improvement to `login()`.
- "Eventually replaces cluster_login_command" becomes trivially true:
  it is already the same code path, differing only in which config key
  supplies the command template.

## Tests

- `pkg/launcher/launcher_test.go` — `TestBuildLoginCommand_RosaBoundary`
  documents that a rosa-boundary command template goes through the
  standard terminal-wrapped build; there is deliberately no
  rosa-specific build path to test.
- Env behavior is covered by the existing `buildPagerDutyEnvVars` and
  login-path tests — shared code, shared coverage.

## Docs

- README "rosa-boundary Support" section and config table row: identical
  behavior to `cluster_login_command` (new terminal window, concurrent
  sessions, `PAGERDUTY_*` env).
- `docs/configuration.md`: added the previously undocumented
  `rosa_boundary_command` row.

## Manual Verification (post-merge)

1. From an incident with a cluster, `ctrl+x b` → session opens in a new
   terminal window; srepd keeps running; a second `ctrl+x b` (or a
   cluster login) works concurrently.
2. Point `rosa_boundary_command` at a script that prints
   `env | grep PAGERDUTY` → context variables present in the session.
3. Repeat in toolbox mode (terminal launches on the host).
