package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setValidConfig populates viper with a complete, valid configuration
// for use in tests. Each test should call viper.Reset() first, then
// optionally call this to set up the baseline valid state.
func setValidConfig() {
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1", "TEAM2"})
	viper.Set("service_escalation_policies", map[string]interface{}{
		"default":        "ESCALATION_POLICY_1",
		"silent_default": "ESCALATION_POLICY_2",
		"SERVICE_1":      "ESCALATION_POLICY_3",
	})
}

func TestValidateConfig_AllRequiredKeys(t *testing.T) {
	viper.Reset()
	setValidConfig()

	err := validateConfig()

	assert.NoError(t, err, "validateConfig should return no error when all required keys are set")
}

func TestValidateConfig_MissingToken(t *testing.T) {
	viper.Reset()
	setValidConfig()
	// Remove the token key by resetting and re-adding everything except token
	viper.Reset()
	viper.Set("teams", []string{"TEAM1", "TEAM2"})
	viper.Set("service_escalation_policies", map[string]interface{}{
		"default":        "ESCALATION_POLICY_1",
		"silent_default": "ESCALATION_POLICY_2",
	})

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return an error when token is missing")
	assert.Contains(t, err.Error(), "token", "error should mention the missing 'token' key")
}

func TestValidateConfig_MissingTeams(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("service_escalation_policies", map[string]interface{}{
		"default":        "ESCALATION_POLICY_1",
		"silent_default": "ESCALATION_POLICY_2",
	})

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return an error when teams is missing")
	assert.Contains(t, err.Error(), "teams", "error should mention the missing 'teams' key")
}

func TestValidateConfig_NoEscalationPoliciesIsValid(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1", "TEAM2"})

	err := validateConfig()

	assert.NoError(t, err, "validateConfig should not error when service_escalation_policies is absent (deprecated)")
}

func TestValidateConfig_MissingDefaultPolicy(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1", "TEAM2"})
	viper.Set("service_escalation_policies", map[string]interface{}{
		"silent_default": "ESCALATION_POLICY_2",
		"SERVICE_1":      "ESCALATION_POLICY_3",
	})

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return an error when DEFAULT escalation policy is missing")
	assert.Contains(t, err.Error(), "DEFAULT", "error should mention the missing 'DEFAULT' key")
}

func TestValidateConfig_MissingSilentDefaultPolicy(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1", "TEAM2"})
	viper.Set("service_escalation_policies", map[string]interface{}{
		"default":   "ESCALATION_POLICY_1",
		"SERVICE_1": "ESCALATION_POLICY_3",
	})

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return an error when SILENT_DEFAULT escalation policy is missing")
	assert.Contains(t, err.Error(), "SILENT_DEFAULT", "error should mention the missing 'SILENT_DEFAULT' key")
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	viper.Reset()
	// Set nothing - should get errors for all three required keys

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return errors when all required keys are missing")
	assert.Contains(t, err.Error(), "token", "error should mention the missing 'token' key")
	assert.Contains(t, err.Error(), "teams", "error should mention the missing 'teams' key")
}

func TestValidateConfig_OptionalKeysGetDefaults(t *testing.T) {
	viper.Reset()
	setValidConfig()
	// Do not set any optional keys

	err := validateConfig()

	assert.NoError(t, err, "validateConfig should not error for missing optional keys")
	assert.Equal(t, "vim", viper.GetString("editor"), "editor should default to 'vim'")
	assert.Equal(t, "gnome-terminal --", viper.GetString("terminal"), "terminal should default to 'gnome-terminal --'")
	assert.Equal(t, "ocm backplane login %%CLUSTER_ID%%", viper.GetString("cluster_login_command"), "cluster_login_command should get default value")
	assert.Equal(t, "auto", viper.GetString("toolbox_mode"), "toolbox_mode should default to 'auto'")
	assert.Equal(t, "ctrl+x", viper.GetString("chord_prefix"), "chord_prefix should default to 'ctrl+x'")
}

func TestValidateConfig_InvalidEscalationPoliciesType(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1", "TEAM2"})
	// Set service_escalation_policies as a string instead of a map
	viper.Set("service_escalation_policies", "not-a-map")

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return an error when service_escalation_policies is a string instead of a map")
	assert.Contains(t, err.Error(), "not a valid map", "error should indicate the value is not a valid map")
}

func TestValidateConfig_DeprecatedKeyDetected(t *testing.T) {
	viper.Reset()
	setValidConfig()
	// Add a deprecated key
	viper.Set("shell", "/bin/bash")

	err := validateConfig()

	// Deprecated keys should not cause an error; they are logged as informational
	assert.NoError(t, err, "validateConfig should not return an error for deprecated keys")
}

func TestValidateConfig_CaseSensitiveEscalationKeys(t *testing.T) {
	// The validateConfig code looks up escalation policy keys using
	// strings.ToLower(), so it always searches for lowercase "default" and
	// "silent_default" in the settings map. Viper also normalizes keys to
	// lowercase. This test verifies that when the inner map keys do NOT
	// contain the lowercase forms that the code expects, validation fails.
	//
	// We simulate this by providing a map where neither "default" nor
	// "silent_default" exists -- only unrelated keys -- to prove the
	// lookup is case-sensitive at the Go map level.
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1", "TEAM2"})
	viper.Set("service_escalation_policies", map[string]interface{}{
		"some_other_service": "ESCALATION_POLICY_1",
	})

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return an error when required escalation policy keys (default, silent_default) are missing")
	assert.Contains(t, err.Error(), "DEFAULT", "error should mention the missing 'DEFAULT' key")
	assert.Contains(t, err.Error(), "SILENT_DEFAULT", "error should mention the missing 'SILENT_DEFAULT' key")
}

// mockFS is a fully in-memory mock of the configFS interface.
type mockFS struct {
	mkdirAllErr error
	openFileErr error
	buf         bytes.Buffer
	openFlags   int
	openPerm    os.FileMode
	mkdirPerm   os.FileMode
	mkdirPath   string
	openPath    string
	readData    []byte
	readErr     error
	writeData   []byte
	writeErr    error
	writePath   string
	backupData  []byte
}

func (m *mockFS) MkdirAll(path string, perm os.FileMode) error {
	m.mkdirPath = path
	m.mkdirPerm = perm
	return m.mkdirAllErr
}

type nopCloser struct{ *bytes.Buffer }

func (nopCloser) Close() error { return nil }

func (m *mockFS) ReadFile(name string) ([]byte, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	return m.readData, nil
}

func (m *mockFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	if strings.HasSuffix(name, "~") {
		m.backupData = data
	} else {
		m.writePath = name
		m.writeData = data
	}
	return nil
}

func (m *mockFS) OpenFile(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	m.openPath = name
	m.openFlags = flag
	m.openPerm = perm
	if m.openFileErr != nil {
		return nil, m.openFileErr
	}
	return nopCloser{&m.buf}, nil
}

func TestCreateConfig_Success(t *testing.T) {
	m := &mockFS{}

	err := createConfig(m, "/fake/home")

	require.NoError(t, err)

	expected := strings.TrimLeft(exampleConfig, "\n")
	assert.Equal(t, expected, m.buf.String())
}

func TestCreateConfig_ContentTrimmed(t *testing.T) {
	m := &mockFS{}

	err := createConfig(m, "/fake/home")

	require.NoError(t, err)
	assert.True(t, len(m.buf.Bytes()) > 0, "written content should not be empty")
	assert.Equal(t, byte('#'), m.buf.Bytes()[0],
		"config file should start with '#', not a blank line")
}

func TestCreateConfig_UsesCorrectPaths(t *testing.T) {
	m := &mockFS{}

	err := createConfig(m, "/fake/home")

	require.NoError(t, err)

	expectedDir := filepath.Join("/fake/home", cfgFileDir)
	expectedFile := filepath.Join(expectedDir, cfgFileName)

	assert.Equal(t, expectedDir, m.mkdirPath)
	assert.Equal(t, expectedFile, m.openPath)
}

func TestCreateConfig_UsesExclFlag(t *testing.T) {
	m := &mockFS{}

	err := createConfig(m, "/fake/home")

	require.NoError(t, err)

	expectedFlags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	assert.Equal(t, expectedFlags, m.openFlags,
		"OpenFile should use O_WRONLY|O_CREATE|O_EXCL for atomic creation")
}

func TestCreateConfig_UsesCorrectPerms(t *testing.T) {
	m := &mockFS{}

	err := createConfig(m, "/fake/home")

	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), m.mkdirPerm, "directory should use 0755 permissions")
	assert.Equal(t, os.FileMode(0644), m.openPerm, "file should use 0644 permissions")
}

func TestCreateConfig_ErrorWhenFileExists(t *testing.T) {
	m := &mockFS{
		openFileErr: &os.PathError{
			Op:   "open",
			Path: "/fake/home/.config/srepd/srepd.yaml",
			Err:  os.ErrExist,
		},
	}

	err := createConfig(m, "/fake/home")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Contains(t, err.Error(), fmt.Sprintf("%s/%s/%s", "/fake/home", cfgFileDir, cfgFileName))
}

func TestCreateConfig_ErrorOnMkdirFail(t *testing.T) {
	m := &mockFS{
		mkdirAllErr: errors.New("permission denied"),
	}

	err := createConfig(m, "/fake/home")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create config directory")
	assert.Contains(t, err.Error(), "permission denied")
}

func TestCreateConfig_ErrorOnOpenFail(t *testing.T) {
	m := &mockFS{
		openFileErr: errors.New("disk full"),
	}

	err := createConfig(m, "/fake/home")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create config file")
	assert.Contains(t, err.Error(), "disk full")
}

func TestHasPlaceholderTeams_Empty(t *testing.T) {
	assert.True(t, hasPlaceholderTeams([]string{}))
}

func TestHasPlaceholderTeams_Nil(t *testing.T) {
	assert.True(t, hasPlaceholderTeams(nil))
}

func TestHasPlaceholderTeams_AllPlaceholders(t *testing.T) {
	teams := []string{"<PagerDuty Team ID 1>", "<PagerDuty Team ID 2>"}
	assert.True(t, hasPlaceholderTeams(teams))
}

func TestHasPlaceholderTeams_OneReal(t *testing.T) {
	teams := []string{"P1ABC23"}
	assert.False(t, hasPlaceholderTeams(teams))
}

func TestHasPlaceholderTeams_MixedRealAndPlaceholder(t *testing.T) {
	teams := []string{"<PagerDuty Team ID 1>", "P1ABC23"}
	assert.False(t, hasPlaceholderTeams(teams))
}

func TestHasPlaceholderTeams_WhitespaceOnly(t *testing.T) {
	teams := []string{"  ", ""}
	assert.True(t, hasPlaceholderTeams(teams))
}

func TestUpdateTeamsInConfig_ReplacesPlaceholders(t *testing.T) {
	input := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
  - <PagerDuty Team ID 2>
editor: vim
`)
	result, err := updateTeamsInConfig(input, []string{"P1ABC23", "P4DEF56"}, map[string]string{"P1ABC23": "Team Alpha", "P4DEF56": "Team Beta"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "- P1ABC23")
	assert.Contains(t, string(result), "- P4DEF56")
	assert.NotContains(t, string(result), "<PagerDuty Team ID")
	assert.Contains(t, string(result), "token: my-token")
	assert.Contains(t, string(result), "editor: vim")
}

func TestUpdateTeamsInConfig_AddsTeamNameComments(t *testing.T) {
	input := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
`)
	result, err := updateTeamsInConfig(input, []string{"PASPK4G"}, map[string]string{"PASPK4G": "Platform SRE"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "- PASPK4G # Platform SRE")
}

func TestUpdateTeamsInConfig_PreservesComments(t *testing.T) {
	input := []byte(`# Main config
token: my-token
# Teams to filter on
teams:
  - <PagerDuty Team ID 1>
editor: vim
`)
	result, err := updateTeamsInConfig(input, []string{"PREAL1"}, map[string]string{"PREAL1": "Real Team"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "# Main config")
	assert.Contains(t, string(result), "# Teams to filter on")
	assert.Contains(t, string(result), "- PREAL1")
}

func TestUpdateTeamsInConfig_EmptyTeams(t *testing.T) {
	input := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
`)
	result, err := updateTeamsInConfig(input, []string{}, nil)

	require.NoError(t, err)
	assert.Contains(t, string(result), "teams: []")
}

func TestUpdateTeamsInConfig_NoTeamsKey(t *testing.T) {
	input := []byte(`token: my-token
editor: vim
`)
	_, err := updateTeamsInConfig(input, []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams")
}

func TestWriteConfigTeams_Success(t *testing.T) {
	configData := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
`)
	m := &mockFS{readData: configData}

	err := writeConfigTeams(m, "/fake/home", []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

	require.NoError(t, err)
	assert.Contains(t, string(m.writeData), "- P1ABC23")
	assert.NotContains(t, string(m.writeData), "<PagerDuty Team ID")
}

func TestWriteConfigTeams_CreatesBackup(t *testing.T) {
	configData := []byte(`token: my-token
teams:
  - OLD_TEAM
`)
	m := &mockFS{readData: configData}

	err := writeConfigTeams(m, "/fake/home", []string{"NEW_TEAM"}, map[string]string{"NEW_TEAM": "New"})

	require.NoError(t, err)
	assert.Equal(t, string(configData), string(m.backupData))
}

func TestWriteConfigTeams_ReadError(t *testing.T) {
	m := &mockFS{readErr: errors.New("no such file")}

	err := writeConfigTeams(m, "/fake/home", []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestWriteConfigTeams_WriteError(t *testing.T) {
	configData := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
`)
	m := &mockFS{readData: configData, writeErr: errors.New("disk full")}

	err := writeConfigTeams(m, "/fake/home", []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}
