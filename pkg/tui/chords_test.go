package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
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

func TestChordShowHelp_SetsChordHelpActive(t *testing.T) {
	t.Run("pressing ctrl+x ? sets chordHelpActive and expands help", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.help = newHelp()
		m.chordPending = true

		// Press '?' to trigger chord help
		questionKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
		result, _ := m.Update(questionKeyMsg)
		updatedModel := result.(model)

		assert.True(t, updatedModel.chordHelpActive,
			"chordHelpActive should be true after chord help is triggered")
		assert.True(t, updatedModel.help.ShowAll,
			"help.ShowAll should be true to display full chord help")
	})
}

func TestChordShowHelp_DoesNotSetStatus(t *testing.T) {
	t.Run("chord help does not use the status bar", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.help = newHelp()
		m.chordPending = true
		m.status = ""

		// Press '?' to trigger chord help
		questionKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
		result, _ := m.Update(questionKeyMsg)
		updatedModel := result.(model)

		assert.Empty(t, updatedModel.status,
			"status bar should remain empty when chord help is shown")
	})
}

func TestChordHelp_ClearedOnKeypress(t *testing.T) {
	t.Run("any keypress clears chordHelpActive", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.help = newHelp()
		m.chordHelpActive = true
		m.help.ShowAll = true

		// Press any key (e.g., 'j' for down)
		jKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
		result, _ := m.Update(jKeyMsg)
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordHelpActive,
			"chordHelpActive should be cleared after any keypress")
	})

	t.Run("pressing chord prefix while chord help is active clears it and enters chord mode", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.help = newHelp()
		m.chordHelpActive = true
		m.help.ShowAll = true

		// Press ctrl+x again
		result, _ := m.Update(chordPrefixKeyMsg())
		updatedModel := result.(model)

		assert.False(t, updatedModel.chordHelpActive,
			"chordHelpActive should be cleared when entering new chord mode")
		assert.True(t, updatedModel.chordPending,
			"chordPending should be set after pressing chord prefix")
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

func TestChordHelp_ViewRendersChordKeymap(t *testing.T) {
	t.Run("View renders chord help bindings when chordHelpActive", func(t *testing.T) {
		m := createTestModel()
		m.chordPrefix = "ctrl+x"
		m.help = newHelp()
		m.help.ShowAll = true
		m.chordHelpActive = true

		// Set window size so View can render
		windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

		view := m.View()

		// The chord help should contain chord-specific entries
		assert.Contains(t, view, "show chord help",
			"View should render chord help descriptions when chordHelpActive is true")
		assert.Contains(t, view, "view debug log",
			"View should render chord debug log description when chordHelpActive is true")
	})
}

func TestChordKeymap_ImplementsHelpKeyMap(t *testing.T) {
	t.Run("chordKeymap ShortHelp returns dismissal hint", func(t *testing.T) {
		km := chordKeymap{prefix: "ctrl+x"}
		short := km.ShortHelp()
		assert.NotEmpty(t, short, "ShortHelp should return at least one binding")
	})

	t.Run("chordKeymap FullHelp returns chord bindings", func(t *testing.T) {
		km := chordKeymap{prefix: "ctrl+x"}
		full := km.FullHelp()
		assert.NotEmpty(t, full, "FullHelp should return at least one column")
		totalBindings := 0
		for _, col := range full {
			totalBindings += len(col)
		}
		visibleCount := 0
		for _, entry := range chordRegistry {
			if !entry.Hidden {
				visibleCount++
			}
		}
		assert.GreaterOrEqual(t, totalBindings, visibleCount,
			"FullHelp should have at least as many bindings as visible chord registry entries")
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

func TestChordBulkSilence_Registered(t *testing.T) {
	t.Run("bulk silence chord is registered", func(t *testing.T) {
		action := resolveChord("s")
		assert.NotNil(t, action, "resolveChord should return an action for 's'")
		assert.Equal(t, "s", action.Key)
		assert.Equal(t, "bulk silence", action.Description)
	})
}

func TestChordBulkSilence_NoIncidents(t *testing.T) {
	t.Run("bulk silence with no incidents shows status", func(t *testing.T) {
		m := createTestModel()
		m.incidentList = nil

		result, cmd := chordBulkSilence(m)
		updated := result.(model)

		assert.Contains(t, updated.status, "no incidents")
		assert.Nil(t, cmd)
	})
}

func TestChordBulkSilence_WithIncidents(t *testing.T) {
	t.Run("bulk silence with incidents returns enterBulkSilenceMsg", func(t *testing.T) {
		m := createTestModel()
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Test"},
		}

		_, cmd := chordBulkSilence(m)
		assert.NotNil(t, cmd, "should return a command")

		msg := cmd()
		_, ok := msg.(enterBulkSilenceMsg)
		assert.True(t, ok, "command should produce enterBulkSilenceMsg, got %T", msg)
	})
}

func TestChordViewLog_ReturnsCommand(t *testing.T) {
	t.Run("chordViewLog returns a non-nil tea.Cmd", func(t *testing.T) {
		m := createTestModel()
		m.logFilePath = "/tmp/test-srepd-debug.log"

		_, cmd := chordViewLog(m)
		assert.NotNil(t, cmd, "chordViewLog should return a non-nil command")
	})

	t.Run("chordViewLog command reads log file", func(t *testing.T) {
		m := createTestModel()
		m.logFilePath = "/tmp/nonexistent-srepd-chord-test.log"

		_, cmd := chordViewLog(m)
		assert.NotNil(t, cmd, "should return a command")

		// Execute the command to verify it produces a logFileContentMsg
		result := cmd()
		msg, ok := result.(logFileContentMsg)
		assert.True(t, ok, "command should produce a logFileContentMsg, got %T", result)
		assert.Contains(t, string(msg), "No log file found",
			"should indicate log file not found for nonexistent path")
	})
}
