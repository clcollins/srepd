package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// #324 item 3: an in-TUI guided tour that teaches by showing. Steps cover
// the incident table, navigation, detail tabs, key actions, command mode,
// chords, flags, and the watcher.
func TestTourSteps_CoverTheFeatureSet(t *testing.T) {
	steps := tourSteps()

	assert.GreaterOrEqual(t, len(steps), 8, "#324 asks for ~8-10 steps")

	var all strings.Builder
	for _, s := range steps {
		assert.NotEmpty(t, s.Title)
		assert.NotEmpty(t, s.Body)
		all.WriteString(s.Title + " " + s.Body + " ")
	}
	content := all.String()
	for _, want := range []string{"acknowledge", "silence", ":agent", "chord", "flag", "watcher", "tab"} {
		assert.Contains(t, strings.ToLower(content), want, "tour must cover %q", want)
	}
}

func TestIsTourCommand(t *testing.T) {
	assert.True(t, isTourCommand(":tour"))
	assert.True(t, isTourCommand("  :tour  "))
	assert.False(t, isTourCommand(":tourism"))
	assert.False(t, isTourCommand("tour"))
}

func TestStartTour(t *testing.T) {
	m := createConfigTestModel()

	m2, cmd := m.startTour()

	assert.True(t, m2.tourMode)
	assert.Equal(t, 0, m2.tourStep)
	assert.NotNil(t, cmd, "starting the tour persists tour_seen")
}

func TestSwitchTourFocusMode_AdvanceAndBack(t *testing.T) {
	m := createConfigTestModel()
	m.tourMode = true
	m.tourStep = 0

	result, _ := switchTourFocusMode(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := result.(model)
	assert.Equal(t, 1, updated.tourStep, "any ordinary key advances")

	result, _ = switchTourFocusMode(updated, tea.KeyMsg{Type: tea.KeyShiftTab})
	updated = result.(model)
	assert.Equal(t, 0, updated.tourStep, "shift+tab goes back")

	result, _ = switchTourFocusMode(updated, tea.KeyMsg{Type: tea.KeyShiftTab})
	updated = result.(model)
	assert.Equal(t, 0, updated.tourStep, "back at the first step stays put")
}

func TestSwitchTourFocusMode_EscExits(t *testing.T) {
	m := createConfigTestModel()
	m.tourMode = true
	m.tourStep = 3

	result, _ := switchTourFocusMode(m, tea.KeyMsg{Type: tea.KeyEscape})
	updated := result.(model)

	assert.False(t, updated.tourMode)
	assert.Contains(t, updated.status, "tour")
}

func TestSwitchTourFocusMode_CompletesPastLastStep(t *testing.T) {
	m := createConfigTestModel()
	m.tourMode = true
	m.tourStep = len(tourSteps()) - 1

	result, _ := switchTourFocusMode(m, tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(model)

	assert.False(t, updated.tourMode, "advancing past the last step ends the tour")
	assert.Contains(t, updated.status, "complete")
}

func TestRenderTourPanel(t *testing.T) {
	m := createConfigTestModel()
	m.tourMode = true
	m.tourStep = 0

	out := m.renderTourPanel()

	steps := tourSteps()
	assert.Contains(t, out, steps[0].Title)
	assert.Contains(t, out, fmt.Sprintf("1/%d", len(steps)), "progress indicator must be shown")
	assert.Contains(t, out, "esc", "must show how to exit")
}
