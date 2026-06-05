# Unified Color Scheme

## Problem
Modal views (incident viewer, cluster select, merge mode, log viewer)
used default bubbletea colors instead of srepd's custom palette. Tab
borders were incomplete. No config-driven color customization existed.

## Solution
- Centralized Theme/Styles system with DefaultTheme() and ThemeFromConfig()
- All views use model-level styles instead of module-level hardcoded vars
- Custom glamour StyleConfig for themed markdown rendering
- Tab borders extend to full width with right border
- Optional `colors:` config key for user customization
- Dynamic word wrap on terminal resize
