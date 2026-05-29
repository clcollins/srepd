package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRunningInToolbox_FileExists(t *testing.T) {
	// Create a temporary file to simulate /run/.toolboxenv
	tmpDir := t.TempDir()
	toolboxEnvFile := filepath.Join(tmpDir, ".toolboxenv")
	err := os.WriteFile(toolboxEnvFile, []byte(""), 0644)
	assert.NoError(t, err, "failed to create temp .toolboxenv file")

	// Use the internal function with injected dependencies
	result := checkToolbox(
		toolboxEnvFile,
		func(key string) string { return "" },
	)
	assert.True(t, result, "should detect toolbox when .toolboxenv file exists")
}

func TestIsRunningInToolbox_ToolboxPathEnvVar(t *testing.T) {
	// No file exists, but TOOLBOX_PATH is set
	result := checkToolbox(
		"/nonexistent/path/.toolboxenv",
		func(key string) string {
			if key == "TOOLBOX_PATH" {
				return "/usr/bin/toolbox"
			}
			return ""
		},
	)
	assert.True(t, result, "should detect toolbox when TOOLBOX_PATH env var is set")
}

func TestIsRunningInToolbox_ContainerEnvVar(t *testing.T) {
	// No file exists, TOOLBOX_PATH is not set, but container=toolbox
	result := checkToolbox(
		"/nonexistent/path/.toolboxenv",
		func(key string) string {
			if key == "container" {
				return "toolbox"
			}
			return ""
		},
	)
	assert.True(t, result, "should detect toolbox when container env var equals 'toolbox'")
}

func TestIsRunningInToolbox_ContainerEnvVarWrongValue(t *testing.T) {
	// No file, no TOOLBOX_PATH, but container is set to something other than "toolbox"
	result := checkToolbox(
		"/nonexistent/path/.toolboxenv",
		func(key string) string {
			if key == "container" {
				return "podman"
			}
			return ""
		},
	)
	assert.False(t, result, "should not detect toolbox when container env var is not 'toolbox'")
}

func TestIsRunningInToolbox_NotInToolbox(t *testing.T) {
	// None of the detection methods match
	result := checkToolbox(
		"/nonexistent/path/.toolboxenv",
		func(key string) string { return "" },
	)
	assert.False(t, result, "should not detect toolbox when no indicators are present")
}

func TestIsRunningInToolbox_PriorityOrder(t *testing.T) {
	// When file exists, env vars should not matter (file check is first)
	tmpDir := t.TempDir()
	toolboxEnvFile := filepath.Join(tmpDir, ".toolboxenv")
	err := os.WriteFile(toolboxEnvFile, []byte(""), 0644)
	assert.NoError(t, err)

	envCalled := false
	result := checkToolbox(
		toolboxEnvFile,
		func(key string) string {
			envCalled = true
			return ""
		},
	)
	assert.True(t, result, "should detect toolbox from file")
	assert.False(t, envCalled, "env lookup should not be called when file check succeeds")
}
