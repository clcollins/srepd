package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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

func TestValidateConfig_MissingEscalationPolicies(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1", "TEAM2"})

	err := validateConfig()

	assert.Error(t, err, "validateConfig should return an error when service_escalation_policies is missing")
	assert.Contains(t, err.Error(), "service_escalation_policies", "error should mention the missing 'service_escalation_policies' key")
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
	assert.Contains(t, err.Error(), "service_escalation_policies", "error should mention the missing 'service_escalation_policies' key")
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
}
