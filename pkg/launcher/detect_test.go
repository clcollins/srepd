package launcher

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeFileInfo struct {
	name string
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0755 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return true }
func (f fakeFileInfo) Sys() interface{}   { return nil }

func fakeStat(existing ...string) func(string) (os.FileInfo, error) {
	set := make(map[string]bool)
	for _, e := range existing {
		set[e] = true
	}
	return func(path string) (os.FileInfo, error) {
		if set[path] {
			return fakeFileInfo{name: path}, nil
		}
		return nil, fmt.Errorf("%s not found", path)
	}
}

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
	dts := DetectTerminals(fakeLookPath("konsole", "kitty"), fakeGetenv(nil), "linux", fakeStat())

	assert.Equal(t, []string{"konsole", "kitty"}, names(dts))
	for _, dt := range dts {
		assert.NotEmpty(t, dt.Command)
	}
}

func TestDetectTerminals_NoneFound(t *testing.T) {
	dts := DetectTerminals(fakeLookPath(), fakeGetenv(nil), "linux", fakeStat())
	assert.Empty(t, dts)
}

// $TERM_PROGRAM identifies the terminal the user is sitting in — rank it first.
func TestDetectTerminals_TermProgramRanksFirst(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath("gnome-terminal", "wezterm"),
		fakeGetenv(map[string]string{"TERM_PROGRAM": "WezTerm"}),
		"linux",
		fakeStat(),
	)
	assert.Equal(t, "wezterm", dts[0].Name, "the terminal the user is in ranks first")
}

// Inside tmux, offer tmux near the front (new windows land in the session).
func TestDetectTerminals_TmuxWhenInside(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath("tmux", "gnome-terminal"),
		fakeGetenv(map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0"}),
		"linux",
		fakeStat(),
	)
	assert.Contains(t, names(dts), "tmux")
	assert.Equal(t, "tmux", dts[0].Name)
}

func TestDetectTerminals_TmuxOnlyWhenInside(t *testing.T) {
	dts := DetectTerminals(fakeLookPath("tmux", "gnome-terminal"), fakeGetenv(nil), "linux", fakeStat())
	assert.NotContains(t, names(dts), "tmux",
		"tmux outside a session cannot open a window — do not offer it")
}

// macOS: Terminal.app is always a candidate on darwin; iTerm2 only when installed.
func TestDetectTerminals_DarwinIncludesTerminal(t *testing.T) {
	dts := DetectTerminals(fakeLookPath(), fakeGetenv(nil), "darwin", fakeStat())
	assert.Contains(t, names(dts), "terminal")
}

func TestDetectTerminals_DarwinITerm2OnlyWhenInstalled(t *testing.T) {
	dts := DetectTerminals(fakeLookPath(), fakeGetenv(nil), "darwin", fakeStat())
	assert.NotContains(t, names(dts), "iterm2",
		"iTerm2 should not appear when /Applications/iTerm.app does not exist")

	dts = DetectTerminals(fakeLookPath(), fakeGetenv(nil), "darwin",
		fakeStat("/Applications/iTerm.app"))
	assert.Contains(t, names(dts), "iterm2",
		"iTerm2 should appear when /Applications/iTerm.app exists")
}

func TestDetectTerminals_DarwinTermProgramRanksITermFirst(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath(),
		fakeGetenv(map[string]string{"TERM_PROGRAM": "iTerm.app"}),
		"darwin",
		fakeStat("/Applications/iTerm.app"),
	)
	assert.Equal(t, "iterm2", dts[0].Name)
}

func TestDetectTerminals_LinuxExcludesAppleTerminals(t *testing.T) {
	dts := DetectTerminals(fakeLookPath("gnome-terminal"), fakeGetenv(nil), "linux", fakeStat())
	assert.NotContains(t, names(dts), "terminal")
	assert.NotContains(t, names(dts), "iterm2")
}

func TestDetectTerminals_DarwinBundleDetection(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath(),
		fakeGetenv(nil),
		"darwin",
		fakeStat("/Applications/kitty.app", "/Applications/Ghostty.app"),
	)
	n := names(dts)
	assert.Contains(t, n, "kitty", "kitty should be detected from bundle")
	assert.Contains(t, n, "ghostty", "ghostty should be detected from bundle")
	assert.NotContains(t, n, "alacritty", "alacritty bundle doesn't exist")
	assert.NotContains(t, n, "wezterm", "wezterm bundle doesn't exist")
}

func TestDetectTerminals_DarwinBundleSkippedOnLinux(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath(),
		fakeGetenv(nil),
		"linux",
		fakeStat("/Applications/kitty.app"),
	)
	assert.NotContains(t, names(dts), "kitty",
		"bundle detection should not run on linux")
}

func TestDetectTerminals_PATHTakesPrecedenceOverBundle(t *testing.T) {
	dts := DetectTerminals(
		fakeLookPath("kitty"),
		fakeGetenv(nil),
		"darwin",
		fakeStat("/Applications/kitty.app"),
	)
	n := names(dts)
	// kitty should appear exactly once (from PATH, not duplicated by bundle)
	count := 0
	for _, name := range n {
		if name == "kitty" {
			count++
		}
	}
	assert.Equal(t, 1, count, "kitty should appear exactly once when found via both PATH and bundle")
}
