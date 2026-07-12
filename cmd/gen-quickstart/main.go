package main

import (
	"fmt"
	"os"

	"github.com/clcollins/srepd/pkg/tui"
)

func main() {
	keys := tui.KeyBindingEntries()
	chords := tui.ChordEntries()
	inputs := tui.InputCommandEntries()

	md := tui.GenerateQuickstartMarkdown(keys, chords, inputs)

	if _, err := fmt.Fprint(os.Stdout, md); err != nil {
		os.Exit(1)
	}
}
