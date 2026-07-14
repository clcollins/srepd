package launcher

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

// DetectTerminals probes this system for known terminal emulators and
// returns them ranked: the terminal identified by $TERM_PROGRAM first, tmux
// next when running inside a session, then the rest in probe order. On
// darwin, Terminal.app and iTerm2 are always candidates (launched via
// osascript, not PATH). lookPath, getenv, and goos are injectable for tests;
// production callers pass exec.LookPath, os.Getenv, runtime.GOOS.
func DetectTerminals(lookPath func(string) (string, error), getenv func(string) string, goos string) []DetectedTerminal {
	var found []DetectedTerminal

	// Inside a tmux session, a new window lands in the session — offer it.
	if getenv("TMUX") != "" {
		if _, err := lookPath("tmux"); err == nil {
			found = append(found, DetectedTerminal{Name: "tmux", Command: "tmux"})
		}
	}

	for _, name := range detectableTerminals {
		if _, err := lookPath(name); err == nil {
			found = append(found, DetectedTerminal{Name: name, Command: name})
		}
	}

	if goos == "darwin" {
		found = append(found,
			DetectedTerminal{Name: "terminal", Command: "terminal"},
			DetectedTerminal{Name: "iterm2", Command: "iterm2"},
		)
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
