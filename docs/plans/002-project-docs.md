# Add project documentation for Claude Code agents and conventions

> Retroactive plan document for PR #142, created after merge.

## Context

SREPD had no project-level documentation for Claude Code agents,
contributor conventions, or agent coordination. Every subsequent PR
and agent needed these files to understand build commands, test
patterns, architecture, and code style requirements.

## Plan

1. Create `CONVENTIONS.md` modeled on the unifi-bootstrapper style,
   covering Go style, Makefile targets, CI testing, linting, platform,
   and version control conventions
2. Create `AGENTS.md` as single source of truth for Claude Code agents:
   build commands, architecture overview, test patterns, key invariants,
   PR workflow, and key files reference
3. Create `CLAUDE.md` that imports `AGENTS.md` via `@AGENTS.md`
4. Override global gitignore with `!CLAUDE.md` in project `.gitignore`

## Files Modified

- `CONVENTIONS.md` — new, project conventions
- `AGENTS.md` — new, agent instructions
- `CLAUDE.md` — new, imports AGENTS.md
- `.gitignore` — added `!CLAUDE.md` override

## Verification

- All three files render correctly on GitHub
- CLAUDE.md import resolves for Claude Code sessions
