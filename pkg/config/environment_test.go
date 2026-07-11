package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The wizard's environment step (terminal/editor/agent) must flow through
// resolve → detect-changes → write, like every other wizard value.

func TestResolveFinalValues_EnvironmentFields(t *testing.T) {
	existing := ExistingConfig{
		Token:           "tok",
		Teams:           []string{"T1"},
		Terminal:        "gnome-terminal --",
		Editor:          "vim",
		AgentCLICommand: "claude --print",
	}

	rv, err := ResolveFinalValues(existing, WizardInputs{
		TokenInput:    "",
		KeepTeams:     true,
		TerminalInput: "kitty",
		EditorInput:   "  nano  ",
		AgentInput:    "claude --print",
		AgentTouched:  true,
	})
	assert.NoError(t, err)
	assert.Equal(t, "kitty", rv.Terminal)
	assert.Equal(t, "nano", rv.Editor, "editor input is trimmed")
	assert.Equal(t, "claude --print", rv.AgentCLICommand)
}

func TestResolveFinalValues_EnvironmentFallsBackToExisting(t *testing.T) {
	existing := ExistingConfig{
		Token:           "tok",
		Teams:           []string{"T1"},
		Terminal:        "gnome-terminal --",
		Editor:          "emacs",
		AgentCLICommand: "custom-agent",
	}

	rv, err := ResolveFinalValues(existing, WizardInputs{KeepTeams: true})
	assert.NoError(t, err)
	assert.Equal(t, "gnome-terminal --", rv.Terminal, "blank input keeps existing terminal")
	assert.Equal(t, "emacs", rv.Editor)
	assert.Equal(t, "custom-agent", rv.AgentCLICommand, "untouched agent keeps existing value")
}

func TestResolveFinalValues_AgentDisabled(t *testing.T) {
	existing := ExistingConfig{Token: "tok", Teams: []string{"T1"}, AgentCLICommand: "claude --print"}

	rv, err := ResolveFinalValues(existing, WizardInputs{
		KeepTeams:    true,
		AgentInput:   "",
		AgentTouched: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, "", rv.AgentCLICommand, "touched-and-empty means deliberately disabled")
}

func TestDetectChanges_EnvironmentFields(t *testing.T) {
	existing := ExistingConfig{
		Token:    "tok",
		Teams:    []string{"T1"},
		Terminal: "gnome-terminal --",
		Editor:   "vim",
	}
	final := ResolvedValues{
		Token:    "tok",
		Teams:    []string{"T1"},
		Terminal: "kitty",
		Editor:   "vim",
	}

	changes := DetectChanges(existing, final, "")
	assert.True(t, changes.TerminalChanged)
	assert.False(t, changes.EditorChanged)
	assert.True(t, changes.AnyChanged())
}

func TestBuildFullConfig_UsesResolvedEnvironment(t *testing.T) {
	final := ResolvedValues{
		Token:    "tok",
		Teams:    []string{"T1"},
		Terminal: "kitty",
		Editor:   "nano",
	}

	out := string(BuildFullConfig(final, nil, "", nil))
	assert.Contains(t, out, "terminal: kitty")
	assert.Contains(t, out, "editor: nano")
	assert.NotContains(t, out, "terminal: gnome-terminal --", "resolved terminal must replace the hardcoded default")
}

func TestBuildFullConfig_DefaultsWhenEnvironmentEmpty(t *testing.T) {
	final := ResolvedValues{Token: "tok", Teams: []string{"T1"}}

	out := string(BuildFullConfig(final, nil, "", nil))
	assert.Contains(t, out, "terminal: gnome-terminal --")
	assert.Contains(t, out, "editor: vim")
}

func TestBuildFullConfig_AgentDisabledWritten(t *testing.T) {
	final := ResolvedValues{Token: "tok", Teams: []string{"T1"}, AgentCLICommand: "", AgentTouched: true}

	out := string(BuildFullConfig(final, nil, "", nil))
	assert.Contains(t, out, `agent_cli_command: ""`, "a deliberate disable must be persisted")
}

func TestMergeIntoExistingConfig_EnvironmentUpserts(t *testing.T) {
	existing := []byte("token: tok\nteams:\n  - T1\nterminal: gnome-terminal --\n")
	final := ResolvedValues{Token: "tok", Teams: []string{"T1"}, Terminal: "kitty", Editor: "nano"}
	changes := ConfigChanges{TerminalChanged: true, EditorChanged: true}

	out, err := MergeIntoExistingConfig(existing, final, changes, nil, nil)
	assert.NoError(t, err)
	assert.Contains(t, string(out), "terminal: kitty")
	assert.Contains(t, string(out), "editor: nano")
	assert.True(t, strings.Contains(string(out), "token: tok"), "untouched keys preserved")
}
