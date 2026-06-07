# Plan: Integrate Config Wizard INTO the srepd TUI

## Context

The config wizard has been a standalone huh program in `cmd/config.go` — separate from srepd's TUI, with different styles, separate alt-screen sessions, and no access to srepd's header/footer/help. The user wants `srepd config` to run INSIDE the srepd TUI, following the exact pattern the existing team picker already uses. Additionally, `srepd` with no config should automatically enter config mode.

The pure functions built in the previous phase (`resolveExistingConfig`, `resolveFinalValues`, `detectChanges`, `buildFullConfig`, `mergeIntoExistingConfig`, `writeConfig`, `ensureViperDefaults`, etc.) remain the core logic. Only the presentation layer changes.

## Pattern to follow: existing team picker

The team picker in `pkg/tui/tui.go` demonstrates exactly how inline huh forms work within srepd:

1. **State fields** on model struct (`teamSelectMode bool`, `teamSelectForm *huh.Form`, etc.) — `model.go:131-135`
2. **Trigger in Init()** — checks `hasPlaceholderTeamsCfg()`, queues `fetchUserTeams()` command — `tui.go:51-52`
3. **Receive data in Update()** — `fetchedTeamsMsg` handler builds form, sets mode, returns `form.Init()` — `tui.go:188-226`
4. **Focus dispatch** — `switchTeamSelectFocusMode()` forwards keystrokes to form, checks for completion/abort — `msgHandlers.go:240-263`
5. **View()** — renders `m.teamSelectForm.View()` when in mode — `views.go:59`
6. **Completion** — queues `teamsSelectedMsg`, then `writeTeamsToConfigCmd` — `tui.go:228-244`

## What changes

### `pkg/tui/model.go` — add config wizard state

Add fields to model struct (alongside existing `teamSelectMode` at line 131):
```go
configMode       bool
configForm       *huh.Form
configTokenInput string
configSelectedTeams []string
configSilentPolicy  string
configCustomInput   string
configKeepTeams     bool
configKeepSilent    bool
configKeepCustom    bool
configConfirm       bool
configIsNewFile     bool
```

### `pkg/tui/tui.go` — trigger and handle config mode

**In Init()**: Check if config is needed. Two triggers:
- `srepd config` subcommand sets a flag (passed via InitialModel)
- No config file exists → auto-enter config mode

**In Update()**: Handle new message types:
- `configWizardReadyMsg` — config data resolved, build multi-group huh form
- `configSavedMsg` — config written to disk, transition to normal mode

The form uses the same theme as the team picker (`m.theme` fields). Multiple groups: token → keep-teams/select-teams → keep-silent/enter-silent → keep-custom/enter-custom → summary+confirm. Same `OptionsFunc`/`WithHideFunc` pattern as the standalone version, but rendered inside the TUI.

### `pkg/tui/msgHandlers.go` — config mode focus handler

Add `case m.configMode` to the dispatch switch (before team select):
```go
case m.configMode:
    return switchConfigFocusMode(m, msg)
```

`switchConfigFocusMode` follows the exact pattern of `switchTeamSelectFocusMode`:
- Forward msg to form via `m.configForm.Update(msg)`
- Check `StateCompleted` → resolve values, detect changes, validate, write, return `configSavedMsg`
- Check `StateAborted` → exit config mode, set status

### `pkg/tui/views.go` — render config form

Add `case m.configMode` to the View() switch (before team select):
```go
case m.configMode:
    s.WriteString(m.configForm.View())
```

### `pkg/tui/commands.go` — config write command

Add `writeConfigCmd()` that uses the existing pure functions:
- `resolveFinalValues()` from `cmd/config.go`
- `detectChanges()` / `detectChangesForNewFile()`  
- `writeConfig()` via `configFS`
- `ensureViperDefaults()`
- Returns `configSavedMsg`

### `cmd/config.go` — simplify to launcher

`runConfigWizard()` becomes thin: call `ensureViperDefaults()`, then `launchTUI()` with a "config mode" flag. The standalone huh form code is removed. The pure functions stay (they're used by the TUI commands).

### `cmd/root.go` — auto-detect missing config

In PreRun or launchTUI: if no config file exists, pass a flag to InitialModel indicating config mode should start.

## Move pure functions to pkg/tui or shared package

The pure functions (`resolveExistingConfig`, `resolveFinalValues`, `detectChanges`, `buildFullConfig`, `writeConfig`, etc.) currently live in `cmd/config.go`. They need to be accessible from `pkg/tui/commands.go`. Options:
- Move them to a new `pkg/config/` package
- Keep them in `cmd/` and have the TUI commands call back via a function reference passed through the model
- Export them from `cmd/` (but `cmd` importing `pkg/tui` and `pkg/tui` importing `cmd` creates a cycle)

Best approach: move pure config functions to `pkg/config/config.go` with tests in `pkg/config/config_test.go`. The TUI imports `pkg/config`. The `cmd/config.go` becomes a thin launcher.

## Files to modify

| File | Change |
|------|--------|
| `pkg/config/config.go` | **NEW** — move pure functions here |
| `pkg/config/config_test.go` | **NEW** — move tests here |
| `pkg/tui/model.go` | Add config mode state fields |
| `pkg/tui/tui.go` | Init() trigger, Update() handlers |
| `pkg/tui/views.go` | View() config mode case |
| `pkg/tui/msgHandlers.go` | Focus dispatch + handler |
| `pkg/tui/commands.go` | Config write command |
| `cmd/config.go` | Simplify to thin launcher |
| `cmd/root.go` | Auto-detect missing config |

## Tests

### Existing tests move to `pkg/config/config_test.go`
All ~185 tests for pure functions move with them. No logic changes.

### New tests in `pkg/tui/` — flow coverage matrix

**Flow 1: Brand new user — no config file, no env vars**
- `TestConfigMode_TriggeredWhenNoConfigFile` — model created with `configFileExists=false` enters config mode
- `TestConfigMode_NewUserWritesCompleteConfig` — after form completion, writeConfig produces file with token, teams, defaults for editor/terminal/cluster_login_command/toolbox_mode/chord_prefix
- `TestConfigMode_NewUserConfigIsValidYAML` — written file parses as valid YAML with all required keys

**Flow 2: New user — env vars set, no config file**
- `TestConfigMode_EnvVarsPreFillForm` — resolveExistingConfig picks up env var values, form shows "keep" prompts for populated fields
- `TestConfigMode_EnvVarUserSavesCompleteFile` — even when keeping all env var values, writes full config file with defaults

**Flow 3: Existing user — old format config**
- `TestConfigMode_OldFormatMigrated` — resolveExistingConfig reads `service_escalation_policies`, migrates to new keys, shows "keep" prompts
- `TestConfigMode_OldFormatPreservesUnknownKeys` — mergeIntoExistingConfig preserves `ignoredusers`, `log_to_journal`, comments

**Flow 4: Existing user — new format config**
- `TestConfigMode_ExistingNewFormatKeepAll` — all "keep"=true, detectChanges returns no changes, transitions to incident view without writing
- `TestConfigMode_ExistingNewFormatChangeToken` — token changed, file updated, other keys preserved

**Flow 5: Existing user — changes nothing**
- `TestConfigMode_NoChangesLaunchesDirectly` — detectChanges returns false, skips confirm, transitions to incident view

**Flow 6: User ctrl+c aborts**
- `TestSwitchConfigFocusMode_Aborted` — form StateAborted sets configMode=false, no file written, status message set

**State machine tests**
- `TestConfigMode_NotTriggeredOnValidConfig` — model with existing config file skips config mode
- `TestSwitchConfigFocusMode_Completed` — form StateCompleted triggers resolve+write+transition
- `TestSwitchConfigFocusMode_ForwardsKeys` — keystrokes forwarded to huh form when in config mode
- `TestConfigModeView_RendersForm` — View() returns form content when configMode=true

**Config write integration (using pkg/config pure functions)**
- `TestWriteConfigCmd_NewFile` — writeConfigCmd with isNewFile=true produces complete config via buildFullConfig
- `TestWriteConfigCmd_ExistingFile` — writeConfigCmd with isNewFile=false uses mergeIntoExistingConfig, creates backup
- `TestWriteConfigCmd_SetsViperDefaults` — after write, ensureViperDefaults populates missing optional keys in Viper

### Existing tests
- All ~185 tests in `pkg/config/config_test.go` (moved from `cmd/config_test.go`) continue to pass
- Existing TUI tests (model, team picker, commands) unaffected

## Implementation order (TDD)

1. Create `pkg/config/` — move pure functions + tests, verify all pass
2. Update `cmd/config.go` to import from `pkg/config`, verify tests pass
3. Write tests for config mode state in `pkg/tui/` (trigger, completion, abort)
4. Add config mode fields to model struct
5. Implement Init() trigger and Update() handlers
6. Implement focus handler and View() rendering
7. Implement writeConfigCmd in commands.go
8. Simplify `cmd/config.go` to thin launcher
9. Add auto-detect in `cmd/root.go`
10. Full CI suite + build + manual test all flows

## Verification

### Automated (must all pass before asking user to test)
1. `go test ./... -count=1` — all pass including ~200+ tests covering every flow
2. `golangci-lint run --timeout 5m` — 0 issues
3. `go vet ./...` — clean
4. `gofmt -s -l cmd pkg` — clean
5. `CGO_ENABLED=1 go test -race ./...` — no races
6. Source scan tests verify:
   - Exactly one `huh.NewForm` in the config flow (no separate programs)
   - `ensureViperDefaults` called before every code path that launches the TUI
   - Every huh form uses `WithProgramOptions(tea.WithAltScreen())`
7. Unit tests verify for EVERY flow (new user, env vars, old format, new format, no changes, abort):
   - Correct config file YAML output (parsed and key-checked)
   - State transitions (configMode set/cleared, messages dispatched)
   - Existing keys/comments preserved in merge path
   - Backup created for existing files
   - Viper defaults populated before TUI launch

### Manual smoke test (visual confirmation only — logic already proven by tests)
1. `go build -o ~/srepd .` — build binary
2. `~/srepd config` — verify it renders inside srepd's TUI (same styles/header/footer), not a separate window
3. `~/srepd` with no config — verify auto-enters config mode
