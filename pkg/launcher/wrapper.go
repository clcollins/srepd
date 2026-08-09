package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// shQuote wraps a string in single quotes for safe use in a POSIX shell
// script. Single quotes inside the string are handled via the '\” idiom:
// close the current single-quoted segment, emit an escaped single quote,
// then reopen single quoting.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// WriteWrapperScript creates a short-lived shell script that exports
// environment variables and exec's the login command. This is used for
// AppleScript terminals where env vars cannot be passed via exec.Cmd.Env
// (AppleEvents don't carry environment) or -e flags (no ocm-container).
//
// The baseDir parameter specifies where to create the script (e.g.,
// ~/.cache/srepd/launch). The directory is created with 0700 permissions
// if it doesn't exist.
//
// Returns the path to the created script.
func WriteWrapperScript(loginCmd []string, envPairs []string, baseDir string) (string, error) {
	if len(loginCmd) == 0 {
		return "", fmt.Errorf("login command must not be empty")
	}

	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return "", fmt.Errorf("creating wrapper script directory: %w", err)
	}

	var b strings.Builder
	b.WriteString("#!/bin/sh\n")

	for _, pair := range envPairs {
		eqIdx := strings.Index(pair, "=")
		if eqIdx < 0 {
			continue
		}
		key := pair[:eqIdx]
		value := pair[eqIdx+1:]
		fmt.Fprintf(&b, "export %s=%s\n", key, shQuote(value))
	}

	b.WriteString("exec")
	for _, arg := range loginCmd {
		b.WriteString(" ")
		b.WriteString(shQuote(arg))
	}
	b.WriteString("\n")

	f, err := os.CreateTemp(baseDir, "login-*.sh")
	if err != nil {
		return "", fmt.Errorf("creating wrapper script: %w", err)
	}
	path := f.Name()
	success := false
	defer func() {
		_ = f.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()

	if _, err := f.WriteString(b.String()); err != nil {
		return "", fmt.Errorf("writing wrapper script: %w", err)
	}

	if err := f.Chmod(0700); err != nil {
		return "", fmt.Errorf("setting wrapper script permissions: %w", err)
	}

	success = true
	log.Debug("launcher.WriteWrapperScript(): created wrapper", "path", path)
	return path, nil
}

// CleanupWrapperScripts removes .sh files in baseDir older than maxAge.
// It is best-effort: non-existent directories and individual removal
// failures are logged but do not cause the function to return an error.
func CleanupWrapperScripts(baseDir string, maxAge time.Duration) error {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading wrapper script directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sh") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			log.Debug("launcher.CleanupWrapperScripts(): stat failed", "file", entry.Name(), "error", err)
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(baseDir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Debug("launcher.CleanupWrapperScripts(): remove failed", "path", path, "error", err)
			} else {
				log.Debug("launcher.CleanupWrapperScripts(): removed stale wrapper", "path", path)
			}
		}
	}

	return nil
}

// WrapperScriptDir returns the default directory for wrapper scripts.
func WrapperScriptDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("determining cache directory: %w", err)
	}
	return filepath.Join(cacheDir, "srepd", "launch"), nil
}
