package tui

import (
	"fmt"
	"strings"
)

type KeyBindingEntry struct {
	Keys string
	Help string
}

type ChordEntry struct {
	Key         string
	Description string
}

type InputCommandEntry struct {
	Command     string
	Description string
}

func KeyBindingEntries() []KeyBindingEntry {
	km := defaultKeyMap
	bindings := []struct {
		keys string
		help string
	}{
		{km.Help.Help().Key, km.Help.Help().Desc},
		{km.Up.Help().Key, km.Up.Help().Desc},
		{km.Down.Help().Key, km.Down.Help().Desc},
		{km.Top.Help().Key, km.Top.Help().Desc},
		{km.Bottom.Help().Key, km.Bottom.Help().Desc},
		{km.Enter.Help().Key, km.Enter.Help().Desc},
		{km.Back.Help().Key, km.Back.Help().Desc},
		{km.Quit.Help().Key, km.Quit.Help().Desc},
		{km.Team.Help().Key, km.Team.Help().Desc},
		{km.Refresh.Help().Key, km.Refresh.Help().Desc},
		{km.AutoRefresh.Help().Key, km.AutoRefresh.Help().Desc},
		{km.Note.Help().Key, km.Note.Help().Desc},
		{km.Silence.Help().Key, km.Silence.Help().Desc},
		{km.Ack.Help().Key, km.Ack.Help().Desc},
		{km.UnAck.Help().Key, km.UnAck.Help().Desc},
		{km.AutoAck.Help().Key, km.AutoAck.Help().Desc},
		{km.Urgency.Help().Key, km.Urgency.Help().Desc},
		{km.Input.Help().Key, km.Input.Help().Desc},
		{km.Login.Help().Key, km.Login.Help().Desc},
		{km.Open.Help().Key, km.Open.Help().Desc},
		{km.SOP.Help().Key, km.SOP.Help().Desc},
		{km.ViewLog.Help().Key, km.ViewLog.Help().Desc},
		{km.Merge.Help().Key, km.Merge.Help().Desc},
		{km.Watcher.Help().Key, km.Watcher.Help().Desc},
		{km.TabNext.Help().Key, km.TabNext.Help().Desc},
		{km.TabPrev.Help().Key, km.TabPrev.Help().Desc},
		{km.ViewDocs.Help().Key, km.ViewDocs.Help().Desc},
	}

	entries := make([]KeyBindingEntry, 0, len(bindings))
	for _, b := range bindings {
		entries = append(entries, KeyBindingEntry{Keys: b.keys, Help: b.help})
	}
	return entries
}

func ChordEntries() []ChordEntry {
	var entries []ChordEntry
	for _, r := range chordRegistry {
		if r.Hidden {
			continue
		}
		entries = append(entries, ChordEntry{Key: r.Key, Description: r.Description})
	}
	return entries
}

func InputCommandEntries() []InputCommandEntry {
	return []InputCommandEntry{
		{Command: ":agent <query>", Description: "ask Claude AI"},
		{Command: ":watcher <query>", Description: "query AI watcher"},
		{Command: ":flag cluster <id>", Description: "flag incidents by cluster ID"},
		{Command: ":flag org <pattern>", Description: "flag incidents by org name"},
		{Command: ":unflag <id>", Description: "remove a flag condition by ID"},
		{Command: ":unflag all", Description: "clear all flag conditions"},
		{Command: ":flags", Description: "list all flag conditions"},
		{Command: ":flags save [path]", Description: "save flags to file"},
		{Command: ":flags load [path]", Description: "load flags from file"},
	}
}

func GenerateQuickstartMarkdown(keys []KeyBindingEntry, chords []ChordEntry, inputs []InputCommandEntry) string {
	var b strings.Builder

	b.WriteString("# Quickstart\n\n")

	b.WriteString("## Key Bindings\n\n")
	b.WriteString("| Key | Action |\n")
	b.WriteString("|-----|--------|\n")
	for _, e := range keys {
		fmt.Fprintf(&b, "| %s | %s |\n", e.Keys, e.Help)
	}

	b.WriteString("\n## Chord Commands (ctrl+x + key)\n\n")
	b.WriteString("| Key | Action |\n")
	b.WriteString("|-----|--------|\n")
	for _, e := range chords {
		fmt.Fprintf(&b, "| %s | %s |\n", e.Key, e.Description)
	}

	b.WriteString("\n## Input Commands\n\n")
	b.WriteString("| Command | Action |\n")
	b.WriteString("|---------|--------|\n")
	for _, e := range inputs {
		fmt.Fprintf(&b, "| %s | %s |\n", e.Command, e.Description)
	}

	return b.String()
}
