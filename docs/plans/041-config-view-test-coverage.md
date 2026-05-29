# 041: Config Validation and View/Render Test Coverage

## Status: Complete

## Objective

Expand test coverage for config validation logic in `cmd/config.go` and
view/render functions in `pkg/tui/views.go` and `pkg/tui/commands.go`.

## Part 1: Config Validation Tests (`cmd/config_test.go`)

### Tests Added

1. **TestValidateConfig_InvalidEscalationPoliciesType** - Verifies that
   `validateConfig` returns an error when `service_escalation_policies` is set
   to a string instead of a map. Exercises the `.(map[string]interface{})`
   type assertion guard at line 171 of `config.go`.

2. **TestValidateConfig_DeprecatedKeyDetected** - Verifies that deprecated keys
   (e.g., `"shell"`) do not cause validation errors. The `deprecation.Deprecated`
   function detects these and the code logs an informational message but continues
   validation successfully.

3. **TestValidateConfig_CaseSensitiveEscalationKeys** - Verifies that the
   required inner keys `"default"` and `"silent_default"` must be present in
   the escalation policies map. The validation code uses `strings.ToLower()`
   for lookup, and viper normalizes keys to lowercase, so both required keys
   must resolve to their lowercase forms.

## Part 2: View/Render Function Tests

### `stateShorthand()` (`pkg/tui/commands_test.go`)

4. **TestStateShorthand_Triggered** - Incident with no acknowledgements returns
   the dot character.

5. **TestStateShorthand_AckedByUser** - Incident acknowledged by the current
   user returns `"A"` (uppercase).

6. **TestStateShorthand_AckedByOther** - Incident acknowledged by a different
   user returns `"a"` (lowercase).

### `renderFooter()` (`pkg/tui/views_test.go`)

7. **TestRenderFooter_ContainsRefreshStatus** - Verifies that `renderFooter()`
   output contains the `refreshArea()` text ("Watching for updates...").

### `renderBottomStatus()` (`pkg/tui/views_test.go`)

8. **TestRenderBottomStatus_ShowsIncidentID** - With a selected incident, the
   bottom status line includes the incident ID.

9. **TestRenderBottomStatus_ShowsGitSHA** - The bottom status line includes the
   `GitSHA` variable value.

### `summarizeAlerts()` edge cases (`pkg/tui/views_test.go`)

10. **TestSummarizeAlerts_EmptyAlerts** - Empty alert slice returns nil.

11. **TestSummarizeAlerts_AlertWithNilBody** - Alert with nil Body does not
    panic and returns a summary with empty name, link, cluster, and nil details.

## Files Changed

- `cmd/config_test.go` - 3 new test functions
- `pkg/tui/commands_test.go` - 3 new test functions
- `pkg/tui/views_test.go` - 5 new test functions
