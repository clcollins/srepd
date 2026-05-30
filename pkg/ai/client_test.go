package ai

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/alert"
	"github.com/stretchr/testify/assert"
)

func TestNewProvider_Anthropic(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test-key-123")

	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
		Model:     "claude-sonnet-4-6",
	}

	provider, err := NewProvider(cfg)

	assert.NoError(t, err, "NewProvider should succeed with valid Anthropic config")
	assert.NotNil(t, provider, "provider should not be nil")
	assert.Equal(t, "anthropic", provider.Name(), "provider name should be 'anthropic'")
}

func TestNewProvider_AnthropicWithEndpoint(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test-key-123")

	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
		Model:     "claude-sonnet-4-6",
		Endpoint:  "https://custom.api.example.com",
	}

	provider, err := NewProvider(cfg)

	assert.NoError(t, err, "NewProvider should succeed with custom endpoint")
	assert.NotNil(t, provider, "provider should not be nil")
}

func TestNewProvider_AnthropicDefaultModel(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test-key-123")

	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
		// Model intentionally left empty to test default
	}

	provider, err := NewProvider(cfg)

	assert.NoError(t, err, "NewProvider should succeed with empty model (uses default)")
	assert.NotNil(t, provider, "provider should not be nil")

	// Verify the default model is set via the internal struct
	ap, ok := provider.(*anthropicProvider)
	assert.True(t, ok, "provider should be *anthropicProvider")
	assert.Equal(t, defaultModel, ap.model, "model should default to %q", defaultModel)
}

func TestNewProvider_Unknown(t *testing.T) {
	t.Setenv("TEST_UNKNOWN_KEY", "some-key")

	cfg := Config{
		Provider:  "unknown-provider",
		APIKeyEnv: "TEST_UNKNOWN_KEY",
	}

	provider, err := NewProvider(cfg)

	assert.Error(t, err, "NewProvider should return error for unknown provider")
	assert.Nil(t, provider, "provider should be nil for unknown provider")
	assert.Contains(t, err.Error(), "unknown provider", "error should mention unknown provider")
}

func TestNewProvider_EmptyProvider(t *testing.T) {
	cfg := Config{
		Provider: "",
	}

	provider, err := NewProvider(cfg)

	assert.Error(t, err, "NewProvider should return error for empty provider")
	assert.Nil(t, provider, "provider should be nil for empty provider")
	assert.Contains(t, err.Error(), "provider name is required", "error should mention provider name is required")
}

func TestNewProvider_MissingAPIKeyEnv(t *testing.T) {
	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "",
	}

	provider, err := NewProvider(cfg)

	assert.Error(t, err, "NewProvider should return error when api_key_env is empty")
	assert.Nil(t, provider, "provider should be nil when api_key_env is empty")
	assert.Contains(t, err.Error(), "api_key_env is required", "error should mention api_key_env is required")
}

func TestNewProvider_EmptyAPIKey(t *testing.T) {
	t.Setenv("TEST_EMPTY_KEY", "")

	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_EMPTY_KEY",
	}

	provider, err := NewProvider(cfg)

	assert.Error(t, err, "NewProvider should return error when API key env var is empty")
	assert.Nil(t, provider, "provider should be nil when API key is empty")
	assert.Contains(t, err.Error(), "not set or empty", "error should mention env var is not set or empty")
}

func TestNewProvider_UnsetAPIKeyEnv(t *testing.T) {
	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "DEFINITELY_NOT_SET_ENV_VAR_12345",
	}

	provider, err := NewProvider(cfg)

	assert.Error(t, err, "NewProvider should return error when API key env var is unset")
	assert.Nil(t, provider, "provider should be nil when API key env var is unset")
	assert.Contains(t, err.Error(), "DEFINITELY_NOT_SET_ENV_VAR_12345", "error should include env var name")
}

func TestValidateConfig_Valid(t *testing.T) {
	t.Setenv("TEST_VALID_KEY", "sk-ant-test-key-valid")

	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_VALID_KEY",
		Model:     "claude-sonnet-4-6",
	}

	err := ValidateConfig(cfg)

	assert.NoError(t, err, "ValidateConfig should return nil for valid config")
}

func TestValidateConfig_EmptyProvider(t *testing.T) {
	cfg := Config{
		Provider: "",
	}

	err := ValidateConfig(cfg)

	assert.Error(t, err, "ValidateConfig should return error for empty provider")
}

func TestValidateConfig_UnknownProvider(t *testing.T) {
	cfg := Config{
		Provider:  "chatgpt",
		APIKeyEnv: "SOME_KEY",
	}

	err := ValidateConfig(cfg)

	assert.Error(t, err, "ValidateConfig should return error for unknown provider")
	assert.Contains(t, err.Error(), "unknown provider", "error should mention unknown provider")
}

func TestValidateConfig_MissingAPIKeyEnv(t *testing.T) {
	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "",
	}

	err := ValidateConfig(cfg)

	assert.Error(t, err, "ValidateConfig should return error for missing api_key_env")
}

func TestValidateConfig_EmptyEnvVar(t *testing.T) {
	t.Setenv("TEST_EMPTY", "")

	cfg := Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_EMPTY",
	}

	err := ValidateConfig(cfg)

	assert.Error(t, err, "ValidateConfig should return error for empty env var")
}

func TestBuildSystemPrompt_WithIncident(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID: "P123ABC",
		},
		Title:   "ClusterOperatorDown WARNING (1)",
		Status:  "triggered",
		Urgency: "high",
		Service: pagerduty.APIObject{
			Summary: "osd-test-cluster-hive-cluster",
		},
	}

	alerts := []alert.NormalizedAlert{
		{
			AlertName: "ClusterOperatorDown",
			ClusterID: "abc-123-def-456",
			Severity:  "warning",
		},
		{
			AlertName: "PodDisruptionBudgetLimit",
			ClusterID: "abc-123-def-456",
			Severity:  "critical",
		},
	}

	prompt := BuildSystemPrompt(incident, alerts)

	assert.Contains(t, prompt, "ClusterOperatorDown WARNING (1)", "prompt should contain incident title")
	assert.Contains(t, prompt, "P123ABC", "prompt should contain incident ID")
	assert.Contains(t, prompt, "osd-test-cluster-hive-cluster", "prompt should contain service name")
	assert.Contains(t, prompt, "triggered", "prompt should contain status")
	assert.Contains(t, prompt, "high", "prompt should contain urgency")
	assert.Contains(t, prompt, "abc-123-def-456", "prompt should contain cluster ID")
	assert.Contains(t, prompt, "2", "prompt should contain alert count")
	assert.Contains(t, prompt, "ClusterOperatorDown", "prompt should contain first alert name")
	assert.Contains(t, prompt, "PodDisruptionBudgetLimit", "prompt should contain second alert name")
	assert.Contains(t, prompt, "Do not suggest destructive commands", "prompt should contain safety instruction")
}

func TestBuildSystemPrompt_NilIncident(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil)

	assert.Contains(t, prompt, "SRE assistant", "nil incident prompt should identify as SRE assistant")
	assert.Contains(t, prompt, "Do not suggest destructive commands", "nil incident prompt should contain safety instruction")
	assert.NotContains(t, prompt, "Incident:", "nil incident prompt should not contain incident details")
}

func TestBuildSystemPrompt_EmptyAlerts(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID: "P456DEF",
		},
		Title:   "Test Incident",
		Status:  "acknowledged",
		Urgency: "low",
		Service: pagerduty.APIObject{
			Summary: "test-service",
		},
	}

	prompt := BuildSystemPrompt(incident, []alert.NormalizedAlert{})

	assert.Contains(t, prompt, "P456DEF", "prompt should contain incident ID with empty alerts")
	assert.Contains(t, prompt, "Alert count: 0", "prompt should show zero alert count")
	assert.Contains(t, prompt, "Alert names: ", "prompt should have empty alert names")
}

func TestBuildSystemPrompt_AlertsWithEmptyNames(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID: "P789GHI",
		},
		Title:   "Test Incident",
		Status:  "triggered",
		Urgency: "high",
		Service: pagerduty.APIObject{
			Summary: "test-service",
		},
	}

	alerts := []alert.NormalizedAlert{
		{AlertName: "SomeAlert", ClusterID: "cluster-1"},
		{AlertName: "", ClusterID: "cluster-1"},    // empty alert name
		{AlertName: "AnotherAlert", ClusterID: ""}, // empty cluster ID
	}

	prompt := BuildSystemPrompt(incident, alerts)

	assert.Contains(t, prompt, "SomeAlert", "prompt should contain non-empty alert names")
	assert.Contains(t, prompt, "AnotherAlert", "prompt should contain non-empty alert names")
	assert.Contains(t, prompt, "cluster-1", "prompt should use first non-empty cluster ID")
	assert.Contains(t, prompt, "Alert count: 3", "prompt should count all alerts including those with empty names")
}

func TestExtractClusterID_FirstWithID(t *testing.T) {
	alerts := []alert.NormalizedAlert{
		{ClusterID: ""},
		{ClusterID: "cluster-abc"},
		{ClusterID: "cluster-def"},
	}

	result := extractClusterID(alerts)

	assert.Equal(t, "cluster-abc", result, "should return first non-empty cluster ID")
}

func TestExtractClusterID_NoneWithID(t *testing.T) {
	alerts := []alert.NormalizedAlert{
		{ClusterID: ""},
		{ClusterID: ""},
	}

	result := extractClusterID(alerts)

	assert.Equal(t, "", result, "should return empty string when no alerts have cluster IDs")
}

func TestExtractClusterID_EmptySlice(t *testing.T) {
	result := extractClusterID([]alert.NormalizedAlert{})

	assert.Equal(t, "", result, "should return empty string for empty slice")
}

func TestExtractClusterID_NilSlice(t *testing.T) {
	result := extractClusterID(nil)

	assert.Equal(t, "", result, "should return empty string for nil slice")
}

func TestKnownProviders(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		known    bool
	}{
		{name: "anthropic is known", provider: "anthropic", known: true},
		{name: "openai is not yet known", provider: "openai", known: false},
		{name: "ollama is not yet known", provider: "ollama", known: false},
		{name: "empty is not known", provider: "", known: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.known, knownProviders[tt.provider],
				"provider %q known status should be %v", tt.provider, tt.known)
		})
	}
}
