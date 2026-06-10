# Color Palettes

SREPD supports custom color themes via the `colors` key in `~/.config/srepd/srepd.yaml`. All keys are optional — unspecified keys fall back to defaults. Values must be valid hex colors (`#rgb` or `#rrggbb`, case-insensitive). Invalid values are silently ignored.

## Color Keys

| Key | Controls |
|-----|----------|
| `text` | Normal text, table rows, document body |
| `border` | Borders, tab outlines, separators, horizontal rules |
| `highlight` | Table headers, selected row text, active tab text, headings |
| `selected` | Selected row background |
| `warning` | Warning and confirmation prompt background |
| `error` | Error modal background |
| `muted` | De-emphasized text (version string, incident IDs) |
| `tab` | Tab accent color |

## Default

The built-in palette uses blue-gray tones from the [Coolors "Quantum Harmony"](https://coolors.co/palette/0d1b2a-1b263b-415a77-778da9-e0e1dd) palette. Designed for dark terminal backgrounds.

```yaml
colors:
  text: "#778da9"
  border: "#415a77"
  highlight: "#ffffff"
  selected: "#415a77"
  warning: "#a4133c"
  error: "#0d1b2a"
  muted: "#5C5C5C"
  tab: "#7D56F4"
```

## Nord

Arctic, blue-tinted palette with muted accents. Works well on dark terminal backgrounds. From the [Nord](https://www.nordtheme.com) theme (MIT license).

```yaml
colors:
  text: "#D8DEE9"
  border: "#4C566A"
  highlight: "#88C0D0"
  selected: "#434C5E"
  warning: "#EBCB8B"
  error: "#BF616A"
  muted: "#4C566A"
  tab: "#5E81AC"
```

## Catppuccin Latte

Pastel palette designed for light terminal backgrounds. From the [Catppuccin](https://catppuccin.com) theme (MIT license).

```yaml
colors:
  text: "#4c4f69"
  border: "#bcc0cc"
  highlight: "#7287fd"
  selected: "#ccd0da"
  warning: "#df8e1d"
  error: "#d20f39"
  muted: "#acb0be"
  tab: "#8839ef"
```
