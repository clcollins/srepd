# 061: Medium Priority Test Coverage

## Problem

Issue #205 identifies medium-priority functions with insufficient test
coverage. After PR #230 addressed high-priority functions, these remain:

| Function               | File             | Current | Target |
|------------------------|------------------|---------|--------|
| `InitialModel`         | model.go:117     | 0%      | >80%   |
| `InitialModelWithConfig` | model.go:186   | 0%      | >80%   |
| `renderIncident`       | commands.go:235  | 0%      | >80%   |
| `renderIncidentMarkdown` | views.go:657   | 0%      | >80%   |
| `switchInputFocusMode` | msgHandlers.go:531 | 89.5% | 100%   |
| `switchErrorFocusMode` | msgHandlers.go:789 | 80%   | 100%   |
| `doIfIncidentSelected` | commands.go:990  | 62.5%  | 100%   |

Functions already at 100%: `runScheduledJobs`, `removeCommentsFromBytes`.

## Solution

Write unit tests for each uncovered function using existing test patterns:
table-driven tests, testify assertions, mock PD client.

### InitialModel / InitialModelWithConfig

- Valid config returns a non-nil model with expected defaults
- Debug flag propagates
- Editor and launcher fields propagate
- Nil config in InitialModelWithConfig sets m.err
- Error from pd.NewConfig sets m.err
- Default field values: autoRefresh=true, showLowUrgency=true, chordPrefix="ctrl+x"

### renderIncident / renderIncidentMarkdown

- renderIncident with valid model produces renderedIncidentMsg
- renderIncident with nil selectedIncident produces errMsg
- renderIncidentMarkdown with nil renderer returns plain content
- renderIncidentMarkdown with valid renderer returns rendered content

### switchInputFocusMode gaps

- Quit key (ctrl+c/ctrl+q) in input mode returns tea.Quit (line 535-537)
- Non-KeyMsg returns unchanged model (line 570)

### switchErrorFocusMode gaps

- Escape key clears m.err (line 794-795)

### doIfIncidentSelected gaps

- Selected row exists: returns tea.Sequence with getIncidentMsg (lines 996-998)

## Files to modify

- `pkg/tui/model_test.go` -- InitialModel and InitialModelWithConfig tests
- `pkg/tui/commands_test.go` -- renderIncident, doIfIncidentSelected tests
- `pkg/tui/views_test.go` -- renderIncidentMarkdown tests
- `pkg/tui/msgHandlers_test.go` -- switchInputFocusMode, switchErrorFocusMode tests

## Verification

All 7 make checks must pass before commit.
