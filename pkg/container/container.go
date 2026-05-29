package container

import (
	"os"

	"github.com/charmbracelet/log"
)

const defaultToolboxEnvPath = "/run/.toolboxenv"

// IsRunningInToolbox detects whether the current process is running inside
// a Fedora Toolbox container. It checks three indicators in priority order:
//  1. The /run/.toolboxenv file exists
//  2. The TOOLBOX_PATH environment variable is set
//  3. The "container" environment variable equals "toolbox"
func IsRunningInToolbox() bool {
	return checkToolbox(defaultToolboxEnvPath, os.Getenv)
}

// checkToolbox is the internal, testable implementation of the toolbox
// detection logic. It accepts the file path to check and a function for
// looking up environment variables, allowing tests to inject mocks.
func checkToolbox(toolboxEnvPath string, getenv func(string) string) bool {
	// Check 1: /run/.toolboxenv file exists
	if _, err := os.Stat(toolboxEnvPath); err == nil {
		log.Debug("container.checkToolbox", "detected", "toolboxenv file exists", "path", toolboxEnvPath)
		return true
	}

	// Check 2: TOOLBOX_PATH environment variable is set
	if getenv("TOOLBOX_PATH") != "" {
		log.Debug("container.checkToolbox", "detected", "TOOLBOX_PATH env var set")
		return true
	}

	// Check 3: container environment variable equals "toolbox"
	if getenv("container") == "toolbox" {
		log.Debug("container.checkToolbox", "detected", "container env var equals toolbox")
		return true
	}

	return false
}
