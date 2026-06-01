# Incident Viewer Tab Refactor

## Context

The incident viewer renders Details, Alerts, and Notes as stacked
sections in a single markdown blob. All sections are always visible
and scrolling is confusing. This refactors the viewer into three
proper lipgloss-styled tabs following the bubbletea tabs example.

## Design

Three tabs: Details, Alerts, Notes. Only active tab's content visible.
Tab/Shift+Tab cycles tabs. Up/Down scrolls within the active tab.
Alerts and Notes render as full scrollable lists (no item cycling).

## Changes

- model.go: activeSection → activeTab, remove activeAlertIdx/activeNoteIdx
- views.go: Add renderTabContent() per-tab, renderTabBar() with lipgloss
- msgHandlers.go: Tab switches tabs, Up/Down scrolls viewport
- keymap.go: Update bindings and help text
- commands.go: renderIncident() calls renderTabContent()
- Remove dead section-based code

## Verification

- make test-all passes
- Dev mode shows 3 styled tabs with correct counts
- Tab/Shift+Tab switches content, Up/Down scrolls
- Loading states and edge cases work correctly
