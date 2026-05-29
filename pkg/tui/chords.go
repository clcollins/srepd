package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// chordAction defines a single chord command triggered by a prefix key + second key.
type chordAction struct {
	Key         string
	Description string
	Handler     func(m model) (tea.Model, tea.Cmd)
}

// chordRegistry holds the chord key-to-description mappings without handler
// references, breaking the init cycle between chordActions and chordShowHelp.
var chordRegistry = []struct {
	Key         string
	Description string
}{
	{Key: "?", Description: "show chord help"},
	{Key: "d", Description: "view debug log"},
}

// getChordActions returns the full chord action list with handlers attached.
// This is a function rather than a package-level var to avoid an initialization
// cycle (chordShowHelp -> chordHelpText -> chordActions -> chordShowHelp).
func getChordActions() []chordAction {
	handlers := map[string]func(m model) (tea.Model, tea.Cmd){
		"?": chordShowHelp,
		"d": chordViewLog,
	}

	var actions []chordAction
	for _, entry := range chordRegistry {
		actions = append(actions, chordAction{
			Key:         entry.Key,
			Description: entry.Description,
			Handler:     handlers[entry.Key],
		})
	}
	return actions
}

// resolveChord looks up a chord action by its second key.
// Returns nil if no action is registered for the given key.
func resolveChord(key string) *chordAction {
	actions := getChordActions()
	for i := range actions {
		if actions[i].Key == key {
			return &actions[i]
		}
	}
	return nil
}

// chordShowHelp displays the list of available chord commands in the status bar.
func chordShowHelp(m model) (tea.Model, tea.Cmd) {
	m.setStatus(chordHelpText(m.chordPrefix))
	return m, nil
}

// chordViewLog is a placeholder for the future debug log viewer (Issue #203).
func chordViewLog(m model) (tea.Model, tea.Cmd) {
	m.setStatus("debug log viewer not yet implemented")
	return m, nil
}

// chordHelpText generates a human-readable help string listing all chord commands.
func chordHelpText(prefix string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Chords (%s + key): ", prefix)
	for i, entry := range chordRegistry {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s=%s", entry.Key, entry.Description)
	}
	return b.String()
}

// chordHelpBindings generates key.Binding entries for the help display.
// Each chord action is rendered as "prefix key" with its description.
func chordHelpBindings() []key.Binding {
	// Use a fixed prefix for display since we don't have model context here.
	// The actual prefix is configurable, but ctrl+x is the default and
	// is used in help display for consistency.
	prefix := "ctrl+x"
	var bindings []key.Binding
	for _, entry := range chordRegistry {
		bindings = append(bindings, key.NewBinding(
			key.WithKeys(prefix+" "+entry.Key),
			key.WithHelp(prefix+" "+entry.Key, entry.Description),
		))
	}
	return bindings
}
