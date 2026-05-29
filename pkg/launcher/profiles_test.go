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
