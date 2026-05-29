package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TerminalProfile defines how a terminal emulator expects to receive
// the command it should execute.
type TerminalProfile interface {
	// Name returns a human-readable name for this profile.
	Name() string

	// BuildCommand constructs the full exec argument slice by combining
	// the terminal's own arguments with the login command according to
	// the terminal's expected syntax.
	BuildCommand(terminalArgs []string, loginCmd []string) ([]string, error)
}

// SeparatorProfile is used by terminals that expect a "--" separator
// between their own flags and the executed command (gnome-terminal,
// ptyxis, wezterm, BlackBox, tmux).
type SeparatorProfile struct {
	terminalName string
}

func (p *SeparatorProfile) Name() string {
	return fmt.Sprintf("separator (%s)", p.terminalName)
}

func (p *SeparatorProfile) BuildCommand(terminalArgs []string, loginCmd []string) ([]string, error) {
	if len(terminalArgs) == 0 {
		return nil, fmt.Errorf("terminal args must not be empty")
	}
	if len(loginCmd) == 0 {
		return nil, fmt.Errorf("login command must not be empty")
	}

	cmd := make([]string, 0, len(terminalArgs)+1+len(loginCmd))
	cmd = append(cmd, terminalArgs...)
	cmd = append(cmd, "--")
	cmd = append(cmd, loginCmd...)
	return cmd, nil
}

// FlagProfile is used by terminals that require a specific flag before
// the executed command (konsole -e, alacritty -e, terminator --execute).
type FlagProfile struct {
	terminalName string
	flag         string
}

func (p *FlagProfile) Name() string {
	return fmt.Sprintf("flag[%s] (%s)", p.flag, p.terminalName)
}

func (p *FlagProfile) BuildCommand(terminalArgs []string, loginCmd []string) ([]string, error) {
	if len(terminalArgs) == 0 {
		return nil, fmt.Errorf("terminal args must not be empty")
	}
	if len(loginCmd) == 0 {
		return nil, fmt.Errorf("login command must not be empty")
	}

	cmd := make([]string, 0, len(terminalArgs)+1+len(loginCmd))
	cmd = append(cmd, terminalArgs...)
	cmd = append(cmd, p.flag)
	cmd = append(cmd, loginCmd...)
	return cmd, nil
}

// DirectProfile is used by terminals where the executed command follows
// terminal flags directly with no separator (kitty, foot, Contour).
type DirectProfile struct {
	terminalName string
}

func (p *DirectProfile) Name() string {
	return fmt.Sprintf("direct (%s)", p.terminalName)
}

func (p *DirectProfile) BuildCommand(terminalArgs []string, loginCmd []string) ([]string, error) {
	if len(terminalArgs) == 0 {
		return nil, fmt.Errorf("terminal args must not be empty")
	}
	if len(loginCmd) == 0 {
		return nil, fmt.Errorf("login command must not be empty")
	}

	cmd := make([]string, 0, len(terminalArgs)+len(loginCmd))
	cmd = append(cmd, terminalArgs...)
	cmd = append(cmd, loginCmd...)
	return cmd, nil
}

// GenericProfile is the fallback that preserves the original launcher
// behavior: terminal args and login command are simply concatenated.
type GenericProfile struct{}

func (p *GenericProfile) Name() string {
	return "generic"
}

func (p *GenericProfile) BuildCommand(terminalArgs []string, loginCmd []string) ([]string, error) {
	if len(terminalArgs) == 0 {
		return nil, fmt.Errorf("terminal args must not be empty")
	}
	if len(loginCmd) == 0 {
		return nil, fmt.Errorf("login command must not be empty")
	}

	cmd := make([]string, 0, len(terminalArgs)+len(loginCmd))
	cmd = append(cmd, terminalArgs...)
	cmd = append(cmd, loginCmd...)
	return cmd, nil
}

// separatorTerminals maps executable names to their profile category.
var separatorTerminals = map[string]bool{
	"gnome-terminal": true,
	"ptyxis":         true,
	"wezterm":        true,
	"blackbox":       true,
	"tmux":           true,
}

// flagTerminals maps executable names to the flag they use.
var flagTerminals = map[string]string{
	"konsole":    "-e",
	"alacritty":  "-e",
	"ghostty":    "-e",
	"terminator": "--execute",
}

// directTerminals maps executable names to their profile category.
var directTerminals = map[string]bool{
	"kitty":   true,
	"foot":    true,
	"contour": true,
}

// flatpakAppTerminals maps Flatpak application IDs to executable names
// so that detection works for flatpak-launched terminals.
var flatpakAppTerminals = map[string]string{
	"org.gnome.Terminal":          "gnome-terminal",
	"org.gnome.Ptyxis":            "ptyxis",
	"org.wezfurlong.wezterm":      "wezterm",
	"com.raggesilver.BlackBox":    "blackbox",
	"org.kde.konsole":             "konsole",
	"org.codeberg.dnkl.foot":      "foot",
	"net.kovidgoyal.kitty":        "kitty",
	"io.github.AtomsDevs.Contour": "contour",
}

// DetectTerminalProfile inspects the terminal command string and returns
// the appropriate TerminalProfile. It checks the base executable name
// first, then checks for Flatpak app IDs. Unknown terminals get the
// GenericProfile fallback.
func DetectTerminalProfile(terminalCmd string) TerminalProfile {
	if terminalCmd == "" {
		return &GenericProfile{}
	}

	parts := strings.Fields(terminalCmd)
	if len(parts) == 0 {
		return &GenericProfile{}
	}

	// Extract the executable name (basename, no path).
	execName := filepath.Base(parts[0])

	// If the first token is a bare Flatpak app ID (reverse-DNS with 2+
	// dots and no spaces), resolve the inner terminal from it.
	if isFlatpakAppID(parts[0]) {
		if mapped, ok := flatpakAppTerminals[parts[0]]; ok {
			execName = mapped
		}
		return profileForExecName(execName)
	}

	// For flatpak commands, try to find the app ID in the arguments.
	if execName == "flatpak" || execName == "flatpak-spawn" {
		resolvedName := resolveFlatpakTerminal(parts)
		if resolvedName != "" {
			execName = resolvedName
		}
	}

	return profileForExecName(execName)
}

// resolveFlatpakTerminal scans the argument list for a known Flatpak
// application ID and returns the corresponding executable name.
func resolveFlatpakTerminal(parts []string) string {
	for _, arg := range parts {
		if mapped, ok := flatpakAppTerminals[arg]; ok {
			return mapped
		}
	}
	return ""
}

// profileForExecName returns the correct profile for a given executable name.
func profileForExecName(execName string) TerminalProfile {
	if separatorTerminals[execName] {
		return &SeparatorProfile{terminalName: execName}
	}
	if flag, ok := flagTerminals[execName]; ok {
		return &FlagProfile{terminalName: execName, flag: flag}
	}
	if directTerminals[execName] {
		return &DirectProfile{terminalName: execName}
	}
	return &GenericProfile{}
}

// isFlatpakAppID returns true if the string looks like a reverse-DNS
// Flatpak application ID (e.g., "org.kde.konsole"). A valid app ID has
// at least two dots, no spaces, and no path separators.
func isFlatpakAppID(s string) bool {
	if s == "" {
		return false
	}
	// Must not contain spaces (bare token only).
	if strings.Contains(s, " ") {
		return false
	}
	// Must not start with a path separator.
	if strings.HasPrefix(s, "/") {
		return false
	}
	// Must have at least two dots (three segments: org.kde.konsole).
	return strings.Count(s, ".") >= 2
}

// detectRedundantFlatpakPrefix checks whether a terminal command string
// starts with "flatpak run <app-id>" where <app-id> is a recognized
// Flatpak application ID. If so, it returns the app ID and true,
// indicating that the user can simplify their config to just the app ID.
// Commands using "flatpak-spawn" are not flagged as redundant because
// they serve a different purpose (running from inside a container).
func detectRedundantFlatpakPrefix(terminalCmd string) (string, bool) {
	parts := strings.Fields(terminalCmd)
	if len(parts) < 3 {
		return "", false
	}
	// Only flag "flatpak run <app-id>", not flatpak-spawn variants.
	if filepath.Base(parts[0]) != "flatpak" {
		return "", false
	}
	if parts[1] != "run" {
		return "", false
	}
	if isFlatpakAppID(parts[2]) {
		return parts[2], true
	}
	return "", false
}

// validateTerminalExists checks whether the terminal command is
// available on the system. It returns a non-empty warning message if the
// terminal cannot be found, or an empty string if everything looks fine.
// For Flatpak app IDs, it checks for the "flatpak" binary instead.
func validateTerminalExists(terminal string) string {
	if terminal == "" {
		return "terminal command is empty"
	}

	parts := strings.Fields(terminal)
	name := parts[0]

	// For Flatpak app IDs, check that "flatpak" is available.
	if isFlatpakAppID(name) {
		_, err := exec.LookPath("flatpak")
		if err != nil {
			return fmt.Sprintf("flatpak command not found in PATH; cluster login via %s may fail", name)
		}
		return ""
	}

	// Full path: use os.Stat.
	if strings.HasPrefix(name, "/") {
		_, err := os.Stat(name)
		if err != nil {
			return fmt.Sprintf("terminal path %s does not exist", name)
		}
		return ""
	}

	// Bare command: use exec.LookPath.
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Sprintf("terminal command %s not found in PATH", name)
	}
	return ""
}
