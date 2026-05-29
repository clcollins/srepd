# Plan 018: Add config validation tests and bump cobra to 1.10.1

## Status: Complete

## Objective

Add unit tests for the `validateConfig()` function in `cmd/config.go` (previously at 0% coverage), then safely upgrade `github.com/spf13/cobra` from v1.9.1 to v1.10.1.

## Background

- `cmd/config.go` contains `validateConfig()` which validates required config keys (token, teams, service_escalation_policies with DEFAULT and SILENT_DEFAULT sub-keys) and sets defaults for optional keys.
- cobra v1.10.0 introduced a pflag breaking change; v1.10.1 fixed it.
- pflag was also bumped from v1.0.6 to v1.0.9 as a transitive dependency.
- Dependabot PR #128 proposed this cobra update but lacked pre-update regression tests. This PR supersedes it.

## Approach

### Phase 1: Pre-update tests (TDD safety net)

Write tests for `validateConfig()` before upgrading cobra, ensuring the existing behavior is captured and any regression from the upgrade would be caught.

Tests written in `cmd/config_test.go`:
- `TestValidateConfig_AllRequiredKeys` - all required keys set, no error
- `TestValidateConfig_MissingToken` - omit token, error contains "token"
- `TestValidateConfig_MissingTeams` - omit teams, error contains "teams"
- `TestValidateConfig_MissingEscalationPolicies` - omit service_escalation_policies, error
- `TestValidateConfig_MissingDefaultPolicy` - policies present but missing DEFAULT key, error
- `TestValidateConfig_MissingSilentDefaultPolicy` - policies present but missing SILENT_DEFAULT, error
- `TestValidateConfig_MultipleErrors` - all keys missing, all errors reported
- `TestValidateConfig_OptionalKeysGetDefaults` - optional keys get default values when absent

Each test uses `viper.Reset()` to isolate state.

### Phase 2: Dependency update

- `go get github.com/spf13/cobra@v1.10.1`
- `go mod tidy`
- Verified all tests pass, go vet clean, gofmt clean

## Changes

| File | Change |
|------|--------|
| `cmd/config_test.go` | New file: 8 test cases for validateConfig() |
| `go.mod` | cobra v1.9.1 -> v1.10.1, pflag v1.0.6 -> v1.0.9 |
| `go.sum` | Updated checksums |

## Verification

- All tests pass: `go test ./... -count=1`
- go vet clean: `go vet ./...`
- gofmt clean: `gofmt -s -l cmd pkg`

## Notes

- Supersedes Dependabot PR #128
- v1.10.0 had a pflag breaking change but v1.10.1 fixed it, making this a safe upgrade path
