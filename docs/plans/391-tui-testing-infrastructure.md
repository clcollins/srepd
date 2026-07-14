# 391: TUI Testing Infrastructure — tui-mcp, Unit Tests, Golden Files, Property-Based Testing

Branch: `srepd/tui-testing-infrastructure`

## Context

Research across five agents surveyed the TUI testing ecosystem (July 2026). srepd
has strong model-level unit tests (715+ test functions) but four gaps:

1. **No agent-driven visual validation** — unlike web UIs where firefox-devtools
   provides launch/screenshot/interact, no equivalent exists for srepd's TUI today.
   `tui-mcp` is an MCP server that fills this gap exactly.

2. **47 untested Update handler branches** — the main `switch msg.(type)` in
   `tui.go` has ~70 message types; 47 handler branches have zero direct test
   coverage.

3. **No golden file / snapshot tests** — View() rendering is only tested for 5
   of 12 focus modes, and only via string-contains assertions. No snapshot
   comparison catches visual regressions (the class of bugs behind the tab bar
   scroll issue, modal rendering freeze, etc.).

4. **No property-based / fuzz testing** — unit tests only cover hand-written
   scenarios. The TUI is a state machine with 9 focus mode booleans and ~25 key
   bindings. Random sequences of inputs can reach states no human would think to
   test. Property-based testing generates thousands of random input sequences and
   checks invariants after every step.

## Approach

### Part A: Install tui-mcp for agent-driven validation

Add `tui-mcp` to `.claude/settings.json` and document in AGENTS.md so Claude
can launch srepd, send keystrokes, take PNG screenshots, and visually verify
the TUI during development.

### Part B: Fill unit test gaps for untested Update handlers

Add tests for 47 untested handler branches in `tui.go`, grouped by functional
area. TDD workflow: write failing test, implement if needed, verify green.

### Part C: Golden file snapshot tests for View() rendering

Add `charmbracelet/x/exp/golden` and create `pkg/tui/golden_test.go` with
golden file comparison tests for all 12 focus modes. Force ASCII color profile
for CI portability.

### Part D: Property-based state machine testing with rapid

Add `pgregory.net/rapid` and create `pkg/tui/statemachine_test.go` that fuzzes
the TUI state machine with random key sequences, checking 6 invariants:
View() never panics, never returns empty, at most one focus mode active,
selectedIncident always in incidentList, activeTab in bounds, window size
consistent.

## Execution Order

1. Part A (tui-mcp) — enables visual validation for subsequent work
2. Part C (golden files) — establishes snapshot infrastructure
3. Part B (unit tests) — fill handler gaps
4. Part D (property-based testing) — fuzz to find edge cases

## Verification

- `make test-all` passes
- `go test ./pkg/tui/ -run TestGolden -update` creates correct golden files
- `go test ./pkg/tui/ -run TestGolden` passes without `-update`
- `go test ./pkg/tui/ -run TestStateMachine -count=1 -v` passes
- `make test-fuzz` works
- tui-mcp can launch srepd and take screenshots
