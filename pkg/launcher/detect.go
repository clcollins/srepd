package launcher

import (
	"os"
	"path/filepath"
	"sort"
)

// DetectedTerminal describes a terminal emulator found on this system,
// with the config-ready `terminal` value to use for it. Command is the
// executable name for PATH-found terminals, the full binary path for
// macOS bundle-detected terminals, or an AppleScript identifier on macOS.
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

// macOSBundleTerminal describes a terminal installed as a macOS .app bundle.
// appName is the .app directory name and binaryRelPath is the executable
// path relative to the bundle root (e.g., "Contents/MacOS/kitty").
// Detection probes /Applications/<appName> and $HOME/Applications/<appName>
// in that order; the first hit wins.
type macOSBundleTerminal struct {
	appName       string // e.g., "kitty.app"
	binaryRelPath string // e.g., "Contents/MacOS/kitty"
}

// macOSBundleTerminals maps terminal names to their bundle metadata.
// DetectTerminalProfile resolves the profile from filepath.Base, so full
// binary paths work transparently.
//
// Ghostty is intentionally excluded from this map: its macOS CLI cannot
// reliably launch terminal windows. Ghostty >=1.3 supports AppleScript,
// which will be handled by a dedicated profile. See ghostty#5739, #10203.
var macOSBundleTerminals = map[string]macOSBundleTerminal{
	"alacritty": {appName: "Alacritty.app", binaryRelPath: "Contents/MacOS/alacritty"},
	"kitty":     {appName: "kitty.app", binaryRelPath: "Contents/MacOS/kitty"},
	"wezterm":   {appName: "WezTerm.app", binaryRelPath: "Contents/MacOS/wezterm"},
}

// DetectTerminals probes this system for known terminal emulators and
// returns them ranked: the terminal identified by $TERM_PROGRAM first, tmux
// next when running inside a session, then the rest in probe order. On
// darwin, Terminal.app is always a candidate (built into macOS); iTerm2 is
// offered only when iTerm.app exists; and bundle-installed terminals
// (kitty, alacritty, wezterm) are detected from /Applications/ and
// ~/Applications/ with their full binary path as Command. lookPath, getenv,
// goos, and statFn are injectable for tests; production callers pass
// exec.LookPath, os.Getenv, runtime.GOOS, os.Stat.
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
		bundleRoots := []string{"/Applications"}
		if home := getenv("HOME"); home != "" {
			bundleRoots = append(bundleRoots, filepath.Join(home, "Applications"))
		}

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
			bundle := macOSBundleTerminals[name]
			for _, root := range bundleRoots {
				appPath := filepath.Join(root, bundle.appName)
				if _, err := statFn(appPath); err == nil {
					binaryPath := filepath.Join(appPath, bundle.binaryRelPath)
					found = append(found, DetectedTerminal{Name: name, Command: binaryPath})
					foundSet[name] = true
					break
				}
			}
		}

		// Terminal.app is built into macOS — always available.
		found = append(found, DetectedTerminal{Name: "terminal", Command: "terminal"})
		foundSet["terminal"] = true

		// iTerm2 only when installed (check both roots).
		for _, root := range bundleRoots {
			if _, err := statFn(filepath.Join(root, "iTerm.app")); err == nil {
				found = append(found, DetectedTerminal{Name: "iterm2", Command: "iterm2"})
				foundSet["iterm2"] = true
				break
			}
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
