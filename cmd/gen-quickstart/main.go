package main

import (
	"fmt"
	"os"

	"github.com/openshift-online/srepd/pkg/tui"
)

func main() {
	keys := tui.KeyBindingEntries()
	chords := tui.ChordEntries()
	inputs := tui.InputCommandEntries()
	chatMode := tui.ChatModeEntries()

	md := tui.GenerateQuickstartMarkdown(keys, chords, inputs, chatMode)

	if _, err := fmt.Fprint(os.Stdout, md); err != nil {
		os.Exit(1)
	}
}
