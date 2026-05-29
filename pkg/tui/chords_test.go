package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// chordPrefixKeyMsg returns a tea.KeyMsg that matches the default chord prefix (ctrl+x).
func chordPrefixKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlX}
}

// escKeyMsg returns a tea.KeyMsg for the Escape key.
func escKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEsc}
}

func TestChordPrefix_ActivatesChordMode(t *testing.T) {
	t.Run("pressing ctrl+x sets chordPending and shows status", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"

		// Press ctrl+x
		result, cmd := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.True(t, updatedModel.chordPending,
			"chordPending should be true after pressing chord prefix")
		assert.Contains(t, updatedModel.status, "ctrl+x",
			"status should show the chord prefix")
		assert.Contains(t, updatedModel.status, "...",
			"status should indicate waiting for second key")
		assert.Nil(t, cmd,
			"no command should be returned when entering chord mode")
	})
}

func TestChord_EscapeCancels(t *testing.T) {
	t.Run("pressing Escape while chord pending clears chordPending and status", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.chordPending = true
		m.status = "ctrl+x ..."

		// Press Escape
		result, cmd := m.Update(escKeyMsg())
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should be false after pressing Escape")
		assert.Equal(t, "", updatedModel.status,
			"status should be cleared after cancelling chord")
		assert.Nil(t, cmd,
			"no command should be returned when cancelling chord")
	})
}

func TestChord_ValidSecondKey(t *testing.T) {
	t.Run("pressing known second key executes the chord handler", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.chordPending = true

		// Press '?' which should be the chord help key
		questionKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
		result, _ := m.Update(questionKeyMsg)
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should be cleared after resolving chord")
		// The '?' chord shows help, so status should not contain "unknown chord"
		assert.NotContains(t, updatedModel.status, "unknown chord",
			"known chord should not show unknown chord message")
	})
}

func TestChord_UnknownSecondKey(t *testing.T) {
	t.Run("pressing unknown second key shows error status", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.chordPending = true

		// Press 'z' which is not a registered chord
		zKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}
		result, cmd := m.Update(zKeyMsg)
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should be cleared after unknown chord key")
		assert.Contains(t, updatedModel.status, "unknown chord",
			"status should indicate unknown chord")
		assert.Contains(t, updatedModel.status, "ctrl+x",
			"status should include the chord prefix")
		assert.Contains(t, updatedModel.status, "z",
			"status should include the unrecognized key")
		assert.Nil(t, cmd,
			"no command should be returned for unknown chord")
	})
}

func TestChord_DisabledDuringConfirmation(t *testing.T) {
	t.Run("chord prefix is ignored when pendingConfirmation is set", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.chordPrefix = "ctrl+x"

		// Set up a pending confirmation
		m.pendingConfirmation = &confirmActionState{
			prompt: "Silence P1234567? [y/n]",
			action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
		}

		// Press ctrl+x -- should be ignored because confirmation is active
		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should NOT be set during confirmation")
		assert.NotNil(t, updatedModel.pendingConfirmation,
			"pendingConfirmation should still be active")
	})

	t.Run("chord prefix is ignored during cluster selection", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.clusterSelectMode = true
		m.clusterSelectOptions = []string{"cluster-abc"}

		// Press ctrl+x -- should be ignored because cluster selection is active
		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should NOT be set during cluster selection")
		assert.True(t, updatedModel.clusterSelectMode,
			"cluster selection mode should still be active")
	})

	t.Run("chord prefix is ignored during input mode", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.input = newTextInput()
		m.input.Focus()

		// Press ctrl+x -- should be passed to input component, not chord handler
		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should NOT be set during input mode")
	})

	t.Run("chord prefix is ignored during error mode", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.err = errMsg{error: nil}

		// Press ctrl+x -- should be ignored because error mode is active
		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should NOT be set during error mode")
	})
}

func TestChord_ConfigurablePrefix(t *testing.T) {
	t.Run("different chord prefix works correctly", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+b"

		// Press ctrl+b (the configured prefix)
		ctrlBMsg := tea.KeyMsg{Type: tea.KeyCtrlB}
		result, cmd := m.Update(ctrlBMsg)
		updatedModel := result.(model)

		assert.True(t, updatedModel.chordPending,
			"chordPending should be true after pressing configured chord prefix")
		assert.Contains(t, updatedModel.status, "ctrl+b",
			"status should show the configured chord prefix")
		assert.Nil(t, cmd,
			"no command should be returned when entering chord mode")
	})

	t.Run("default prefix does not activate when different prefix configured", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+b"

		// Press ctrl+x (the default, but not the configured prefix)
		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordPending,
			"chordPending should NOT be set when pressing non-configured prefix key")
	})
}

func TestResolveChord(t *testing.T) {
	t.Run("returns action for known key", func(t *testing.T) {
		action := resolveChord("?")
		assert.NotNil(t, action, "resolveChord should return an action for '?'")
		assert.Equal(t, "?", action.Key, "action key should be '?'")
	})

	t.Run("returns nil for unknown key", func(t *testing.T) {
		action := resolveChord("z")
		assert.Nil(t, action, "resolveChord should return nil for unknown key 'z'")
	})
}

func TestChordHelpText(t *testing.T) {
	t.Run("returns non-empty help text", func(t *testing.T) {
		text := chordHelpText("ctrl+x")
		assert.NotEmpty(t, text, "chord help text should not be empty")
		assert.Contains(t, text, "ctrl+x", "help text should mention the prefix")
	})
}

func TestChord_WorksInTableMode(t *testing.T) {
	t.Run("chord prefix works when table is focused", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.chordPrefix = "ctrl+x"
		// table is focused by default in createTestModelWithSelectedIncident

		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.True(t, updatedModel.chordPending,
			"chordPending should be set in table mode")
	})
}

func TestChord_WorksInIncidentViewMode(t *testing.T) {
	t.Run("chord prefix works when viewing incident", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.chordPrefix = "ctrl+x"
		m.viewingIncident = true

		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.True(t, updatedModel.chordPending,
			"chordPending should be set in incident view mode")
	})
}
