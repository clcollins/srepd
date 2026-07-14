# 383: Fix rosa-boundary Direct Execution

Branch: `srepd/fix-rosa-boundary-direct-execution` (PR #386)

## Problem

SREPD's rosa-boundary launcher wrapped the command in a terminal emulator
(`gnome-terminal -- rosa-boundary start-task --cluster-id <id> --connect`),
opening a new terminal window unnecessarily. rosa-boundary is a standalone
CLI that manages its own interactive session via `session-manager-plugin`,
unlike `ocm-container`/`ocm backplane login` which need a new terminal so
the TUI can keep running.

rosa-boundary is a **peer of `cluster_login_command`** — and its eventual
replacement — so it must follow the same conventions and code paths as
`login()` wherever the intent doesn't force a difference.

## Root Cause(s)

1. `rosaBoundaryLogin()` (`pkg/tui/commands.go`) built its command with
   `BuildLoginCommand()`, which adds the terminal wrapper.
2. The original execution mechanics — `exec.Command` → `c.Start()` with no
   stdio, `Wait()` in a goroutine — only worked *because* the child was a
   GUI terminal bringing its own TTY. Removing the wrapper without changing
   the mechanics would spawn the interactive session headless while the
   bubbletea TUI kept the terminal: the session had nowhere to exist, and
   errors were only debug-logged.

## Solution

1. **New method** `BuildRosaBoundaryCommand()` (`pkg/launcher/launcher.go`):
   variable substitution only — no terminal wrapper. In toolbox mode it
   prepends `flatpak-spawn --host`, the **same convention as
   `BuildLoginCommand`** (peer parity: srepd in a toolbox runs the command
   on the host).
2. **Execution via `tea.ExecProcess`** (`pkg/tui/commands.go`), mirroring
   the notes editor (`openEditorCmd`): the TUI suspends, the child gets the
   real TTY, and the callback returns `loginFinishedMsg{err}` on exit — so
   session errors surface through the existing handler (status + error
   view) instead of being silently dropped.
3. **Env parity with `login()`**: `buildRosaBoundaryExec()` builds
   `PAGERDUTY_*` context via the shared `buildPagerDutyEnvVars`, passed as
   `flatpak-spawn --env=` flags in toolbox mode or directly on the process
   env otherwise — the same two mechanisms `login()` uses (the
   ocm-container `-e` branch does not apply). rosa-boundary ignores
   variables it does not support. Both call sites (single-cluster and
   multi-cluster select) now pass the selected incident, alerts, and notes.

## Design Decisions

- **Separate method, not a flag on `BuildLoginCommand`** — keeps the
  distinction explicit: terminal-wrapped (login) vs direct (rosa-boundary).
- **Extracted `buildRosaBoundaryExec` helper** — mirrors `login()`'s body
  shape and keeps the `tea.ExecProcess` line trivial glue, so tests can
  assert argv and env on the built `*exec.Cmd`.
- **Toolbox parity (changed from the first draft of this PR)**: the initial
  draft skipped `flatpak-spawn` in toolbox mode; as a peer (and future
  replacement) of `cluster_login_command`, rosa-boundary now follows the
  identical toolbox convention.
- **Known quirk, documented not fixed**: `NewClusterLauncher` still
  requires `terminal` to be set even though direct execution ignores it;
  in practice viper's default (`gnome-terminal`) means it is never empty.

## Tests

- `pkg/launcher/launcher_test.go` — argv-level: direct execution without
  wrapper; toolbox adds `flatpak-spawn --host` (parity with
  `BuildLoginCommand`); multi-variable substitution; comparison against
  `BuildLoginCommand`.
- `pkg/tui/commands_test.go` — `buildRosaBoundaryExec`: direct case sets
  `PAGERDUTY_*` on `c.Env`; toolbox case passes `--env=` flags and leaves
  `c.Env` empty; `rosaBoundaryLogin` returns a `tea.ExecProcess` command
  (not the fire-and-forget immediate `loginFinishedMsg`).

## Docs

- README "rosa-boundary Support" section and config table row: direct
  in-terminal execution, TUI suspend/resume, `PAGERDUTY_*` env vars,
  toolbox convention.
- `docs/configuration.md`: added the previously undocumented
  `rosa_boundary_command` row.

## Manual Verification (post-merge)

1. From an incident with a cluster, `ctrl+x b` → TUI suspends,
   rosa-boundary runs in the same terminal, TUI resumes on session exit.
2. Point `rosa_boundary_command` at a script that prints `env` → verify
   `PAGERDUTY_*` variables are present.
3. Point it at a nonexistent binary → visible "failed to login: …" status.
4. Repeat inside a toolbox (command must exist on the host).
