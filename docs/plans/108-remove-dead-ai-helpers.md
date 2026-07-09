# Plan 108: Remove dead AI helper functions

**Branch:** `srepd/remove-dead-ai-helpers`

## Problem

`deadcode ./...` flagged three exported functions in `pkg/ai` with **zero production
callers**:

- `BuildSystemPrompt` (`pkg/ai/prompts.go`) — builds a hardcoded SRE system prompt.
  **Superseded**: the `:agent`/`:watcher` paths now use configurable system prompts
  (`agentSystemPrompt` / `watcherSystemPrompt` from config), so this hardcoded builder
  is obsolete.
- `extractClusterID` (`pkg/ai/prompts.go`) — only called by `BuildSystemPrompt`; dies
  with it.
- `KnownProviders` (`pkg/ai/factory.go`) — returns registered provider names; never
  called.

Scope decided with the maintainer: remove the three `ai` helpers; **keep**
`WriteConfigTeams`/`WriteConfigKey`/`WriteConfigMap` (also dead but a tested, hardened,
viable programmatic config-write API for a future `config set` subcommand).

## Solution

- Delete `pkg/ai/prompts.go` and `pkg/ai/prompts_test.go` (the file contained only the
  two dead functions and their tests).
- Remove `KnownProviders` from `pkg/ai/factory.go` and `TestKnownProviders` from
  `pkg/ai/factory_test.go`.

## Files Modified

- `pkg/ai/prompts.go` — deleted.
- `pkg/ai/prompts_test.go` — deleted.
- `pkg/ai/factory.go` — removed `KnownProviders`.
- `pkg/ai/factory_test.go` — removed `TestKnownProviders`.

## Verification

- `deadcode ./...` no longer reports the three functions.
- `go build ./...`, `gofmt -s`, `go vet`, `go test ./...`, `golangci-lint` all pass
  (go1.26.5).

## Lessons Learned

- When a configurable/config-driven mechanism replaces a hardcoded helper
  (`BuildSystemPrompt` → config prompts), the old helper becomes vestigial — prune it
  so `deadcode` stays quiet and the API surface reflects reality.
- Not all "dead" exported code should be removed: the config-writer trio was kept
  deliberately as a viable, tested API. Dead-code removal is a judgment call, not a
  mechanical sweep.
