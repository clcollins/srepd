# Merge incident into another incident

## Context

SREs need to merge duplicate PagerDuty incidents from the TUI.
ctrl+m opens a scrollable incident selection view, user picks
a target, and the source is merged into it via PD API.

## Changes

- Add MergeIncidentsWithContext to PD client interface, mock, dev, ratelimit
- Add merge mode with scrollable table (same format as main view)
- ctrl+m triggers merge, t toggles team/individual, Enter confirms
- Confirmation prompt before dispatching merge
- Flash notification and INFO log on completion

## Verification

- make test-all passes
- Dev mode: ctrl+m shows merge target list, Enter/Escape work
- Real PD: merge completes and incident disappears
