# 395: Fix overlapping focus modes — ctrl+h from incident view

Branch: `srepd/fix-docs-incident-focus-overlap`

## What

Property-based testing (PR #394) found that pressing ctrl+h (open docs) while
viewing an incident leaves both `viewingIncident` and `viewingDocs` true
simultaneously, violating the mutual-exclusivity invariant on focus modes.

## Fix

Add `docsReturnToIncident` field to model. When opening docs from incident view,
clear `viewingIncident` and set the return flag. When closing docs, restore
`viewingIncident` if the flag is set. This keeps focus modes mutually exclusive
while preserving the UX of returning to the incident after closing docs.

Also removed the `knownOverlap` exception from statemachine_test.go — the
property-based tests now enforce mutual exclusivity without exceptions.
