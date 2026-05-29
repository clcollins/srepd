# 055: README Cleanup

## Problem

The README is 470 lines with too many terminal configuration examples,
verbose architecture descriptions, and upcoming-feature references.
It should be ~100-150 lines covering the essentials.

## Approach

Rewrite the README to match the target structure from issue #206:

1. Header image and badges (existing)
2. One-line description
3. Features (5-6 bullets)
4. Installation (`make install` or `go install`)
5. Configuration (required/optional keys table, 1-2 config examples max)
6. Terminal support (bullet list without separators/flags)
7. Key bindings (compact table verified against keymap.go)
8. Environment variables (list verified against buildPagerDutyEnvVars())
9. Development (make targets table)

## Verification

- Every key binding in README exists in `pkg/tui/keymap.go`
- Every config key exists in `cmd/config.go`
- Every terminal in the list exists in `pkg/launcher/profiles.go`
- Every env var matches `buildPagerDutyEnvVars()` in `pkg/tui/commands.go`
- `toolbox_mode` config key is documented
- `PAGERDUTY_CLAUDE_AVAILABLE` conditional env var is documented
- No separators/flags shown in terminal list (profiles handle this)
- No "upcoming" feature references

## Testing

- `go test ./... -count=1` passes (README-only change, no code modified)
- Manual line count within 100-150 range

## Risks

None -- documentation-only change with no code impact.
