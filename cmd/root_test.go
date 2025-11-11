package cmd

import (
	"testing"

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
