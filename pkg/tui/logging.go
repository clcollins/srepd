package tui

import "log"

var debugLogging = false

// debug receives a string or strings, and logs it to the debug log if debugLogging is enabled
func debug(msg ...string) {
	if !debugLogging {
		return
	}
	log.Printf("%s\n", msg)
}
