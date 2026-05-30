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

func TestValidateConfig_LLMApiValid(t *testing.T) {
	viper.Reset()
	setValidConfig()
	t.Setenv("TEST_LLM_KEY", "sk-ant-test-key")
	viper.Set("llm_api", map[string]interface{}{
		"provider":    "anthropic",
		"api_key_env": "TEST_LLM_KEY",
		"model":       "claude-sonnet-4-6",
	})

	err := validateConfig()

	assert.NoError(t, err, "validateConfig should not error when llm_api is valid")
}

func TestValidateConfig_LLMApiMissing(t *testing.T) {
	viper.Reset()
	setValidConfig()
	// No llm_api set at all - should be fine since it is optional

	err := validateConfig()

	assert.NoError(t, err, "validateConfig should not error when llm_api is absent")
}

func TestValidateConfig_LLMApiInvalidProvider(t *testing.T) {
	viper.Reset()
	setValidConfig()
	viper.Set("llm_api", map[string]interface{}{
		"provider":    "unknown-llm",
		"api_key_env": "SOME_KEY",
	})

	// Invalid llm_api should not cause validateConfig to fail (it's optional),
	// but it should log a warning. The error from validateLLMConfig is only logged.
	err := validateConfig()

	assert.NoError(t, err, "validateConfig should not error for invalid llm_api (it's optional)")
}

func TestValidateLLMConfig_Valid(t *testing.T) {
	viper.Reset()
	t.Setenv("TEST_LLM_VALID_KEY", "sk-ant-test-valid")
	viper.Set("llm_api.provider", "anthropic")
	viper.Set("llm_api.api_key_env", "TEST_LLM_VALID_KEY")
	viper.Set("llm_api.model", "claude-sonnet-4-6")

	err := validateLLMConfig()

	assert.NoError(t, err, "validateLLMConfig should succeed with valid config")
}

func TestValidateLLMConfig_UnknownProvider(t *testing.T) {
	viper.Reset()
	viper.Set("llm_api.provider", "chatgpt")
	viper.Set("llm_api.api_key_env", "SOME_KEY")

	err := validateLLMConfig()

	assert.Error(t, err, "validateLLMConfig should error for unknown provider")
	assert.Contains(t, err.Error(), "unknown provider", "error should mention unknown provider")
}

func TestValidateLLMConfig_MissingAPIKeyEnv(t *testing.T) {
	viper.Reset()
	viper.Set("llm_api.provider", "anthropic")
	viper.Set("llm_api.api_key_env", "")

	err := validateLLMConfig()

	assert.Error(t, err, "validateLLMConfig should error when api_key_env is empty")
}

func TestValidateLLMConfig_EmptyEnvVar(t *testing.T) {
	viper.Reset()
	t.Setenv("TEST_EMPTY_LLM_KEY", "")
	viper.Set("llm_api.provider", "anthropic")
	viper.Set("llm_api.api_key_env", "TEST_EMPTY_LLM_KEY")

	err := validateLLMConfig()

	assert.Error(t, err, "validateLLMConfig should error when env var is empty")
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
