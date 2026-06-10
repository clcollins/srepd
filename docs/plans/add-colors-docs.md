# Plan: Add color palette documentation

## Context

SREPD's color system is configurable but the README only shows the default values without explaining what each key controls or offering alternative palettes. Users who want to customize colors need to read the source code.

## Changes

- Add `docs/colors.md` documenting all 8 color keys, the default palette, and two pre-built alternatives (Nord for dark terminals, Catppuccin Latte for light terminals)
- Add cross-reference from README Colors section to the new doc
