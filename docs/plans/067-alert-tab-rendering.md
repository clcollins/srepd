# Improve alert tab rendering

## Context

Alert tab displayed raw text for alert names and SOP links instead
of markdown links. Dev fixture PDEV_INC_012 was assigned to the
wrong user, hiding it from the default view.

## Changes

- Alert ID bold at top, alert name as [Name](url) link
- SOP rendered as [filename.md](url) with filename extracted from path
- Removed redundant Alert: bullet line
- Reassigned PDEV_INC_012 to Dev User

## Verification

- make test-all passes
- Dev mode and production render alerts with clickable links
