package cmd

import (
	"testing"

	"github.com/coreos/go-systemd/journal"
	"github.com/stretchr/testify/assert"
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
