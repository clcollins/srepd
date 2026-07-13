package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// #324 item 2: `srepd config generate` emits a complete, annotated config —
// every supported key, default values, and comments explaining each section —
// for users who prefer editing a file over the wizard.

func TestGenerateAnnotatedConfig_ValidYAML(t *testing.T) {
	out := GenerateAnnotatedConfig(nil)

	var parsed map[string]any
	assert.NoError(t, yaml.Unmarshal(out, &parsed), "generated config must be valid YAML")
}

func TestGenerateAnnotatedConfig_RequiredKeysWithHelp(t *testing.T) {
	out := string(GenerateAnnotatedConfig(nil))

	assert.Contains(t, out, `token: ""`)
	assert.Contains(t, out, "User Settings", "token comment must include the acquisition path")
	assert.Contains(t, out, "teams: []")
	assert.Contains(t, out, "exactly one team", "teams comment must carry the single-team tip")
	assert.Contains(t, out, "srepd config", "must point at the wizard as the recommended path")
}

func TestGenerateAnnotatedConfig_OptionalDefaults(t *testing.T) {
	out := string(GenerateAnnotatedConfig(nil))

	for _, want := range []string{
		"editor: vim",
		"terminal: gnome-terminal --",
		"cluster_login_command: ocm backplane login %%CLUSTER_ID%%",
		"toolbox_mode: auto",
		"chord_prefix: ctrl+x",
		"emoji: true",
		"reescalate_level: 2",
		"stream_responses: true",
	} {
		assert.Contains(t, out, want)
	}
}

func TestGenerateAnnotatedConfig_OptionalSectionsCommented(t *testing.T) {
	out := string(GenerateAnnotatedConfig(nil))

	assert.Contains(t, out, `# default_silent_escalation_policy:`)
	assert.Contains(t, out, `# custom_service_escalation_policies:`)
	assert.Contains(t, out, `# agent_cli_command:`)
	assert.Contains(t, out, `# colors:`)
	assert.Contains(t, out, `# llm_api:`)
}

// Issue #322: flag_marker is dead — the generator must not advertise it.
func TestGenerateAnnotatedConfig_NoDeadKeys(t *testing.T) {
	out := string(GenerateAnnotatedConfig(nil))
	assert.NotContains(t, out, "flag_marker")
}

// A generated file must route into the wizard (OB-1), never a fatal error.
func TestGenerateAnnotatedConfig_RoutesToWizard(t *testing.T) {
	out := GenerateAnnotatedConfig(nil)

	var settings map[string]any
	assert.NoError(t, yaml.Unmarshal(out, &settings))

	token, _ := settings["token"].(string)
	var teams []string
	if raw, ok := settings["teams"].([]any); ok {
		for _, e := range raw {
			teams = append(teams, e.(string))
		}
	}

	health, _ := ClassifyConfigHealth(token, teams, settings)
	assert.Equal(t, HealthNeedsWizard, health)
}

func TestGenerateAnnotatedConfig_ActiveKeysMatchDefaults(t *testing.T) {
	out := GenerateAnnotatedConfig(nil)

	var parsed map[string]any
	assert.NoError(t, yaml.Unmarshal(out, &parsed))

	// Every uncommented key with a default must carry exactly that default,
	// so a generated file never drifts from DefaultOptionalKeys.
	for _, key := range []string{"editor", "terminal", "cluster_login_command", "toolbox_mode", "chord_prefix", "rosa_boundary_command"} {
		val, ok := parsed[key]
		assert.True(t, ok, "generated config must include %s", key)
		assert.Equal(t, DefaultOptionalKeys[key], val, "%s must match the code default", key)
	}
	assert.False(t, strings.Contains(string(out), "NAME HERE"), "no placeholder junk")
}

func TestGenerateAnnotatedConfig_WithEnvironment(t *testing.T) {
	env := &GenerateEnvironment{
		Terminals:        []string{"ptyxis", "gnome-terminal", "kitty"},
		Editor:           "nano",
		AgentCLI:         "claude --print",
		ClusterLoginCmds: []string{"ocm backplane login %%CLUSTER_ID%%", "ocm-container --cluster-id %%CLUSTER_ID%%"},
	}
	out := string(GenerateAnnotatedConfig(env))

	assert.Contains(t, out, "terminal: ptyxis", "first detected terminal is active")
	assert.Contains(t, out, "# terminal: gnome-terminal", "alternatives listed as comments")
	assert.Contains(t, out, "# terminal: kitty")
	assert.Contains(t, out, "editor: nano", "detected editor used")
	assert.Contains(t, out, "agent_cli_command: claude --print", "detected agent uncommented")
	assert.NotContains(t, out, "# agent_cli_command:", "agent should not be commented when detected")
	assert.Contains(t, out, "cluster_login_command: ocm backplane login", "first cluster login active")
	assert.Contains(t, out, "# cluster_login_command: ocm-container", "alternative listed as comment")
}

func TestGenerateAnnotatedConfig_WithEnvironment_ValidYAML(t *testing.T) {
	env := &GenerateEnvironment{
		Terminals: []string{"kitty", "foot"},
		Editor:    "emacs",
	}
	out := GenerateAnnotatedConfig(env)

	var parsed map[string]any
	assert.NoError(t, yaml.Unmarshal(out, &parsed), "env-aware config must still be valid YAML")
	assert.Equal(t, "kitty", parsed["terminal"])
	assert.Equal(t, "emacs", parsed["editor"])
}
