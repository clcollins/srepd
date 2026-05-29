package launcher

import (
	"fmt"
	"testing"
)

func TestHasClaudeCode_NotInstalled(t *testing.T) {
	// When exec.LookPath cannot find "claude", HasClaudeCode returns false
	lookPath := func(file string) (string, error) {
		return "", fmt.Errorf("executable file not found in $PATH")
	}

	got := hasClaudeCodeWith(lookPath)
	if got {
		t.Fatal("expected HasClaudeCode to return false when claude is not on PATH")
	}
}

func TestHasClaudeCode_Installed(t *testing.T) {
	// When exec.LookPath finds "claude", HasClaudeCode returns true
	lookPath := func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", fmt.Errorf("not found")
	}

	got := hasClaudeCodeWith(lookPath)
	if !got {
		t.Fatal("expected HasClaudeCode to return true when claude is on PATH")
	}
}
