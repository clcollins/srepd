package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyBindingEntries(t *testing.T) {
	entries := KeyBindingEntries()

	assert.NotEmpty(t, entries, "should return at least one key binding entry")

	for _, e := range entries {
		assert.NotEmpty(t, e.Keys, "Keys should not be empty for entry with Help=%q", e.Help)
		assert.NotEmpty(t, e.Help, "Help should not be empty for entry with Keys=%q", e.Keys)
	}

	helpTexts := make(map[string]bool)
	for _, e := range entries {
		helpTexts[e.Help] = true
	}
	assert.True(t, helpTexts["help"], "should include help binding")
	assert.True(t, helpTexts["acknowledge"], "should include acknowledge binding")
	assert.True(t, helpTexts["quit"], "should include quit binding")
	assert.True(t, helpTexts["view"], "should include enter/view binding")
	assert.True(t, helpTexts["next tab"], "should include tab navigation")
	assert.True(t, helpTexts["docs"], "should include docs binding")
}

func TestChordEntries(t *testing.T) {
	entries := ChordEntries()

	assert.NotEmpty(t, entries, "should return at least one chord entry")

	for _, e := range entries {
		assert.NotEmpty(t, e.Key, "Key should not be empty for entry with Description=%q", e.Description)
		assert.NotEmpty(t, e.Description, "Description should not be empty for entry with Key=%q", e.Key)
	}

	descriptions := make(map[string]bool)
	for _, e := range entries {
		descriptions[e.Description] = true
	}
	assert.True(t, descriptions["show chord help"], "should include chord help")
	assert.True(t, descriptions["rosa-boundary login"], "should include rosa-boundary login")

	for _, e := range entries {
		assert.NotEqual(t, "bulk silence", e.Description, "should not include hidden chord commands")
	}
}

func TestInputCommandEntries(t *testing.T) {
	entries := InputCommandEntries()

	assert.NotEmpty(t, entries, "should return at least one input command entry")

	for _, e := range entries {
		assert.NotEmpty(t, e.Command, "Command should not be empty for entry with Description=%q", e.Description)
		assert.NotEmpty(t, e.Description, "Description should not be empty for entry with Command=%q", e.Command)
	}

	commands := make(map[string]bool)
	for _, e := range entries {
		commands[e.Command] = true
	}
	assert.True(t, commands[":agent <query>"], "should include agent command")
	assert.True(t, commands[":watcher <query>"], "should include watcher command")
	assert.True(t, commands[":flag cluster <id>"], "should include flag cluster command")
	assert.True(t, commands[":flag org <pattern>"], "should include flag org command")
	assert.True(t, commands[":unflag <id>"], "should include unflag command")
	assert.True(t, commands[":unflag all"], "should include unflag all command")
	assert.True(t, commands[":flags"], "should include flags list command")
	assert.True(t, commands[":flags save [path]"], "should include flags save command")
	assert.True(t, commands[":flags load [path]"], "should include flags load command")
}

func TestGenerateQuickstartMarkdown(t *testing.T) {
	keys := []KeyBindingEntry{
		{Keys: "h", Help: "help"},
		{Keys: "a", Help: "acknowledge"},
	}
	chords := []ChordEntry{
		{Key: "?", Description: "show chord help"},
	}
	inputs := []InputCommandEntry{
		{Command: ":agent <query>", Description: "ask Claude AI"},
	}

	result := GenerateQuickstartMarkdown(keys, chords, inputs)

	assert.Contains(t, result, "# Quickstart")
	assert.Contains(t, result, "## Key Bindings")
	assert.Contains(t, result, "help")
	assert.Contains(t, result, "acknowledge")
	assert.Contains(t, result, "## Chord Commands (ctrl+x + key)")
	assert.Contains(t, result, "show chord help")
	assert.Contains(t, result, "## Input Commands")
	assert.Contains(t, result, ":agent <query>")
	assert.Contains(t, result, "ask Claude AI")
}

