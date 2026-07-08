package cmd

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

//go:embed config.go
var configGoSource string

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

// --- ensureViperDefaults tests ---

func TestEnsureViperDefaults_SetsAllOptionalKeys(t *testing.T) {
	viper.Reset()

	ensureViperDefaults()

	assert.Equal(t, "vim", viper.GetString("editor"))
	assert.Equal(t, "gnome-terminal --", viper.GetString("terminal"))
	assert.Equal(t, "ocm backplane login %%CLUSTER_ID%%", viper.GetString("cluster_login_command"))
	assert.Equal(t, "auto", viper.GetString("toolbox_mode"))
	assert.Equal(t, "ctrl+x", viper.GetString("chord_prefix"))
}

func TestEnsureViperDefaults_DoesNotOverrideExisting(t *testing.T) {
	viper.Reset()
	viper.Set("terminal", "ptyxis")
	viper.Set("editor", "nano")

	ensureViperDefaults()

	assert.Equal(t, "ptyxis", viper.GetString("terminal"))
	assert.Equal(t, "nano", viper.GetString("editor"))
	assert.Equal(t, "ocm backplane login %%CLUSTER_ID%%", viper.GetString("cluster_login_command"))
}

func TestEnsureViperDefaults_NewUserReadyForLaunch(t *testing.T) {
	viper.Reset()
	viper.Set("token", "new-user-token")
	viper.Set("teams", []string{"TEAM1"})

	ensureViperDefaults()

	assert.NotEmpty(t, viper.GetString("terminal"), "terminal must be set for launchTUI")
	assert.NotEmpty(t, viper.GetString("editor"), "editor must be set for launchTUI")
	clc := viper.GetString("cluster_login_command")
	assert.NotEmpty(t, clc, "cluster_login_command must be set for launchTUI")
	assert.Contains(t, clc, "%%CLUSTER_ID%%", "cluster_login_command must contain %%CLUSTER_ID%% placeholder")
}

func TestWizardHasZeroHuhFormsInCmd(t *testing.T) {
	formCount := strings.Count(configGoSource, "huh.NewForm(")

	assert.Equal(t, 0, formCount, "cmd/config.go should have zero huh.NewForm calls; the form is now in pkg/tui/")
}

func TestRunConfigWizardCallsLaunchTUIWithConfig(t *testing.T) {
	assert.Contains(t, configGoSource, "launchTUIWithConfig()",
		"runConfigWizard should call launchTUIWithConfig to launch the TUI in config mode")
}

// TestMaskConfigValue verifies the config-value masking used in --debug logging is
// allowlist-based: only known-safe keys reveal their value; everything else (any
// current or future secret-bearing key, not just those literally named "token") is
// masked.
func TestMaskConfigValue(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		masked bool
	}{
		{"token is masked", "token", "PCGXUDY1", true},
		{"api_key is masked", "api_key", "sk-secret", true},
		{"secret is masked", "webhook_secret", "shh", true},
		{"password is masked", "password", "hunter2", true},
		{"unknown key masked by default", "some_future_key", "sensitive", true},
		{"teams is revealed", "teams", "TEAM1", false},
		{"editor is revealed", "editor", "vim", false},
		{"terminal is revealed", "terminal", "gnome-terminal --", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskConfigValue(tt.key, tt.value)
			if tt.masked {
				assert.Equal(t, "*****", got, "value should be masked")
				assert.NotContains(t, got, tt.value)
			} else {
				assert.Equal(t, tt.value, got, "known-safe key should reveal its value")
			}
		})
	}
}
