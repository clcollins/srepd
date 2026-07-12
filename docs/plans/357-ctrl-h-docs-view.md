# 357: Ctrl+H Docs View with Auto-Generated Quickstart

## Problem

SREPD has no in-app documentation. Users must leave the TUI to read
the README or docs. There's also no quick reference for keybindings,
chord commands, or input commands.

## Approach

Add a docs viewer mode triggered by `ctrl+h` that embeds `README.md`
and all `docs/*.md` files at build time using `go:embed`. The first
tab is an auto-generated `quickstart.md` containing tables of all
keybindings, chord commands, and input commands. README is the second
tab, remaining docs sorted alphabetically by H1 title.

### Key decisions

- **go:embed at root level**: The embed directive lives in `main.go`
  since `go:embed` can only reference files in or below the package
  directory. Content is injected into `pkg/docs` via
  `SetEmbeddedContent()` before the TUI starts.
- **Auto-generated quickstart**: A standalone generator at
  `cmd/gen-quickstart/main.go` reads keybinding data from exported
  functions in `pkg/tui/quickstart_data.go` and writes markdown.
- **CI enforcement**: `quickstart-check` (triggers when keybinding
  files change) and `quickstart-verify` (diffs generated output
  against committed file) catch staleness. `quickstart-verify` is
  part of `test-all`.
- **Tab paging**: Tabs are paged in groups of 8. When all tabs fit on
  one page, no indicator is shown. Paging indicator renders inside
  the gap border when needed.
- **Toggle behavior**: `ctrl+h` toggles the docs view — pressing it
  again returns to the previous view, same as `esc`.

## Testing

All new code is TDD with pure functions and full mocking:

- `pkg/docs/docs_test.go` — ExtractTitle, TruncateTitle, BuildDocList
  using `fstest.MapFS`
- `pkg/tui/quickstart_data_test.go` — KeyBindingEntries, ChordEntries,
  InputCommandEntries, GenerateQuickstartMarkdown
- `pkg/tui/docs_test.go` — paging, tab labels, navigation, clearing
