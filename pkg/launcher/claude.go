package launcher

import (
	"os/exec"

	"github.com/charmbracelet/log"
)

// HasClaudeCode reports whether the "claude" binary is available on
// the system PATH.  It is safe to call unconditionally -- a missing
// binary simply returns false.
func HasClaudeCode() bool {
	return hasClaudeCodeWith(exec.LookPath)
}

// hasClaudeCodeWith is the testable core: it accepts a lookup function
// with the same signature as exec.LookPath so callers can inject a
// stub during tests.
func hasClaudeCodeWith(lookPath func(string) (string, error)) bool {
	path, err := lookPath("claude")
	if err != nil {
		log.Debug("launcher.HasClaudeCode: claude not found on PATH", "error", err)
		return false
	}
	log.Debug("launcher.HasClaudeCode: claude found", "path", path)
	return true
}
