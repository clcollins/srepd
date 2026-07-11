package tui

import (
	"testing"

	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/stretchr/testify/assert"
)

// OB-5: the wizard's environment step offers detected terminals; the current
// setting leads, annotated — with a warning when its binary is missing.
func TestBuildTerminalOptions_CurrentFirstAnnotated(t *testing.T) {
	detected := []launcher.DetectedTerminal{
		{Name: "konsole", Command: "konsole"},
		{Name: "kitty", Command: "kitty"},
	}

	opts := buildTerminalOptions("gnome-terminal --", true, detected)

	assert.Equal(t, "gnome-terminal --", opts[0].Value)
	assert.Contains(t, opts[0].Key, "current")
	assert.NotContains(t, opts[0].Key, "not found")
	assert.Equal(t, "konsole", opts[1].Value)
	assert.Equal(t, "kitty", opts[2].Value)
}

func TestBuildTerminalOptions_MissingCurrentWarns(t *testing.T) {
	opts := buildTerminalOptions("gnome-terminal --", false, []launcher.DetectedTerminal{
		{Name: "kitty", Command: "kitty"},
	})

	assert.Equal(t, "gnome-terminal --", opts[0].Value)
	assert.Contains(t, opts[0].Key, "not found on this system")
}

func TestBuildTerminalOptions_CurrentDeduplicated(t *testing.T) {
	opts := buildTerminalOptions("kitty", true, []launcher.DetectedTerminal{
		{Name: "kitty", Command: "kitty"},
		{Name: "foot", Command: "foot"},
	})

	assert.Len(t, opts, 2, "the current terminal must not be listed twice")
	assert.Equal(t, "kitty", opts[0].Value)
	assert.Equal(t, "foot", opts[1].Value)
}

// Editor default: existing config → $EDITOR → $VISUAL → vim.
func TestResolveEditorDefault(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	assert.Equal(t, "emacs", resolveEditorDefault("emacs", env(map[string]string{"EDITOR": "nano"})),
		"existing config wins")
	assert.Equal(t, "nano", resolveEditorDefault("", env(map[string]string{"EDITOR": "nano", "VISUAL": "code"})),
		"$EDITOR beats $VISUAL")
	assert.Equal(t, "code", resolveEditorDefault("", env(map[string]string{"VISUAL": "code"})))
	assert.Equal(t, "vim", resolveEditorDefault("", env(nil)), "final fallback is vim")
}

// #324: offer AI setup only when the claude CLI is actually on PATH, and not
// when the user has already customized the agent command.
func TestShouldOfferAgentSetup(t *testing.T) {
	defaultCmd := "claude --print"

	assert.True(t, shouldOfferAgentSetup(defaultCmd, true), "default command + claude present → offer")
	assert.True(t, shouldOfferAgentSetup("", true), "unset command + claude present → offer")
	assert.False(t, shouldOfferAgentSetup(defaultCmd, false), "no claude on PATH → silent skip")
	assert.False(t, shouldOfferAgentSetup("ollama run llama", true), "customized command → already configured, skip")
}
