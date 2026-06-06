package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coreos/go-systemd/journal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetermineLogDestination(t *testing.T) {
	tests := []struct {
		name           string
		goos           string
		logToJournal   bool
		journalEnabled bool
		expectedDest   LogDestination
		expectedPath   string
	}{
		{
			name:           "Linux with journal enabled and log_to_journal=true",
			goos:           "linux",
			logToJournal:   true,
			journalEnabled: true,
			expectedDest:   LogToJournal,
			expectedPath:   "",
		},
		{
			name:           "Linux with journal disabled and log_to_journal=true",
			goos:           "linux",
			logToJournal:   true,
			journalEnabled: false,
			expectedDest:   LogToFile,
			expectedPath:   "/var/log/srepd.log",
		},
		{
			name:           "Linux with log_to_journal=false (user wants file logging)",
			goos:           "linux",
			logToJournal:   false,
			journalEnabled: true, // Journal enabled but user wants file
			expectedDest:   LogToFile,
			expectedPath:   "~/.config/srepd/debug.log",
		},
		{
			name:           "Linux with log_to_journal=false and journal disabled",
			goos:           "linux",
			logToJournal:   false,
			journalEnabled: false,
			expectedDest:   LogToFile,
			expectedPath:   "~/.config/srepd/debug.log",
		},
		{
			name:           "macOS always logs to file",
			goos:           "darwin",
			logToJournal:   true, // Ignored on macOS
			journalEnabled: false,
			expectedDest:   LogToFile,
			expectedPath:   "~/Library/Logs/srepd.log",
		},
		{
			name:           "macOS with log_to_journal=false",
			goos:           "darwin",
			logToJournal:   false,
			journalEnabled: false,
			expectedDest:   LogToFile,
			expectedPath:   "~/Library/Logs/srepd.log",
		},
		{
			name:           "Unsupported OS logs to stderr",
			goos:           "windows",
			logToJournal:   true,
			journalEnabled: false,
			expectedDest:   LogToStderr,
			expectedPath:   "",
		},
		{
			name:           "Unknown OS logs to stderr",
			goos:           "freebsd",
			logToJournal:   false,
			journalEnabled: false,
			expectedDest:   LogToStderr,
			expectedPath:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest, path := determineLogDestination(tt.goos, tt.logToJournal, tt.journalEnabled)
			assert.Equal(t, tt.expectedDest, dest, "Log destination mismatch")
			assert.Equal(t, tt.expectedPath, path, "Log path mismatch")
		})
	}
}

func TestSyslogIdentifier(t *testing.T) {
	assert.Equal(t, "srepd", syslogIdentifier, "SYSLOG_IDENTIFIER must be 'srepd' for journalctl -t srepd")
}

func TestJournalPriorityMapping(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected journal.Priority
	}{
		{
			name:     "ERROR maps to PriErr",
			message:  "ERROR something went wrong",
			expected: journal.PriErr,
		},
		{
			name:     "ERRO maps to PriErr",
			message:  "ERRO short form error",
			expected: journal.PriErr,
		},
		{
			name:     "lowercase error maps to PriErr",
			message:  "an error occurred in the system",
			expected: journal.PriErr,
		},
		{
			name:     "WARN maps to PriWarning",
			message:  "WARN something might be wrong",
			expected: journal.PriWarning,
		},
		{
			name:     "WARNING maps to PriWarning",
			message:  "WARNING elevated concern",
			expected: journal.PriWarning,
		},
		{
			name:     "lowercase warn maps to PriWarning",
			message:  "a warning was issued",
			expected: journal.PriWarning,
		},
		{
			name:     "DEBUG maps to PriDebug",
			message:  "DEBUG verbose output here",
			expected: journal.PriDebug,
		},
		{
			name:     "lowercase debug maps to PriDebug",
			message:  "debug information available",
			expected: journal.PriDebug,
		},
		{
			name:     "INFO maps to PriInfo",
			message:  "INFO normal operation",
			expected: journal.PriInfo,
		},
		{
			name:     "plain message defaults to PriInfo",
			message:  "just a regular log line",
			expected: journal.PriInfo,
		},
		{
			name:     "empty message defaults to PriInfo",
			message:  "",
			expected: journal.PriInfo,
		},
		{
			name:     "error takes precedence over warn when both present",
			message:  "ERROR warning about something",
			expected: journal.PriErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority := journalPriority(tt.message)
			assert.Equal(t, tt.expected, priority, "Priority mismatch for message: %s", tt.message)
		})
	}
}

// saveAndRestoreLogWriter saves the current logWriter and restores it after the test.
func saveAndRestoreLogWriter(t *testing.T) {
	t.Helper()
	original := logWriter
	t.Cleanup(func() {
		logWriter = original
	})
}

func TestSetupFileLogging_CreatesFileAndSetsLogWriter(t *testing.T) {
	saveAndRestoreLogWriter(t)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// logWriter should be nil or restored value before calling
	logWriter = nil

	setupFileLogging(logPath)

	assert.NotNil(t, logWriter, "logWriter should be set after setupFileLogging")

	// Write something through the logWriter to verify it works
	msg := []byte("test log message\n")
	n, err := logWriter.Write(msg)
	require.NoError(t, err)
	assert.Equal(t, len(msg), n, "Write should return the number of bytes written")

	// Close the writer so we can read the file
	err = logWriter.Close()
	require.NoError(t, err)

	// Verify the file was created and contains the message
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test log message", "log file should contain the written message")
}

func TestSetupFileLogging_AppendsToExistingFile(t *testing.T) {
	saveAndRestoreLogWriter(t)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Pre-create the file with existing content
	err := os.WriteFile(logPath, []byte("existing content\n"), 0644)
	require.NoError(t, err)

	logWriter = nil
	setupFileLogging(logPath)

	msg := []byte("appended message\n")
	_, err = logWriter.Write(msg)
	require.NoError(t, err)

	err = logWriter.Close()
	require.NoError(t, err)

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "existing content", "file should retain existing content")
	assert.Contains(t, string(content), "appended message", "file should contain appended message")
}

func TestSetupFileLogging_FilePermissions(t *testing.T) {
	saveAndRestoreLogWriter(t)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logWriter = nil
	setupFileLogging(logPath)

	err := logWriter.Close()
	require.NoError(t, err)

	info, err := os.Stat(logPath)
	require.NoError(t, err)
	// File should be created with 0644 permissions (may be affected by umask)
	perm := info.Mode().Perm()
	assert.True(t, perm&0600 == 0600, "file should be readable and writable by owner, got %o", perm)
}

func TestCleanupLogging_WithNonNilLogWriter(t *testing.T) {
	saveAndRestoreLogWriter(t)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logWriter = nil
	setupFileLogging(logPath)
	require.NotNil(t, logWriter, "logWriter should be set before CleanupLogging")

	// Write a message to verify the writer is functional
	_, err := logWriter.Write([]byte("before cleanup\n"))
	require.NoError(t, err)

	// CleanupLogging should close the writer without panicking
	CleanupLogging()

	// After cleanup, writing should fail with ErrClosed
	_, err = logWriter.Write([]byte("after cleanup\n"))
	assert.ErrorIs(t, err, os.ErrClosed, "writing after CleanupLogging should return ErrClosed")
}

func TestCleanupLogging_WithNilLogWriter(t *testing.T) {
	saveAndRestoreLogWriter(t)

	logWriter = nil

	// Should not panic when logWriter is nil
	assert.NotPanics(t, func() {
		CleanupLogging()
	}, "CleanupLogging should not panic when logWriter is nil")
}

func TestCleanupLogging_CalledTwice(t *testing.T) {
	saveAndRestoreLogWriter(t)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	logWriter = nil
	setupFileLogging(logPath)

	// Double cleanup should not panic
	assert.NotPanics(t, func() {
		CleanupLogging()
		CleanupLogging()
	}, "calling CleanupLogging twice should not panic")
}

func TestBindArgsToViper(t *testing.T) {
	tests := []struct {
		name         string
		debugFlag    string
		devFlag      string
		fixturesFlag string
		expectDebug  bool
		expectDev    bool
		expectFixDir string
	}{
		{
			name:         "all flags set to non-default values",
			debugFlag:    "true",
			devFlag:      "true",
			fixturesFlag: "/custom/fixtures",
			expectDebug:  true,
			expectDev:    true,
			expectFixDir: "/custom/fixtures",
		},
		{
			name:         "all flags at defaults",
			debugFlag:    "false",
			devFlag:      "false",
			fixturesFlag: "testdata/fixtures",
			expectDebug:  false,
			expectDev:    false,
			expectFixDir: "testdata/fixtures",
		},
		{
			name:         "only debug enabled",
			debugFlag:    "true",
			devFlag:      "false",
			fixturesFlag: "testdata/fixtures",
			expectDebug:  true,
			expectDev:    false,
			expectFixDir: "testdata/fixtures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(func() { viper.Reset() })

			cmd := &cobra.Command{Use: "test"}
			cmd.PersistentFlags().BoolP("debug", "d", false, "enable debug logging")
			cmd.PersistentFlags().BoolP("dev", "D", false, "enable dev mode")
			cmd.PersistentFlags().StringP("fixtures-dir", "F", "testdata/fixtures", "path to fixture data")
			err := cmd.PersistentFlags().Set("debug", tt.debugFlag)
			require.NoError(t, err)
			err = cmd.PersistentFlags().Set("dev", tt.devFlag)
			require.NoError(t, err)
			err = cmd.PersistentFlags().Set("fixtures-dir", tt.fixturesFlag)
			require.NoError(t, err)

			bindArgsToViper(cmd)

			assert.Equal(t, tt.expectDebug, viper.GetBool("debug"), "debug flag mismatch")
			assert.Equal(t, tt.expectDev, viper.GetBool("dev"), "dev flag mismatch")
			assert.Equal(t, tt.expectFixDir, viper.GetString("fixtures_dir"), "fixtures_dir mismatch")
		})
	}
}

func TestBindArgsToViper_FlagNotSet_UsesDefault(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().BoolP("debug", "d", false, "enable debug logging")
	cmd.PersistentFlags().BoolP("dev", "D", false, "enable dev mode")
	cmd.PersistentFlags().StringP("fixtures-dir", "F", "testdata/fixtures", "path to fixture data")
	// Do not set any flags -- they should retain their defaults
	bindArgsToViper(cmd)

	assert.Equal(t, false, viper.GetBool("debug"), "debug should default to false")
	assert.Equal(t, false, viper.GetBool("dev"), "dev should default to false")
	assert.Equal(t, "testdata/fixtures", viper.GetString("fixtures_dir"), "fixtures_dir should default to testdata/fixtures")
}

func TestConfigureLogging_SetsLogWriter(t *testing.T) {
	saveAndRestoreLogWriter(t)

	// Reset viper to avoid interference from other tests
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	logWriter = nil

	// configureLogging reads runtime.GOOS and journal.Enabled() internally.
	// On Linux with journal available, it will set up journal logging.
	// On Linux without journal, it will set up file logging.
	// Either way, logWriter should be non-nil afterward.
	configureLogging()

	assert.NotNil(t, logWriter, "configureLogging should set the logWriter")

	// Clean up the writer
	err := logWriter.Close()
	assert.NoError(t, err)
}

func TestJournalWriter_Write(t *testing.T) {
	if !journal.Enabled() {
		t.Skip("systemd journal not available")
	}

	jw := journalWriter{}

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "info message",
			message: "INFO test journal write",
		},
		{
			name:    "error message",
			message: "ERROR something failed",
		},
		{
			name:    "empty message",
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := jw.Write([]byte(tt.message))
			assert.NoError(t, err, "journalWriter.Write should not error")
			assert.Equal(t, len(tt.message), n, "journalWriter.Write should return the message length")
		})
	}
}

func TestJournalWriter_WriteReturnsCorrectLength(t *testing.T) {
	if !journal.Enabled() {
		t.Skip("systemd journal not available")
	}

	jw := journalWriter{}

	// Test with various message sizes
	messages := []string{
		"short",
		"a medium length log message with some details about what happened",
		"WARN this is a warning message that should map to PriWarning",
		"DEBUG verbose output with lots of detail about internal state",
	}

	for _, msg := range messages {
		n, err := jw.Write([]byte(msg))
		assert.NoError(t, err)
		assert.Equal(t, len(msg), n, "Write should return len of input for message: %s", msg)
	}
}
