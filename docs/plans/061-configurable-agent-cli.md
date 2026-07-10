# Plan: Configurable CLI Agent Command

**Issue:** #307
**Branch:** `srepd/configurable-agent-cli`

## Goal

Replace the hardcoded `claude --print` command in `:agent` queries with a
configurable `agent_cli_command` config key. This fixes the bug where
`exec.CommandContext(ctx, "claude", "--print")` fails when `claude` is a
shell alias/function or not on PATH.

## Changes

- `pkg/config/config.go`: Add `agent_cli_command` to DefaultOptionalKeys
  (default: `"claude --print"`) and OptionalKeys
- `pkg/tui/model.go`: Add `agentCLICommand string` field, add parameter
  to both `InitialModel` and `InitialModelWithConfig`
- `pkg/tui/claude.go`: Replace hardcoded `exec.CommandContext(ctx, "claude", "--print")`
  with command parsed from `m.agentCLICommand` via `strings.Fields()`.
  Change `handleClaudePrompt` to accept `lookPath func(string) (string, error)`
  instead of `hasClaudeCode func() bool` for binary detection.
- `cmd/root.go`: Read `viper.GetString("agent_cli_command")`, pass to InitialModel
- All call sites and tests updated
