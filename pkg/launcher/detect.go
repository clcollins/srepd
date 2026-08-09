package launcher

import (
	"os"
	"sort"
)

// DetectedTerminal describes a terminal emulator found on this system,
// with the config-ready `terminal` value to use for it. Profiles handle
// argument style at launch time, so Command is just the executable name
// (or AppleScript identifier on macOS).
type DetectedTerminal struct {
	Name    string
	Command string
}

// detectableTerminals is the PATH probe order for the known profiles —
// every executable DetectTerminalProfile understands, most common first.
// tmux is handled separately (only offered inside a session), and the
// macOS AppleScript terminals are appended on darwin.
var detectableTerminals = []string{
	"gnome-terminal",
	"ptyxis",
	"konsole",
	"wezterm",
	"kitty",
	"alacritty",
	"ghostty",
	"foot",
	"blackbox",
	"terminator",
	"contour",
}

// termProgramNames maps $TERM_PROGRAM values to detected-terminal names so
// the terminal the user is sitting in can be ranked first.
var termProgramNames = map[string]string{
	"WezTerm":        "wezterm",
	"ghostty":        "ghostty",
	"kitty":          "kitty",
	"tmux":           "tmux",
	"iTerm.app":      "iterm2",
	"Apple_Terminal": "terminal",
}

// macOSBundleTerminals maps terminal names to their /Applications/ bundle
// paths. On macOS, these terminals are commonly installed as .app bundles
// whose binaries are not on PATH.
var macOSBundleTerminals = map[string]string{
	"alacritty": "/Applications/Alacritty.app",
	"ghostty":   "/Applications/Ghostty.app",
	"kitty":     "/Applications/kitty.app",
	"wezterm":   "/Applications/WezTerm.app",
}

// DetectTerminals probes this system for known terminal emulators and
// returns them ranked: the terminal identified by $TERM_PROGRAM first, tmux
// next when running inside a session, then the rest in probe order. On
// darwin, Terminal.app is always a candidate (built into macOS); iTerm2 is
// offered only when /Applications/iTerm.app exists; and bundle-installed
// terminals (kitty, alacritty, wezterm, ghostty) are detected from
// /Applications/. lookPath, getenv, goos, and statFn are injectable for
// tests; production callers pass exec.LookPath, os.Getenv, runtime.GOOS,
// os.Stat.
func DetectTerminals(lookPath func(string) (string, error), getenv func(string) string, goos string, statFn func(string) (os.FileInfo, error)) []DetectedTerminal {
	var found []DetectedTerminal
	foundSet := make(map[string]bool)

	// Inside a tmux session, a new window lands in the session — offer it.
	if getenv("TMUX") != "" {
		if _, err := lookPath("tmux"); err == nil {
			found = append(found, DetectedTerminal{Name: "tmux", Command: "tmux"})
			foundSet["tmux"] = true
		}
	}

	for _, name := range detectableTerminals {
		if _, err := lookPath(name); err == nil {
			found = append(found, DetectedTerminal{Name: name, Command: name})
			foundSet[name] = true
		}
	}

	if goos == "darwin" {
		// Check for bundle-installed terminals not found via PATH.
		bundleNames := make([]string, 0, len(macOSBundleTerminals))
		for name := range macOSBundleTerminals {
			bundleNames = append(bundleNames, name)
		}
		sort.Strings(bundleNames)

		for _, name := range bundleNames {
			if foundSet[name] {
				continue
			}
			if _, err := statFn(macOSBundleTerminals[name]); err == nil {
				found = append(found, DetectedTerminal{Name: name, Command: name})
				foundSet[name] = true
			}
		}

		// Terminal.app is built into macOS — always available.
		found = append(found, DetectedTerminal{Name: "terminal", Command: "terminal"})
		foundSet["terminal"] = true

		// iTerm2 only when installed.
		if _, err := statFn("/Applications/iTerm.app"); err == nil {
			found = append(found, DetectedTerminal{Name: "iterm2", Command: "iterm2"})
			foundSet["iterm2"] = true
		}
	}

	// Rank the terminal the user is sitting in first.
	if current, ok := termProgramNames[getenv("TERM_PROGRAM")]; ok {
		for i, dt := range found {
			if dt.Name == current && i > 0 {
				found = append(found[:i], found[i+1:]...)
				found = append([]DetectedTerminal{dt}, found...)
				break
			}
		}
	}

	return found
}
