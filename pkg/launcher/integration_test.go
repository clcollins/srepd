//go:build integration

package launcher

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Terminal binary existence tests ---
//
// These tests verify that when a terminal binary is installed on the
// system, exec.LookPath finds it and it can report a version or help
// flag without error. When the binary is absent, t.Skip is called so
// the test is silently skipped rather than failing.

// terminalBinary describes a terminal emulator binary that srepd
// supports, along with a safe flag that can be used to verify the
// binary is callable without actually opening a window.
type terminalBinary struct {
	name     string   // executable name as used in srepd config
	safeArgs []string // args that produce output without opening a window
	platform string   // "linux", "darwin", or "" for both
}

// supportedTerminals is the authoritative list of terminals srepd
// supports, matching the maps in profiles.go.
var supportedTerminals = []terminalBinary{
	{name: "gnome-terminal", safeArgs: []string{"--help"}, platform: "linux"},
	{name: "ptyxis", safeArgs: []string{"--help"}, platform: "linux"},
	{name: "konsole", safeArgs: []string{"--help"}, platform: "linux"},
	{name: "kitty", safeArgs: []string{"--version"}, platform: ""},
	{name: "alacritty", safeArgs: []string{"--version"}, platform: ""},
	{name: "wezterm", safeArgs: []string{"--help"}, platform: ""},
	{name: "blackbox", safeArgs: []string{"--help"}, platform: "linux"},
	{name: "foot", safeArgs: []string{"--version"}, platform: "linux"},
	{name: "ghostty", safeArgs: []string{"--help"}, platform: ""},
	{name: "tmux", safeArgs: []string{"-V"}, platform: ""},
	{name: "terminator", safeArgs: []string{"--help"}, platform: "linux"},
	{name: "contour", safeArgs: []string{"--version"}, platform: "linux"},
}

func TestIntegration_TerminalBinaryExists(t *testing.T) {
	for _, term := range supportedTerminals {
		t.Run(term.name, func(t *testing.T) {
			if term.platform != "" && term.platform != runtime.GOOS {
				t.Skipf("%s is %s-only, running on %s", term.name, term.platform, runtime.GOOS)
			}

			path, err := exec.LookPath(term.name)
			if err != nil {
				t.Skipf("%s not installed: %v", term.name, err)
			}

			assert.NotEmpty(t, path, "LookPath returned empty path for %s", term.name)

			// Verify the binary is a real file (not dangling symlink).
			info, err := os.Stat(path)
			require.NoError(t, err, "os.Stat failed for %s at %s", term.name, path)
			assert.False(t, info.IsDir(), "expected %s to be a file, not a directory", path)
		})
	}
}

// TestIntegration_TerminalBinaryCallable verifies that each installed
// terminal binary can be invoked with a safe flag (--help, --version)
// and produces some output. This catches cases where a binary exists
// but is broken (missing shared libraries, corrupt install, etc.).
func TestIntegration_TerminalBinaryCallable(t *testing.T) {
	for _, term := range supportedTerminals {
		t.Run(term.name, func(t *testing.T) {
			if term.platform != "" && term.platform != runtime.GOOS {
				t.Skipf("%s is %s-only, running on %s", term.name, term.platform, runtime.GOOS)
			}

			_, err := exec.LookPath(term.name)
			if err != nil {
				t.Skipf("%s not installed: %v", term.name, err)
			}

			cmd := exec.Command(term.name, term.safeArgs...)
			output, _ := cmd.CombinedOutput()

			// Some terminals exit 0 on --help, others exit non-zero.
			// We only care that they produced output (proving the binary
			// actually runs). A truly broken binary would fail to start
			// at all and produce no output.
			assert.NotEmpty(t, output,
				"%s %v produced no output; binary may be broken",
				term.name, term.safeArgs)
		})
	}
}

// --- Profile detection against actual binary names ---
//
// These tests verify that DetectTerminalProfile returns the correct
// profile type for every binary name in the profile maps (profiles.go).
// This ensures the maps stay in sync with the profile detection logic.

func TestIntegration_ProfileDetection_SeparatorTerminals(t *testing.T) {
	for name := range separatorTerminals {
		t.Run(name, func(t *testing.T) {
			profile := DetectTerminalProfile(name)
			assert.IsType(t, &SeparatorProfile{}, profile,
				"expected SeparatorProfile for %s, got %s", name, profile.Name())

			sepProfile, ok := profile.(*SeparatorProfile)
			require.True(t, ok)
			assert.Equal(t, name, sepProfile.terminalName,
				"SeparatorProfile.terminalName mismatch for %s", name)
		})
	}
}

func TestIntegration_ProfileDetection_FlagTerminals(t *testing.T) {
	for name, expectedFlag := range flagTerminals {
		t.Run(name, func(t *testing.T) {
			profile := DetectTerminalProfile(name)
			assert.IsType(t, &FlagProfile{}, profile,
				"expected FlagProfile for %s, got %s", name, profile.Name())

			flagProfile, ok := profile.(*FlagProfile)
			require.True(t, ok)
			assert.Equal(t, name, flagProfile.terminalName,
				"FlagProfile.terminalName mismatch for %s", name)
			assert.Equal(t, expectedFlag, flagProfile.flag,
				"FlagProfile.flag mismatch for %s", name)
		})
	}
}

func TestIntegration_ProfileDetection_DirectTerminals(t *testing.T) {
	for name := range directTerminals {
		t.Run(name, func(t *testing.T) {
			profile := DetectTerminalProfile(name)
			assert.IsType(t, &DirectProfile{}, profile,
				"expected DirectProfile for %s, got %s", name, profile.Name())

			dirProfile, ok := profile.(*DirectProfile)
			require.True(t, ok)
			assert.Equal(t, name, dirProfile.terminalName,
				"DirectProfile.terminalName mismatch for %s", name)
		})
	}
}

func TestIntegration_ProfileDetection_AppleScriptTerminals(t *testing.T) {
	for name, expectedAppName := range appleScriptTerminals {
		t.Run(name, func(t *testing.T) {
			profile := DetectTerminalProfile(name)
			assert.IsType(t, &AppleScriptProfile{}, profile,
				"expected AppleScriptProfile for %s, got %s", name, profile.Name())

			asProfile, ok := profile.(*AppleScriptProfile)
			require.True(t, ok)
			assert.Equal(t, name, asProfile.terminalName,
				"AppleScriptProfile.terminalName mismatch for %s", name)
			assert.Equal(t, expectedAppName, asProfile.appName,
				"AppleScriptProfile.appName mismatch for %s", name)
		})
	}
}

func TestIntegration_ProfileDetection_FlatpakAppIDs(t *testing.T) {
	for appID, expectedExecName := range flatpakAppTerminals {
		t.Run(appID, func(t *testing.T) {
			profile := DetectTerminalProfile(appID)

			// The profile should match the mapped executable name's
			// profile type, not GenericProfile.
			expectedProfile := profileForExecName(expectedExecName)
			assert.IsType(t, expectedProfile, profile,
				"Flatpak app ID %s (mapped to %s) should produce %T, got %T",
				appID, expectedExecName, expectedProfile, profile)
		})
	}
}

// --- Validate terminal exists (integration with real PATH) ---
//
// These tests call validateTerminalExists with real terminal names and
// verify the result is consistent with exec.LookPath.

func TestIntegration_ValidateTerminalExists_Consistency(t *testing.T) {
	for _, term := range supportedTerminals {
		t.Run(term.name, func(t *testing.T) {
			if term.platform != "" && term.platform != runtime.GOOS {
				t.Skipf("%s is %s-only, running on %s", term.name, term.platform, runtime.GOOS)
			}

			warning := validateTerminalExists(term.name)
			_, lookErr := exec.LookPath(term.name)

			if lookErr != nil {
				// Terminal not installed: validateTerminalExists should
				// return a non-empty warning.
				assert.NotEmpty(t, warning,
					"validateTerminalExists returned empty warning for missing terminal %s", term.name)
			} else {
				// Terminal installed: validateTerminalExists should
				// return an empty string (no warning).
				assert.Empty(t, warning,
					"validateTerminalExists returned warning for installed terminal %s: %s", term.name, warning)
			}
		})
	}
}

// TestIntegration_ValidateTerminalExists_Flatpak verifies that
// validateTerminalExists checks for the "flatpak" binary when given a
// Flatpak app ID, and produces a result consistent with flatpak
// availability on the system.
func TestIntegration_ValidateTerminalExists_Flatpak(t *testing.T) {
	for appID := range flatpakAppTerminals {
		t.Run(appID, func(t *testing.T) {
			warning := validateTerminalExists(appID)
			_, flatpakErr := exec.LookPath("flatpak")

			if flatpakErr != nil {
				assert.NotEmpty(t, warning,
					"expected warning for Flatpak app ID %s when flatpak is not installed", appID)
				assert.Contains(t, warning, "flatpak",
					"warning should mention flatpak for app ID %s", appID)
			} else {
				assert.Empty(t, warning,
					"expected no warning for Flatpak app ID %s when flatpak is installed: %s", appID, warning)
			}
		})
	}
}

// --- Terminal list completeness ---
//
// These tests verify that the supportedTerminals test list covers all
// entries in the profile maps, catching cases where a new terminal is
// added to profiles.go but not to the integration test list.

func TestIntegration_SupportedTerminals_CoversAllSeparatorTerminals(t *testing.T) {
	for name := range separatorTerminals {
		found := false
		for _, term := range supportedTerminals {
			if term.name == name {
				found = true
				break
			}
		}
		assert.True(t, found,
			"separator terminal %q is in profiles.go but missing from supportedTerminals test list", name)
	}
}

func TestIntegration_SupportedTerminals_CoversAllFlagTerminals(t *testing.T) {
	for name := range flagTerminals {
		found := false
		for _, term := range supportedTerminals {
			if term.name == name {
				found = true
				break
			}
		}
		assert.True(t, found,
			"flag terminal %q is in profiles.go but missing from supportedTerminals test list", name)
	}
}

func TestIntegration_SupportedTerminals_CoversAllDirectTerminals(t *testing.T) {
	for name := range directTerminals {
		found := false
		for _, term := range supportedTerminals {
			if term.name == name {
				found = true
				break
			}
		}
		assert.True(t, found,
			"direct terminal %q is in profiles.go but missing from supportedTerminals test list", name)
	}
}
