# 046: Alert Normalization Engine

## Problem

PagerDuty incidents arrive from 6+ distinct alert pipelines, each with its own
title format, body structure, and field locations. The existing TUI code uses
`getDetailFieldFromAlert()` for raw field extraction, which only works for
types that have structured top-level `details` fields (osd_hive, rhobs_hcp).
Types like appsre bury SOP links and severity inside the `firing` text blob,
and Dead Man's Snitch puts runbook URLs in a `notes` text field.

This means the TUI cannot reliably extract SOP links, severity, cluster names,
or dashboard URLs across all alert types.

## Solution

Introduce a `pkg/alert/` package that normalizes all alert types into a single
`NormalizedAlert` struct. The package:

1. **Identifies** alert type from the service name pattern (`IdentifyType`)
2. **Strips** SRE-added bracket prefixes from titles (`StripBracketPrefixes`)
3. **Parses** the `firing` text field in both Alertmanager and RHOBS formats (`ParseFiring`)
4. **Dispatches** to per-type parsers that extract fields from their known locations
5. **Normalizes** severity to lowercase and extracts region from service names

### Alert Types Supported

| Type | Service Pattern | Parsing Complexity |
|------|----------------|-------------------|
| `osd_hive` | `*-hive-cluster` | Low (structured details) |
| `appsre` | `app-sre-alertmanager*` | High (firing text parsing) |
| `rhobs_hcp` | `rhobs-hcp-*` | Low (best-structured) |
| `rhobs_infra` | `rhobs-infra-*` | Low (structured details) |
| `deadmanssnitch` | `*-deadmanssnitch` | Medium (notes text parsing) |
| `cee_escalation` | `cee-*` | Low (title-only, no alerts) |
| `cad` | `CAD *` | Minimal (test alerts) |
| `unknown` | fallback | Extract what possible, never crash |

### TUI Integration

- `summarizeAlerts()` calls `alert.NormalizeAlert()` for each alert, adding
  severity, tags, and alert type to the `alertSummary` struct
- `getSOPLink()` uses normalized SOPLink first (handles appsre firing text
  and DMS notes extraction), with raw field fallback
- Incident template shows severity and alert type inline

## Files Changed

### New Package: `pkg/alert/`

| File | Purpose |
|------|---------|
| `normalize.go` | `NormalizedAlert` struct, `IdentifyType()`, `NormalizeAlert()` |
| `title.go` | `StripBracketPrefixes()` for SRE-added title tags |
| `firing.go` | `ParseFiring()` for Alertmanager and RHOBS text formats |
| `types.go` | Per-type parsers (parseOSDHive, parseAppSRE, etc.) |
| `normalize_test.go` | Tests for IdentifyType and NormalizeAlert per type |
| `title_test.go` | Tests for bracket prefix stripping |
| `firing_test.go` | Tests for firing text format parsing |

### Modified TUI Files

| File | Change |
|------|--------|
| `pkg/tui/views.go` | `alertSummary` gains Severity/Tags/AlertType; uses `alert.NormalizeAlert()` |
| `pkg/tui/commands.go` | `getSOPLink()` uses normalized extraction with raw fallback |

## Design Decisions

1. **Type identification by service name, not title.** Service names are
   system-generated and stable; titles are mutated by SREs.

2. **Parse raw title before stripping brackets for format-specific types.**
   RHOBS and AppSRE titles contain format-specific brackets (`[HCP] [RHOBS]`,
   `[FIRING:N]`) that must be preserved for regex matching. SRE-added brackets
   may precede these, so the parser tries the raw title first, then falls back
   to the stripped version.

3. **Backward-compatible integration.** The TUI falls back to raw
   `getDetailFieldFromAlert()` when normalization yields empty fields, ensuring
   no regression for existing alert types.

4. **Unknown handler never crashes.** New alert pipelines will appear; the
   unknown parser extracts common fields and leaves type-specific fields empty.

## Testing

- 32 unit tests covering all alert types, edge cases, and error paths
- TDD approach: tests written before implementation
- All existing TUI tests continue to pass
