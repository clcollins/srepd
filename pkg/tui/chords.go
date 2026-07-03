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
	Hidden      bool
}{
	{Key: "?", Description: "show chord help"},
	{Key: "b", Description: "rosa-boundary login"},
	{Key: "d", Description: "view debug log"},
	{Key: "s", Description: "bulk silence", Hidden: true},
}

// getChordActions returns the full chord action list with handlers attached.
// This is a function rather than a package-level var to avoid an initialization
// cycle (chordShowHelp -> chordHelpText -> chordActions -> chordShowHelp).
func getChordActions() []chordAction {
	handlers := map[string]func(m model) (tea.Model, tea.Cmd){
		"?": chordShowHelp,
		"b": chordRosaBoundaryLogin,
		"d": chordViewLog,
		"s": chordBulkSilence,
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

// chordShowHelp displays the list of available chord commands in the help
// section at the bottom of the screen, temporarily replacing the regular help.
func chordShowHelp(m model) (tea.Model, tea.Cmd) {
	m.chordHelpActive = true
	m.help.ShowAll = true
	m.recomputeLayout()
	return m, nil
}

// chordKeymap implements help.KeyMap to display chord bindings in the help section.
type chordKeymap struct {
	prefix string
}

func (k chordKeymap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(
			key.WithKeys("any"),
			key.WithHelp("any key", "dismiss chord help"),
		),
	}
}

func (k chordKeymap) FullHelp() [][]key.Binding {
	var bindings []key.Binding
	for _, entry := range chordRegistry {
		if entry.Hidden {
			continue
		}
		bindings = append(bindings, key.NewBinding(
			key.WithKeys(k.prefix+" "+entry.Key),
			key.WithHelp(k.prefix+" "+entry.Key, entry.Description),
		))
	}
	return [][]key.Binding{bindings}
}

// chordViewLog opens the debug log viewer (same as ctrl+l).
func chordViewLog(m model) (tea.Model, tea.Cmd) {
	return m, readLogFile(m.logFilePath)
}

// chordHelpText generates a human-readable help string listing all chord commands.
func chordHelpText(prefix string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Chords (%s + key): ", prefix)
	first := true
	for _, entry := range chordRegistry {
		if entry.Hidden {
			continue
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&b, "%s=%s", entry.Key, entry.Description)
	}
	return b.String()
}

func chordRosaBoundaryLogin(m model) (tea.Model, tea.Cmd) {
	if !m.rosaBoundaryLauncher.Enabled {
		m.setStatus("rosa-boundary not configured")
		return m, nil
	}
	if m.viewingIncident {
		if m.selectedIncident == nil {
			m.setStatus("no incident selected")
			return m, nil
		}
		if !m.incidentAlertsLoaded {
			m.setStatus("Loading incident alerts, please wait...")
			return m, nil
		}
		return m, func() tea.Msg { return rosaBoundaryLoginMsg("login") }
	}
	return m, doIfIncidentSelected(&m, func() tea.Msg {
		return waitForSelectedIncidentThenDoMsg{
			action: func() tea.Msg { return rosaBoundaryLoginMsg("login") },
			msg:    "wait",
		}
	})
}

// chordBulkSilence enters the bulk-silence incident selection mode.
func chordBulkSilence(m model) (tea.Model, tea.Cmd) {
	if len(m.incidentList) == 0 {
		m.setStatus("no incidents to silence")
		return m, nil
	}
	return m, func() tea.Msg { return enterBulkSilenceMsg{} }
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
		if entry.Hidden {
			continue
		}
		bindings = append(bindings, key.NewBinding(
			key.WithKeys(prefix+" "+entry.Key),
			key.WithHelp(prefix+" "+entry.Key, entry.Description),
		))
	}
	return bindings
}
