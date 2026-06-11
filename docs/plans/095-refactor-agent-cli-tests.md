# Plan: Refactor agent CLI tests to eliminate real subprocess calls (#318)

**Status:** Complete
**Issue:** #318
**PR:** TBD

## Problem

Several tests in `pkg/tui/claude_test.go` called real system binaries
(`echo`, `cat`, `sh`) via `exec.CommandContext`. Tests should be fully
self-contained and not depend on external processes.

## Solution

Introduced a `CommandExecutor` interface that abstracts command execution.
The default `execCommandExecutor` wraps `os/exec` for production use.
Tests inject a `mockCommandExecutor` that records arguments and returns
pre-configured results, eliminating all subprocess calls.

## Files changed

- `pkg/tui/claude.go` — added `CommandExecutor` interface, `execCommandExecutor`
  default impl; changed `agentQuery` to accept executor parameter
- `pkg/tui/claude_test.go` — added `mockCommandExecutor`; refactored 7 tests
  to use mock instead of real binaries
- `pkg/tui/model.go` — added `cmdExecutor` field; wired into both constructors
- `pkg/tui/model_test.go` — set `cmdExecutor` in `createTestModel()`

## Post-mortem / lessons learned

None — straightforward interface extraction.
