package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestHasPlaceholderToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"readme literal", "<PagerDuty API token>", true},
		{"generic angle brackets", "<token>", true},
		{"angle brackets with spaces", "  <PagerDuty API token>  ", true},
		{"real token", "u+wXyZ1234567890abcd", false},
		{"real token with whitespace", "  u+wXyZ1234567890abcd  ", false},
		{"leading angle only", "<unclosed", false},
		{"trailing angle only", "unopened>", false},
		{"angle brackets inside", "ab<cd>ef", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasPlaceholderToken(tt.token))
		})
	}
}

func TestClassifyConfigHealth(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		teams      []string
		settings   map[string]any
		wantHealth ConfigHealth
		wantReason string
	}{
		{
			name:       "valid config",
			token:      "u+realtoken123",
			teams:      []string{"PTEAM01"},
			settings:   map[string]any{"token": "u+realtoken123", "teams": []any{"PTEAM01"}},
			wantHealth: HealthOK,
		},
		{
			name:       "missing token",
			token:      "",
			teams:      []string{"PTEAM01"},
			settings:   map[string]any{"teams": []any{"PTEAM01"}},
			wantHealth: HealthNeedsWizard,
			wantReason: "no PagerDuty API token configured",
		},
		{
			name:       "placeholder token from README",
			token:      "<PagerDuty API token>",
			teams:      []string{"PTEAM01"},
			settings:   map[string]any{"token": "<PagerDuty API token>", "teams": []any{"PTEAM01"}},
			wantHealth: HealthNeedsWizard,
			wantReason: "the configured PagerDuty API token is a placeholder value",
		},
		{
			name:       "missing teams",
			token:      "u+realtoken123",
			teams:      nil,
			settings:   map[string]any{"token": "u+realtoken123"},
			wantHealth: HealthNeedsWizard,
			wantReason: "no PagerDuty teams configured",
		},
		{
			name:       "placeholder teams",
			token:      "u+realtoken123",
			teams:      []string{"<PagerDuty Team ID>"},
			settings:   map[string]any{"token": "u+realtoken123", "teams": []any{"<PagerDuty Team ID>"}},
			wantHealth: HealthNeedsWizard,
			wantReason: "no PagerDuty teams configured",
		},
		{
			name:       "readme-style generic team placeholder",
			token:      "u+realtoken123",
			teams:      []string{"<team ID>"},
			settings:   map[string]any{"token": "u+realtoken123", "teams": []any{"<team ID>"}},
			wantHealth: HealthNeedsWizard,
			wantReason: "no PagerDuty teams configured",
		},
		{
			name:       "everything missing",
			token:      "",
			teams:      nil,
			settings:   map[string]any{},
			wantHealth: HealthNeedsWizard,
			wantReason: "no PagerDuty API token configured",
		},
		{
			name:       "legacy policies not a map is structural",
			token:      "u+realtoken123",
			teams:      []string{"PTEAM01"},
			settings:   map[string]any{"token": "u+realtoken123", "teams": []any{"PTEAM01"}, "service_escalation_policies": "not-a-map"},
			wantHealth: HealthInvalid,
			wantReason: "'service_escalation_policies' is not a valid map",
		},
		{
			name:       "structural problem wins over wizard-shaped problem",
			token:      "",
			teams:      nil,
			settings:   map[string]any{"service_escalation_policies": []any{"nope"}},
			wantHealth: HealthInvalid,
			wantReason: "'service_escalation_policies' is not a valid map",
		},
		{
			name:       "legacy policies as valid map is fine",
			token:      "u+realtoken123",
			teams:      []string{"PTEAM01"},
			settings:   map[string]any{"token": "u+realtoken123", "teams": []any{"PTEAM01"}, "service_escalation_policies": map[string]any{"default": "P123", "silent_default": "P456"}},
			wantHealth: HealthOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health, reason := ClassifyConfigHealth(tt.token, tt.teams, tt.settings)
			assert.Equal(t, tt.wantHealth, health)
			if tt.wantReason != "" {
				assert.Equal(t, tt.wantReason, reason)
			}
			if tt.wantHealth == HealthOK {
				assert.Empty(t, reason)
			}
		})
	}
}

// HasPlaceholderTeams must recognize generic angle-bracket placeholders like
// the README's "<team ID>", not only the "<PagerDuty Team ID" prefix.
func TestHasPlaceholderTeams_GenericAngleBrackets(t *testing.T) {
	assert.True(t, HasPlaceholderTeams([]string{"<team ID>"}))
	assert.True(t, HasPlaceholderTeams([]string{"<team ID>", "<PagerDuty Team ID 1>"}))
	assert.False(t, HasPlaceholderTeams([]string{"<team ID>", "PTEAM01"}))
}

// Regression for OB-1: a config file built by copying the README example
// verbatim must route the user into the wizard, never a fatal error.
func TestClassifyConfigHealth_ReadmeExampleRoutesToWizard(t *testing.T) {
	readmeExample := `
token: <PagerDuty API token>
teams:
  - <team ID>
default_silent_escalation_policy: P654321
terminal: gnome-terminal
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%
rosa_boundary_command: rosa-boundary start-task --cluster-id %%CLUSTER_ID%% --connect
toolbox_mode: auto
`
	var settings map[string]any
	err := yaml.Unmarshal([]byte(readmeExample), &settings)
	assert.NoError(t, err)

	token, _ := settings["token"].(string)
	var teams []string
	if raw, ok := settings["teams"].([]any); ok {
		for _, entry := range raw {
			teams = append(teams, entry.(string))
		}
	}

	health, reason := ClassifyConfigHealth(token, teams, settings)
	assert.Equal(t, HealthNeedsWizard, health)
	assert.NotEmpty(t, reason)
}

// An empty (zero-byte) config file must also route to the wizard.
func TestClassifyConfigHealth_EmptyFileRoutesToWizard(t *testing.T) {
	health, reason := ClassifyConfigHealth("", nil, map[string]any{})
	assert.Equal(t, HealthNeedsWizard, health)
	assert.NotEmpty(t, reason)
}
