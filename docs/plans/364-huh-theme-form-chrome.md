# 364: Shared huh theme + form chrome

Issue: #353/#324 (onboarding overhaul — config UI redesign phase)
Branch: `srepd/huh-theme-form-chrome`

## Problem

The three embedded huh forms (config wizard, team picker, bulk silence)
looked foreign next to the rest of SREPD:

1. Each built on `huh.ThemeCharm()` and re-tinted only a handful of
   `Focused.*` foregrounds — blurred (inactive) fields kept huh's stock
   purple/pink, clashing with SREPD's palette.
2. Forms rendered bare (`m.configForm.View()`), with no rounded-border pane
   while every sibling view (table, log viewer, cluster select, merge) sits
   in `TableContainer`.
3. The config form was full-bleed (`FormWidth = ws.Width`) — unreadable
   description lines on wide terminals.
4. The retint block was copy-pasted at three call sites.

## Approach

- **`SrepdHuhTheme(theme Theme) *huh.Theme`** (pkg/tui/theme.go): built from
  `huh.ThemeBase()`, styling **both Focused and Blurred** states plus Group
  titles and huh's inline help, entirely from the app `Theme` — so user
  `colors:` config overrides restyle the forms along with the rest of the
  UI. Blurred derives from Focused, dimmed to `Muted` (entered text stays
  `Text` for readability). Replaces all three `ThemeCharm()` call sites.
- **`FormContainer`** style in `BuildStyles`: same rounded-border pane
  language as `TableContainer`, `Padding(0, 2)`. views.go wraps all three
  form renders in it.
- **Layout**: `FormWidth = min(width - container frame, layoutMaxFormWidth
  (90))`; `FormHeight`/`TeamSelectFormHeight` subtract the container's
  vertical frame. Resize path unchanged (already flows through
  `m.layout.FormWidth/FormHeight`).

## Tests (TDD — written first)

`pkg/tui/huh_theme_test.go`:
- Focused styles use the app palette (title=Highlight, description=Muted,
  border=Border, errors=Warning, options=Text/Highlight)
- Blurred styles match the app palette (muted titles, readable text) — the
  stock-purple regression case
- `colors:` overrides flow through (custom highlight/muted assert)
- Source-scan guard: `huh.ThemeCharm` may not reappear in pkg/tui
- `FormContainer` is a rounded-border, padded, theme-colored pane
- `FormWidth` fits the container on narrow terminals and caps at 90 on wide

Updated existing layout tests (`layout_test.go`) to the new intended values
(frame subtraction, width cap) — they previously pinned the full-bleed
behavior this change removes.
