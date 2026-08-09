package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// shQuote
// ---------------------------------------------------------------------------

func TestShQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "'hello'",
		},
		{
			name:     "string with spaces",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it'\\''s'",
		},
		{
			name:     "string with double quote",
			input:    `say "hi"`,
			expected: `'say "hi"'`,
		},
		{
			name:     "string with dollar sign",
			input:    "$HOME",
			expected: "'$HOME'",
		},
		{
			name:     "string with backslash",
			input:    `path\to\file`,
			expected: `'path\to\file'`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "multiple single quotes",
			input:    "a'b'c",
			expected: "'a'\\''b'\\''c'",
		},
		{
			name:     "string with backtick",
			input:    "`cmd`",
			expected: "'`cmd`'",
		},
		{
			name:     "string with newline",
			input:    "line1\nline2",
			expected: "'line1\nline2'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, shQuote(tt.input))
		})
	}
}

// ---------------------------------------------------------------------------
// WriteWrapperScript
// ---------------------------------------------------------------------------

func TestWriteWrapperScript_BasicContent(t *testing.T) {
	tmpDir := t.TempDir()
	loginCmd := []string{"ocm", "backplane", "login", "abc123"}
	envPairs := []string{"PAGERDUTY_INCIDENT_ID=Q1ABC", "PAGERDUTY_CLUSTER_ID=abc123"}

	path, err := WriteWrapperScript(loginCmd, envPairs, tmpDir)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(path, tmpDir))
	assert.True(t, strings.HasSuffix(path, ".sh"))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	script := string(content)

	assert.True(t, strings.HasPrefix(script, "#!/bin/sh\n"))
	assert.Contains(t, script, "export PAGERDUTY_INCIDENT_ID='Q1ABC'")
	assert.Contains(t, script, "export PAGERDUTY_CLUSTER_ID='abc123'")
	assert.Contains(t, script, "exec 'ocm' 'backplane' 'login' 'abc123'")
}

func TestWriteWrapperScript_SpecialCharsInEnvValues(t *testing.T) {
	tmpDir := t.TempDir()
	loginCmd := []string{"ocm", "backplane", "login", "test"}
	envPairs := []string{
		"TITLE=it's a test",
		`URL=https://example.com?a=1&b="2"`,
		"DOLLAR=$HOME/path",
	}

	path, err := WriteWrapperScript(loginCmd, envPairs, tmpDir)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	script := string(content)

	assert.Contains(t, script, "export TITLE='it'\\''s a test'")
	assert.Contains(t, script, "export URL='https://example.com?a=1&b=\"2\"'")
	assert.Contains(t, script, "export DOLLAR='$HOME/path'")
}

func TestWriteWrapperScript_SpacesInArgv(t *testing.T) {
	tmpDir := t.TempDir()
	loginCmd := []string{"bash", "-c", "echo hello world"}
	envPairs := []string{"K=V"}

	path, err := WriteWrapperScript(loginCmd, envPairs, tmpDir)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	script := string(content)

	assert.Contains(t, script, "exec 'bash' '-c' 'echo hello world'")
}

func TestWriteWrapperScript_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	loginCmd := []string{"test"}
	envPairs := []string{"K=V"}

	path, err := WriteWrapperScript(loginCmd, envPairs, tmpDir)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), info.Mode().Perm(),
		"wrapper script must be executable only by owner")
}

func TestWriteWrapperScript_EmptyEnvPairs(t *testing.T) {
	tmpDir := t.TempDir()
	loginCmd := []string{"ocm", "backplane", "login", "abc"}

	path, err := WriteWrapperScript(loginCmd, []string{}, tmpDir)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	script := string(content)

	assert.NotContains(t, script, "export")
	assert.Contains(t, script, "exec 'ocm' 'backplane' 'login' 'abc'")
}

func TestWriteWrapperScript_EmptyLoginCmd(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := WriteWrapperScript([]string{}, []string{"K=V"}, tmpDir)
	assert.Error(t, err)
}

func TestWriteWrapperScript_CreatesSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	launchDir := filepath.Join(tmpDir, "srepd", "launch")

	loginCmd := []string{"test-cmd"}
	envPairs := []string{"K=V"}

	path, err := WriteWrapperScript(loginCmd, envPairs, launchDir)
	require.NoError(t, err)

	dirInfo, err := os.Stat(launchDir)
	require.NoError(t, err)
	assert.True(t, dirInfo.IsDir())
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(),
		"launch directory must be accessible only by owner")

	assert.True(t, strings.HasPrefix(path, launchDir))
}

// ---------------------------------------------------------------------------
// Regression: ocm-container + AppleScript with space-containing env values
// ---------------------------------------------------------------------------

func TestWriteWrapperScript_OCMContainerWithSpacesInEnvValues(t *testing.T) {
	tmpDir := t.TempDir()
	loginCmd := []string{"ocm-container", "-e", "PAGERDUTY_INCIDENT_TITLE=CPU high on node-42", "-C", "abc123"}
	envPairs := []string{
		"PAGERDUTY_INCIDENT_SERVICE=Production Web Service",
		"PAGERDUTY_ALERT_NAMES=NodeCPUHigh,NodeMemoryPressure",
	}

	path, err := WriteWrapperScript(loginCmd, envPairs, tmpDir)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	script := string(content)

	assert.Contains(t, script, "export PAGERDUTY_INCIDENT_SERVICE='Production Web Service'",
		"space-containing env value must be single-quoted to survive shell tokenization")
	assert.Contains(t, script, "'ocm-container'",
		"ocm-container must be quoted in exec line")
	assert.Contains(t, script, "'-e'",
		"-e flag must be a separate quoted arg")
	assert.Contains(t, script, "'PAGERDUTY_INCIDENT_TITLE=CPU high on node-42'",
		"space-containing -e value must be quoted to preserve it as one arg")
}

// ---------------------------------------------------------------------------
// CleanupWrapperScripts
// ---------------------------------------------------------------------------

func TestCleanupWrapperScripts_RemovesOldFiles(t *testing.T) {
	tmpDir := t.TempDir()

	oldFile := filepath.Join(tmpDir, "login-old.sh")
	err := os.WriteFile(oldFile, []byte("#!/bin/sh\n"), 0700)
	require.NoError(t, err)
	oldTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	newFile := filepath.Join(tmpDir, "login-new.sh")
	err = os.WriteFile(newFile, []byte("#!/bin/sh\n"), 0700)
	require.NoError(t, err)

	err = CleanupWrapperScripts(tmpDir, 1*time.Hour)
	require.NoError(t, err)

	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err), "old file should be removed")

	_, err = os.Stat(newFile)
	assert.NoError(t, err, "recent file should be kept")
}

func TestCleanupWrapperScripts_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	err := CleanupWrapperScripts(tmpDir, 1*time.Hour)
	assert.NoError(t, err)
}

func TestCleanupWrapperScripts_NonexistentDirectory(t *testing.T) {
	err := CleanupWrapperScripts("/tmp/nonexistent-srepd-test-dir-xyz", 1*time.Hour)
	assert.NoError(t, err, "should not error on nonexistent directory")
}

func TestCleanupWrapperScripts_OnlyRemovesShFiles(t *testing.T) {
	tmpDir := t.TempDir()

	shFile := filepath.Join(tmpDir, "login-old.sh")
	err := os.WriteFile(shFile, []byte("#!/bin/sh\n"), 0700)
	require.NoError(t, err)
	oldTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(shFile, oldTime, oldTime))

	otherFile := filepath.Join(tmpDir, "notes.txt")
	err = os.WriteFile(otherFile, []byte("keep me"), 0600)
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(otherFile, oldTime, oldTime))

	err = CleanupWrapperScripts(tmpDir, 1*time.Hour)
	require.NoError(t, err)

	_, err = os.Stat(shFile)
	assert.True(t, os.IsNotExist(err), "old .sh file should be removed")

	_, err = os.Stat(otherFile)
	assert.NoError(t, err, "non-.sh file should be kept even if old")
}
