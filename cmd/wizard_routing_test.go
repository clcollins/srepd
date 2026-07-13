package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestClassifyStartup_ValidConfigRoutesNormal(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1"})

	route, reason := classifyStartup()

	assert.Equal(t, routeNormal, route)
	assert.Empty(t, reason)
}

func TestClassifyStartup_PlaceholderTokenRoutesWizard(t *testing.T) {
	viper.Reset()
	viper.Set("token", "<PagerDuty API token>")
	viper.Set("teams", []string{"TEAM1"})

	route, reason := classifyStartup()

	assert.Equal(t, routeWizard, route)
	assert.NotEmpty(t, reason)
}

func TestClassifyStartup_MissingTokenRoutesWizard(t *testing.T) {
	viper.Reset()
	viper.Set("teams", []string{"TEAM1"})

	route, reason := classifyStartup()

	assert.Equal(t, routeWizard, route)
	assert.NotEmpty(t, reason)
}

func TestClassifyStartup_MissingTeamsRoutesWizard(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")

	route, reason := classifyStartup()

	assert.Equal(t, routeWizard, route)
	assert.NotEmpty(t, reason)
}

func TestClassifyStartup_StructurallyInvalidRoutesFatal(t *testing.T) {
	viper.Reset()
	viper.Set("token", "test-pagerduty-token")
	viper.Set("teams", []string{"TEAM1"})
	viper.Set("service_escalation_policies", "not-a-map")

	route, reason := classifyStartup()

	assert.Equal(t, routeFatal, route)
	assert.NotEmpty(t, reason)
}

// A salvaged token (from a YAML-broken file) with no teams still routes to
// the wizard — the token is preserved, but teams are still needed.
func TestClassifyStartup_SalvagedTokenNoTeamsRoutesWizard(t *testing.T) {
	viper.Reset()
	viper.Set("token", "u+salvaged-token")

	route, reason := classifyStartup()
	assert.Equal(t, routeWizard, route)
	assert.Contains(t, reason, "teams")
}

// Values supplied only via SREPD_* env vars must count as configured: the
// classifier reads viper's live accessors, not just the settings map.
func TestClassifyStartup_EnvTokenRoutesNormal(t *testing.T) {
	viper.Reset()
	t.Setenv("SREPD_TOKEN", "env-pagerduty-token")
	viper.SetEnvPrefix("srepd")
	viper.AutomaticEnv()
	viper.Set("teams", []string{"TEAM1"})

	route, reason := classifyStartup()

	assert.Equal(t, routeNormal, route)
	assert.Empty(t, reason)
}

// validateConfig must not report a required key missing when it is supplied
// via an SREPD_* env var (AllSettings does not include env-resolved values).
func TestValidateConfig_RequiredKeyFromEnv(t *testing.T) {
	viper.Reset()
	t.Setenv("SREPD_TOKEN", "env-pagerduty-token")
	viper.SetEnvPrefix("srepd")
	viper.AutomaticEnv()
	viper.Set("teams", []string{"TEAM1"})

	err := validateConfig()

	assert.NoError(t, err)
}
