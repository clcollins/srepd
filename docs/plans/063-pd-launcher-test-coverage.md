# 063: Test Coverage for pkg/pd, pkg/launcher, and pkg/container (0% Functions)

## Problem

Six functions across three packages have 0% test coverage:

- `pkg/pd/pd.go` - `NewConfig()` and `newClient()`
- `pkg/pd/dev.go` - `NewDevConfig()`
- `pkg/launcher/launcher.go` - `Profile()`
- `pkg/launcher/claude.go` - `HasClaudeCode()`
- `pkg/container/container.go` - `IsRunningInToolbox()`

Issue: #205

## Solution

### pkg/pd/pd.go - NewConfig and newClient

**NewConfig refactoring**: Extract the body of `NewConfig()` into a new
exported function `NewConfigWithClient()` that accepts a pre-built
`PagerDutyClient` instead of a token string. `NewConfig()` becomes a
thin wrapper that calls `newClient(token)` then delegates to
`NewConfigWithClient()`. This removes the duplicated test helper
`newConfigFromClient` from `pd_test.go`.

**newClient**: Test that `newClient()` returns a non-nil
`RateLimitedClient` wrapping a real PD client.

**NewConfigWithClient tests** (table-driven, using MockPagerDutyClient):
- Success: valid teams, policies, ignored users
- Error: missing "default" escalation policy key
- Error: missing "silent_default" escalation policy key
- Error: GetEscalationPolicyWithContext fails (ID="err")
- Error: bad ignored user (ID="err")
- Multiple teams
- Policy keys uppercased
- With ignored users

### pkg/pd/dev.go - NewDevConfig

Test that `NewDevConfig()` creates a valid Config from fixture files:
- Success: verify Config fields (Client, CurrentUser, Teams, etc.)
- Error: nonexistent fixtures directory

### pkg/launcher/launcher.go - Profile

Test that `Profile()` returns the profile set during construction:
- Create ClusterLauncher via `NewClusterLauncherWithToolbox`, verify
  `Profile()` returns expected TerminalProfile

### pkg/launcher/claude.go - HasClaudeCode

`HasClaudeCode()` is a one-line public wrapper calling
`hasClaudeCodeWith(exec.LookPath)`. The internal function already has
100% coverage. Test the public function by verifying it returns a bool
(the actual return value depends on the test environment).

### pkg/container/container.go - IsRunningInToolbox

`IsRunningInToolbox()` is a one-line public wrapper calling
`checkToolbox(defaultToolboxEnvPath, os.Getenv)`. The internal function
already has 100% coverage. Test the public function by verifying it
returns a bool without panicking.

## Implementation Plan

1. Write plan document (this file)
2. Add `NewConfigWithClient()` to `pkg/pd/pd.go`
3. Refactor `NewConfig()` to delegate to `NewConfigWithClient()`
4. Remove `newConfigFromClient` test helper from `pd_test.go`
5. Update existing tests to use `NewConfigWithClient()` directly
6. Add test for `newClient()`
7. Add test for `NewDevConfig()` success and error
8. Add test for `Profile()` getter
9. Add test for `HasClaudeCode()` public wrapper
10. Add test for `IsRunningInToolbox()` public wrapper
11. Run all 7 CI checks
12. Commit

## Testing

All existing tests continue to pass. New tests cover the 6 functions
at 0% coverage. Run `make test-all` before committing.

## Post-Mortem / Lessons Learned

(To be filled after merge)
