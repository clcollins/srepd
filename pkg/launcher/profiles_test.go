package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Detection tests ---

func TestDetectProfile_GnomeTerminal(t *testing.T) {
	profile := DetectTerminalProfile("gnome-terminal --window")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "gnome-terminal")
}

func TestDetectProfile_Ptyxis(t *testing.T) {
	profile := DetectTerminalProfile("ptyxis --new-window")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "ptyxis")
}

func TestDetectProfile_Wezterm(t *testing.T) {
	profile := DetectTerminalProfile("wezterm start")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "wezterm")
}

func TestDetectProfile_Tmux(t *testing.T) {
	profile := DetectTerminalProfile("tmux new-window -n test")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "tmux")
}

func TestDetectProfile_Konsole(t *testing.T) {
	profile := DetectTerminalProfile("konsole --new-tab")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "konsole")
}

func TestDetectProfile_Alacritty(t *testing.T) {
	profile := DetectTerminalProfile("alacritty --title test")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "alacritty")
}

func TestDetectProfile_Terminator(t *testing.T) {
	profile := DetectTerminalProfile("terminator")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "terminator")
}

func TestDetectProfile_Kitty(t *testing.T) {
	profile := DetectTerminalProfile("kitty --title test")
	assert.IsType(t, &DirectProfile{}, profile)
	assert.Contains(t, profile.Name(), "kitty")
}

func TestDetectProfile_Foot(t *testing.T) {
	profile := DetectTerminalProfile("foot")
	assert.IsType(t, &DirectProfile{}, profile)
	assert.Contains(t, profile.Name(), "foot")
}

func TestDetectProfile_Contour(t *testing.T) {
	profile := DetectTerminalProfile("contour")
	assert.IsType(t, &DirectProfile{}, profile)
	assert.Contains(t, profile.Name(), "contour")
}

func TestDetectProfile_Unknown(t *testing.T) {
	profile := DetectTerminalProfile("/usr/bin/xterm")
	assert.IsType(t, &GenericProfile{}, profile)
	assert.Equal(t, "generic", profile.Name())
}

func TestDetectProfile_Empty(t *testing.T) {
	profile := DetectTerminalProfile("")
	assert.IsType(t, &GenericProfile{}, profile)
}

func TestDetectProfile_FlatpakPtyxis(t *testing.T) {
	profile := DetectTerminalProfile("flatpak run org.gnome.Ptyxis --new-window")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "ptyxis")
}

func TestDetectProfile_FlatpakSpawnPtyxis(t *testing.T) {
	profile := DetectTerminalProfile("flatpak-spawn --host flatpak run org.gnome.Ptyxis")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "ptyxis")
}

func TestDetectProfile_FlatpakKonsole(t *testing.T) {
	profile := DetectTerminalProfile("flatpak run org.kde.konsole")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "konsole")
}

func TestDetectProfile_FlatpakKitty(t *testing.T) {
	profile := DetectTerminalProfile("flatpak run net.kovidgoyal.kitty")
	assert.IsType(t, &DirectProfile{}, profile)
	assert.Contains(t, profile.Name(), "kitty")
}

func TestDetectProfile_FlatpakUnknown(t *testing.T) {
	profile := DetectTerminalProfile("flatpak run com.unknown.Terminal")
	assert.IsType(t, &GenericProfile{}, profile)
}

func TestDetectProfile_FullPath(t *testing.T) {
	profile := DetectTerminalProfile("/usr/bin/gnome-terminal")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "gnome-terminal")
}

// --- BuildCommand tests ---

func TestSeparatorProfile_BuildCommand(t *testing.T) {
	p := &SeparatorProfile{terminalName: "gnome-terminal"}
	termArgs := []string{"gnome-terminal", "--window"}
	loginCmd := []string{"ocm-container", "-C", "abc123"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	expected := []string{"gnome-terminal", "--window", "--", "ocm-container", "-C", "abc123"}
	assert.Equal(t, expected, cmd)
}

func TestSeparatorProfile_BuildCommand_EmptyTerminalArgs(t *testing.T) {
	p := &SeparatorProfile{terminalName: "gnome-terminal"}
	_, err := p.BuildCommand([]string{}, []string{"cmd"})
	assert.Error(t, err)
}

func TestSeparatorProfile_BuildCommand_EmptyLoginCmd(t *testing.T) {
	p := &SeparatorProfile{terminalName: "gnome-terminal"}
	_, err := p.BuildCommand([]string{"gnome-terminal"}, []string{})
	assert.Error(t, err)
}

func TestFlagProfile_BuildCommand(t *testing.T) {
	p := &FlagProfile{terminalName: "konsole", flag: "-e"}
	termArgs := []string{"konsole", "--new-tab"}
	loginCmd := []string{"ocm-container", "-C", "abc123"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	expected := []string{"konsole", "--new-tab", "-e", "ocm-container", "-C", "abc123"}
	assert.Equal(t, expected, cmd)
}

func TestFlagProfile_BuildCommand_Execute(t *testing.T) {
	p := &FlagProfile{terminalName: "terminator", flag: "--execute"}
	termArgs := []string{"terminator"}
	loginCmd := []string{"bash", "-c", "echo hello"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	expected := []string{"terminator", "--execute", "bash", "-c", "echo hello"}
	assert.Equal(t, expected, cmd)
}

func TestFlagProfile_BuildCommand_EmptyTerminalArgs(t *testing.T) {
	p := &FlagProfile{terminalName: "konsole", flag: "-e"}
	_, err := p.BuildCommand([]string{}, []string{"cmd"})
	assert.Error(t, err)
}

func TestFlagProfile_BuildCommand_EmptyLoginCmd(t *testing.T) {
	p := &FlagProfile{terminalName: "konsole", flag: "-e"}
	_, err := p.BuildCommand([]string{"konsole"}, []string{})
	assert.Error(t, err)
}

func TestDirectProfile_BuildCommand(t *testing.T) {
	p := &DirectProfile{terminalName: "kitty"}
	termArgs := []string{"kitty", "--title", "cluster"}
	loginCmd := []string{"ocm-container", "-C", "abc123"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	expected := []string{"kitty", "--title", "cluster", "ocm-container", "-C", "abc123"}
	assert.Equal(t, expected, cmd)
}

func TestDirectProfile_BuildCommand_EmptyTerminalArgs(t *testing.T) {
	p := &DirectProfile{terminalName: "kitty"}
	_, err := p.BuildCommand([]string{}, []string{"cmd"})
	assert.Error(t, err)
}

func TestDirectProfile_BuildCommand_EmptyLoginCmd(t *testing.T) {
	p := &DirectProfile{terminalName: "kitty"}
	_, err := p.BuildCommand([]string{"kitty"}, []string{})
	assert.Error(t, err)
}

func TestGenericProfile_BuildCommand(t *testing.T) {
	p := &GenericProfile{}
	termArgs := []string{"xterm", "-e"}
	loginCmd := []string{"ocm-container", "-C", "abc123"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	expected := []string{"xterm", "-e", "ocm-container", "-C", "abc123"}
	assert.Equal(t, expected, cmd)
}

func TestGenericProfile_BuildCommand_EmptyTerminalArgs(t *testing.T) {
	p := &GenericProfile{}
	_, err := p.BuildCommand([]string{}, []string{"cmd"})
	assert.Error(t, err)
}

func TestGenericProfile_BuildCommand_EmptyLoginCmd(t *testing.T) {
	p := &GenericProfile{}
	_, err := p.BuildCommand([]string{"xterm"}, []string{})
	assert.Error(t, err)
}

// --- Profile Name tests ---

func TestSeparatorProfile_Name(t *testing.T) {
	p := &SeparatorProfile{terminalName: "ptyxis"}
	assert.Equal(t, "separator (ptyxis)", p.Name())
}

func TestFlagProfile_Name(t *testing.T) {
	p := &FlagProfile{terminalName: "alacritty", flag: "-e"}
	assert.Equal(t, "flag[-e] (alacritty)", p.Name())
}

func TestDirectProfile_Name(t *testing.T) {
	p := &DirectProfile{terminalName: "foot"}
	assert.Equal(t, "direct (foot)", p.Name())
}

func TestGenericProfile_Name(t *testing.T) {
	p := &GenericProfile{}
	assert.Equal(t, "generic", p.Name())
}

// --- Flatpak app ID detection tests (Feature 1) ---

func TestIsFlatpakAppID_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"kde konsole", "org.kde.konsole"},
		{"gnome ptyxis", "org.gnome.Ptyxis"},
		{"blackbox", "com.raggesilver.BlackBox"},
		{"kitty", "net.kovidgoyal.kitty"},
		{"wezterm", "org.wezfurlong.wezterm"},
		{"foot", "org.codeberg.dnkl.foot"},
		{"contour", "io.github.AtomsDevs.Contour"},
		{"gnome terminal", "org.gnome.Terminal"},
		{"unknown flatpak app", "com.example.SomeTerminal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, isFlatpakAppID(tt.input), "expected %q to be detected as a Flatpak app ID", tt.input)
		})
	}
}

func TestIsFlatpakAppID_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"bare command", "konsole"},
		{"full path", "/usr/bin/konsole"},
		{"single dot", "org.konsole"},
		{"empty string", ""},
		{"command with args", "konsole --new-tab"},
		{"flatpak run prefix", "flatpak run org.kde.konsole"},
		{"path with dots", "/usr/local/bin/my.terminal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, isFlatpakAppID(tt.input), "expected %q to NOT be detected as a Flatpak app ID", tt.input)
		})
	}
}

// --- Flatpak app ID auto-detection in profile detection (Feature 1) ---

func TestDetectProfile_FlatpakAppID(t *testing.T) {
	// When a bare Flatpak app ID is provided (no "flatpak run" prefix),
	// DetectTerminalProfile should still resolve the inner terminal.
	profile := DetectTerminalProfile("org.kde.konsole")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "konsole")
}

func TestDetectProfile_FlatpakAppID_Ptyxis(t *testing.T) {
	profile := DetectTerminalProfile("org.gnome.Ptyxis")
	assert.IsType(t, &SeparatorProfile{}, profile)
	assert.Contains(t, profile.Name(), "ptyxis")
}

func TestDetectProfile_FlatpakAppID_UnknownApp(t *testing.T) {
	// Unknown Flatpak app IDs should still be detected as Flatpak
	// but fall back to GenericProfile since we cannot resolve the inner terminal.
	profile := DetectTerminalProfile("com.example.SomeTerminal")
	assert.IsType(t, &GenericProfile{}, profile)
}

// --- Ghostty terminal support (Feature 4) ---

func TestDetectProfile_Ghostty(t *testing.T) {
	profile := DetectTerminalProfile("ghostty")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "ghostty")
}

func TestDetectProfile_Ghostty_WithArgs(t *testing.T) {
	profile := DetectTerminalProfile("ghostty --title=cluster")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "ghostty")
}

func TestDetectProfile_Ghostty_FullPath(t *testing.T) {
	profile := DetectTerminalProfile("/usr/bin/ghostty")
	assert.IsType(t, &FlagProfile{}, profile)
	assert.Contains(t, profile.Name(), "ghostty")
}

func TestGhostty_BuildCommand(t *testing.T) {
	p := &FlagProfile{terminalName: "ghostty", flag: "-e"}
	termArgs := []string{"ghostty"}
	loginCmd := []string{"ocm-container", "-C", "abc123"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	expected := []string{"ghostty", "-e", "ocm-container", "-C", "abc123"}
	assert.Equal(t, expected, cmd)
}

// --- Redundant flatpak run prefix detection (Feature 2) ---

func TestRedundantFlatpakPrefix_Detected(t *testing.T) {
	// "flatpak run org.kde.konsole" should be detected as redundant
	// when the Flatpak app ID auto-detection feature exists.
	appID, detected := detectRedundantFlatpakPrefix("flatpak run org.kde.konsole")
	assert.True(t, detected)
	assert.Equal(t, "org.kde.konsole", appID)
}

func TestRedundantFlatpakPrefix_NotDetected_BareCommand(t *testing.T) {
	_, detected := detectRedundantFlatpakPrefix("konsole")
	assert.False(t, detected)
}

func TestRedundantFlatpakPrefix_NotDetected_BareAppID(t *testing.T) {
	_, detected := detectRedundantFlatpakPrefix("org.kde.konsole")
	assert.False(t, detected)
}

func TestRedundantFlatpakPrefix_WithExtraArgs(t *testing.T) {
	appID, detected := detectRedundantFlatpakPrefix("flatpak run org.gnome.Ptyxis --new-window")
	assert.True(t, detected)
	assert.Equal(t, "org.gnome.Ptyxis", appID)
}

func TestRedundantFlatpakPrefix_FlatpakSpawnNotDetected(t *testing.T) {
	// flatpak-spawn is a different use case (running from inside a container)
	// and should NOT be flagged as redundant.
	_, detected := detectRedundantFlatpakPrefix("flatpak-spawn --host flatpak run org.gnome.Ptyxis")
	assert.False(t, detected)
}

// --- Terminal validation (Feature 3) ---

func TestValidateTerminal_NotFound(t *testing.T) {
	// A terminal that does not exist should produce a warning message
	// but not an error.
	warning := validateTerminalExists("nonexistent-terminal-xyz-12345")
	assert.NotEmpty(t, warning, "expected a warning for a non-existent terminal")
}

func TestValidateTerminal_FlatpakAppID(t *testing.T) {
	// For a Flatpak app ID, validation should check for the "flatpak" binary
	// rather than the app ID itself.
	warning := validateTerminalExists("org.kde.konsole")
	// We cannot guarantee flatpak is installed in CI, so just verify it
	// returns something sensible (either empty if flatpak exists, or a
	// warning mentioning flatpak).
	if warning != "" {
		assert.Contains(t, warning, "flatpak")
	}
}

func TestValidateTerminal_FullPath_NotFound(t *testing.T) {
	warning := validateTerminalExists("/nonexistent/path/to/terminal")
	assert.NotEmpty(t, warning, "expected a warning for a non-existent path")
}

func TestValidateTerminal_EmptyString(t *testing.T) {
	warning := validateTerminalExists("")
	assert.NotEmpty(t, warning, "expected a warning for an empty terminal string")
}

// --- AppleScript terminal support (iTerm2, Terminal.app) ---

func TestDetectProfile_ITerm2(t *testing.T) {
	profile := DetectTerminalProfile("iterm2")
	assert.IsType(t, &AppleScriptProfile{}, profile)
	assert.Contains(t, profile.Name(), "iterm2")
}

func TestDetectProfile_ITerm2_MixedCase(t *testing.T) {
	profile := DetectTerminalProfile("iTerm2")
	assert.IsType(t, &AppleScriptProfile{}, profile)
	assert.Contains(t, profile.Name(), "iterm2")
}

func TestDetectProfile_TerminalApp(t *testing.T) {
	profile := DetectTerminalProfile("terminal")
	assert.IsType(t, &AppleScriptProfile{}, profile)
	assert.Contains(t, profile.Name(), "terminal")
}

func TestDetectProfile_TerminalApp_UpperCase(t *testing.T) {
	profile := DetectTerminalProfile("Terminal")
	assert.IsType(t, &AppleScriptProfile{}, profile)
	assert.Contains(t, profile.Name(), "terminal")
}

func TestAppleScriptProfile_Name_ITerm2(t *testing.T) {
	p := &AppleScriptProfile{terminalName: "iterm2", appName: "iTerm2"}
	assert.Equal(t, "applescript (iterm2)", p.Name())
}

func TestAppleScriptProfile_Name_Terminal(t *testing.T) {
	p := &AppleScriptProfile{terminalName: "terminal", appName: "Terminal"}
	assert.Equal(t, "applescript (terminal)", p.Name())
}

func TestAppleScriptProfile_BuildCommand_ITerm2(t *testing.T) {
	p := &AppleScriptProfile{terminalName: "iterm2", appName: "iTerm2"}
	// terminalArgs is just ["iterm2"] since the user sets terminal: iterm2
	termArgs := []string{"iterm2"}
	loginCmd := []string{"ocm-container", "-C", "abc123"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	// The profile should produce an osascript command that tells iTerm2
	// to create a window with the login command.
	assert.Equal(t, "osascript", cmd[0])
	assert.Equal(t, "-e", cmd[1])

	// The AppleScript should reference iTerm2 and contain the login command.
	script := cmd[2]
	assert.Contains(t, script, "iTerm2")
	assert.Contains(t, script, "ocm-container -C abc123")
}

func TestAppleScriptProfile_BuildCommand_TerminalApp(t *testing.T) {
	p := &AppleScriptProfile{terminalName: "terminal", appName: "Terminal"}
	termArgs := []string{"terminal"}
	loginCmd := []string{"ocm-container", "-C", "abc123"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	// Should produce osascript command for Terminal.app.
	assert.Equal(t, "osascript", cmd[0])
	assert.Equal(t, "-e", cmd[1])

	script := cmd[2]
	assert.Contains(t, script, "Terminal")
	assert.Contains(t, script, "ocm-container -C abc123")
}

func TestAppleScriptProfile_BuildCommand_EmptyTerminalArgs(t *testing.T) {
	p := &AppleScriptProfile{terminalName: "iterm2", appName: "iTerm2"}
	_, err := p.BuildCommand([]string{}, []string{"cmd"})
	assert.Error(t, err)
}

func TestAppleScriptProfile_BuildCommand_EmptyLoginCmd(t *testing.T) {
	p := &AppleScriptProfile{terminalName: "iterm2", appName: "iTerm2"}
	_, err := p.BuildCommand([]string{"iterm2"}, []string{})
	assert.Error(t, err)
}

func TestAppleScriptProfile_BuildCommand_ITerm2_QuotesInCommand(t *testing.T) {
	p := &AppleScriptProfile{terminalName: "iterm2", appName: "iTerm2"}
	termArgs := []string{"iterm2"}
	loginCmd := []string{"bash", "-c", "echo hello world"}

	cmd, err := p.BuildCommand(termArgs, loginCmd)
	require.NoError(t, err)

	assert.Equal(t, "osascript", cmd[0])
	script := cmd[2]
	assert.Contains(t, script, "bash -c echo hello world")
}

func TestAppleScriptProfile_ValidateTerminal_ITerm2(t *testing.T) {
	// "iterm2" is a virtual terminal name; validation should check for
	// "osascript" rather than "iterm2" itself.
	warning := validateTerminalExists("iterm2")
	// On macOS, osascript exists. On Linux CI, it will not, so just
	// verify it returns something sensible.
	if warning != "" {
		assert.Contains(t, warning, "osascript")
	}
}

func TestAppleScriptProfile_ValidateTerminal_Terminal(t *testing.T) {
	warning := validateTerminalExists("terminal")
	if warning != "" {
		assert.Contains(t, warning, "osascript")
	}
}
