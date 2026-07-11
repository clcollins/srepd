# 373: In-TUI guided tour (:tour)

Issue: #324 item 3; final phase of the onboarding overhaul (#353, #324)
Branch: `srepd/guided-tour`

## Problem

srepd has dozens of features (8 detail tabs, chord commands, flags, :agent,
bulk silence, the watcher) but a new user has no way to learn them beyond
the README or stumbling onto `h`. #324 asks for an in-app tour that teaches
by showing.

## Approach

A tour mode following the established mode pattern (model flag →
`keyMsgHandler` dispatch case → `views.go` case):

- **Content**: 9 steps (`tourSteps()`) — incident table, navigation, viewer
  tabs, key actions (a / ctrl+s / ctrl+e / n / l), command mode
  (:agent/:watcher), chords (ctrl+x ?), flags, the watcher, and a closer
  pointing at `h` and `:tour`.
- **Rendering**: the live incident table stays on screen for context with
  the step panel beneath it in the app's `FormContainer` pane language:
  bold title, muted "n/9" progress, body, and the key hints. Keys are
  DESCRIBED, never executed — safe with live incidents; works fine with an
  empty table or `--dev` fixtures.
- **Controls**: any key advances, shift+tab/left goes back, esc/q exits;
  advancing past the last step completes ("tour complete").
- **Entry points**: `:tour` in command mode (registered beside
  :agent/:watcher/:flag), plus a one-time suggestion appended to the
  post-setup status line ("config saved — new here? type :tour…") when
  `tour_seen` is unset. Never auto-forced.
- **`tour_seen`**: persisted via `UpsertScalarInConfig` (comment-preserving)
  the first time the tour starts, so the suggestion shows at most once.

## Tests (TDD — written first)

`pkg/tui/tour_test.go`: step content covers the feature set (≥8 steps;
acknowledge/silence/:agent/chord/flag/watcher/tab all mentioned);
`isTourCommand`; start sets mode + persists; advance/back/floor semantics;
esc exits with status; completion past the last step; panel render contains
title, progress, and exit hint. Existing config-mode tests pin the combined
post-setup status line.
