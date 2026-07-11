package launcher

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func fakeLookPath(found ...string) func(string) (string, error) {
	set := make(map[string]bool)
	for _, f := range found {
		set[f] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", fmt.Errorf("%s not found", name)
	}
}

func fakeGetenv(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func names(dts []DetectedTerminal) []string {
	var out []string
	for _, dt := range dts {
		out = append(out, dt.Name)
	}
	return out
}

// OB-5: probe PATH for the known terminal profiles so the wizard can offer
// real choices instead of assuming gnome-terminal.
func TestDetectTerminals_ProbesKnownProfiles(t *testing.T) {
	dts := DetectTerminals(fakeLookPath("konsole", "kitty"), fakeGetenv(nil), "linux")

	assert.Equal(t, []string{"konsole", "kitty"}, names(dts))
	for _, dt := range dts {
		assert.NotEmpty(t, dt.Command)
	}
}

func TestDetectTerminals_NoneFound(t *testing.T) {
	dts := DetectTerminals(fakeLookPath(), fakeGetenv(nil), "linux")
	assert.Empty(t, dts)
}

// $TERM_PROGRAM identifies the terminal the user is sitting in — rank it first.
func TestDetectTerminals_TermProgramRanksFirst(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath("gnome-terminal", "wezterm"),
		fakeGetenv(map[string]string{"TERM_PROGRAM": "WezTerm"}),
		"linux",
	)
	assert.Equal(t, "wezterm", dts[0].Name, "the terminal the user is in ranks first")
}

// Inside tmux, offer tmux near the front (new windows land in the session).
func TestDetectTerminals_TmuxWhenInside(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath("tmux", "gnome-terminal"),
		fakeGetenv(map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0"}),
		"linux",
	)
	assert.Contains(t, names(dts), "tmux")
	assert.Equal(t, "tmux", dts[0].Name)
}

func TestDetectTerminals_TmuxOnlyWhenInside(t *testing.T) {
	dts := DetectTerminals(fakeLookPath("tmux", "gnome-terminal"), fakeGetenv(nil), "linux")
	assert.NotContains(t, names(dts), "tmux",
		"tmux outside a session cannot open a window — do not offer it")
}

// macOS: Terminal.app and iTerm2 are launched via osascript, not PATH — they
// are always candidates on darwin.
func TestDetectTerminals_DarwinIncludesAppleTerminals(t *testing.T) {
	dts := DetectTerminals(fakeLookPath(), fakeGetenv(nil), "darwin")
	assert.Contains(t, names(dts), "terminal")
	assert.Contains(t, names(dts), "iterm2")
}

func TestDetectTerminals_DarwinTermProgramRanksITermFirst(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath(),
		fakeGetenv(map[string]string{"TERM_PROGRAM": "iTerm.app"}),
		"darwin",
	)
	assert.Equal(t, "iterm2", dts[0].Name)
}

func TestDetectTerminals_LinuxExcludesAppleTerminals(t *testing.T) {
	dts := DetectTerminals(fakeLookPath("gnome-terminal"), fakeGetenv(nil), "linux")
	assert.NotContains(t, names(dts), "terminal")
	assert.NotContains(t, names(dts), "iterm2")
}
