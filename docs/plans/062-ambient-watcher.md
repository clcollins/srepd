# Plan: Ambient LLM Incident Watcher

**Issue:** #305
**Depends on:** #195 (merged as PR #314), #307 (merged as PR #317)
**Branch:** `srepd/ambient-watcher`

## Goal

Add an ambient watcher system that monitors the incident queue for
cross-incident patterns and provides AI-assisted analysis through a
collapsible pane below the incident table.

## What was built

### Phase 1: Watcher pane layout
- Collapsible viewport below table, toggle via `w` key
- Dynamic table/watcher height split (2/3 table, 1/3 watcher, min 10/5 rows)
- Rounded border matching table style, mouse wheel scrolling
- Provider status merged into footer line

### Phase 2: Claude CLI responses refactored into watcher pane
- `:agent` responses redirect from incident viewer to watcher pane
- Ring buffer for observation history (50 entries)
- Configurable emoji/text markers (emoji config toggle)
- Per-line marker prefixing, word wrap at viewport width
- Persistent input during agent queries (later reverted to vim-style blur)

### Phase 3: Configurable CLI agent command
- Merged separately as PR #317
- `agent_cli_command` config key (default: `claude --print`)

### Phase 4: Heuristic pattern detection
- Service storm (3+ on same service), cluster storm (2+ on same cluster)
- Urgency shift (3+ high urgency)
- Deduplication with 5-minute cooldown
- Triggered on incident list updates and flag changes

### Phase 5: LLM synthesis
- Heuristic observations sent to LLM for natural-language synthesis
- Falls back to raw text when provider unavailable
- `:watcher` command for direct LLM queries with full incident context
- Typewriter word-by-word response display
- Countdown timer in footer during queries

### Additional improvements
- `:` prefix for commands (vim-style), `/` reserved for future search
- `f` and `i` keys removed, `:/` enter command input
- Flag markers appear immediately on add/remove/load
- Flags load triggers OCM enrichment for unenriched incidents
- Flag cache rebuilt on cluster enrichment arrival
- Configurable system prompts for agent and watcher
- Shared incident context between agent and watcher
- Flash notifications for all errors (timeouts, not found, offline)
- Mouse event and OCM log noise suppressed from journal
