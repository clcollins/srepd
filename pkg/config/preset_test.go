package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const validPreset = `
teams:
  - PTEAM99
default_silent_escalation_policy: PSILENT9
custom_service_escalation_policies:
  PSVC1: PPOL1
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%
terminal: kitty
editor: nano
`

// A preset is a team-published YAML fragment of policy decisions — the
// "have to know to know" values. It pre-seeds the wizard; every value is
// still confirmed by the user.

func TestParsePreset_Valid(t *testing.T) {
	p, err := ParsePreset([]byte(validPreset), "team-docs.yaml")

	assert.NoError(t, err)
	assert.Equal(t, []string{"PTEAM99"}, p.Teams)
	assert.Equal(t, "PSILENT9", p.SilentPolicy)
	assert.Equal(t, map[string]string{"PSVC1": "PPOL1"}, p.CustomPolicies)
	assert.Equal(t, "ocm-container --cluster-id %%CLUSTER_ID%%", p.ClusterLoginCommand)
	assert.Equal(t, "kitty", p.Terminal)
	assert.Equal(t, "nano", p.Editor)
	assert.Equal(t, "team-docs.yaml", p.Source)
}

// Secrets and personal credentials must never ride in on a preset.
func TestParsePreset_RejectsToken(t *testing.T) {
	_, err := ParsePreset([]byte("token: stolen\nteams: [T1]\n"), "x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

func TestParsePreset_RejectsLLMAPI(t *testing.T) {
	_, err := ParsePreset([]byte("llm_api:\n  api_key_env: SECRET\n"), "x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm_api")
}

// Strict allowlist: anything unrecognized is rejected loudly rather than
// silently ignored — a typoed key in a team preset should not pass review.
func TestParsePreset_RejectsUnknownKeys(t *testing.T) {
	_, err := ParsePreset([]byte("teams: [T1]\nsurprise_key: value\n"), "x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "surprise_key")
}

func TestParsePreset_InvalidYAML(t *testing.T) {
	_, err := ParsePreset([]byte("{{nope"), "x")
	assert.Error(t, err)
}

// ApplyPreset overlays preset values only where the existing config is
// empty: the user's real config always wins over team defaults.
func TestApplyPreset_FillsEmptyFieldsOnly(t *testing.T) {
	p, err := ParsePreset([]byte(validPreset), "team-docs.yaml")
	assert.NoError(t, err)

	existing := ExistingConfig{
		Token:  "usertoken",
		Teams:  []string{"PMYTEAM"}, // user already picked a team
		Editor: "emacs",             // and an editor
	}

	merged, applied := ApplyPreset(existing, p)

	assert.Equal(t, []string{"PMYTEAM"}, merged.Teams, "existing teams win")
	assert.False(t, applied.Teams)
	assert.Equal(t, "emacs", merged.Editor, "existing editor wins")
	assert.False(t, applied.Editor)

	assert.Equal(t, "PSILENT9", merged.SilentPolicy, "empty fields take preset values")
	assert.True(t, applied.Silent)
	assert.Equal(t, map[string]string{"PSVC1": "PPOL1"}, merged.CustomPolicies)
	assert.True(t, applied.Custom)
	assert.Equal(t, "kitty", merged.Terminal)
	assert.True(t, applied.Terminal)
	assert.Equal(t, "ocm-container --cluster-id %%CLUSTER_ID%%", merged.ClusterLoginCommand)
	assert.True(t, applied.ClusterLogin)
	assert.Equal(t, "team-docs.yaml", applied.Source)
}

func TestApplyPreset_NilPresetNoOp(t *testing.T) {
	existing := ExistingConfig{Token: "t"}
	merged, applied := ApplyPreset(existing, nil)
	assert.Equal(t, existing, merged)
	assert.False(t, applied.Any())
}

func TestLoadPreset_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preset.yaml")
	assert.NoError(t, os.WriteFile(path, []byte(validPreset), 0644))

	p, err := LoadPreset(path, nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{"PTEAM99"}, p.Teams)
	assert.Equal(t, path, p.Source)
}

func TestLoadPreset_HTTPSOnly(t *testing.T) {
	_, err := LoadPreset("http://example.com/preset.yaml", http.DefaultClient)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestLoadPreset_URLFetch(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, validPreset) //nolint:errcheck
	}))
	defer ts.Close()

	p, err := LoadPreset(ts.URL, ts.Client())
	assert.NoError(t, err)
	assert.Equal(t, []string{"PTEAM99"}, p.Teams)
	assert.Equal(t, ts.URL, p.Source)
}

func TestLoadPreset_URLSizeCap(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "teams: [T1]\n# %s\n", strings.Repeat("x", presetMaxBytes)) //nolint:errcheck
	}))
	defer ts.Close()

	_, err := LoadPreset(ts.URL, ts.Client())
	assert.Error(t, err, "oversized presets must be rejected")
}

func TestLoadPreset_URLNon200(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	_, err := LoadPreset(ts.URL, ts.Client())
	assert.Error(t, err)
}

// Preset-applied fields must be persisted even when they equal the seeded
// "existing" values (the file doesn't actually contain them yet).
func TestForcePresetChanges(t *testing.T) {
	changes := ConfigChanges{}
	applied := PresetApplied{Teams: true, Silent: true, Custom: true, Terminal: true, Editor: true}

	forced := ForcePresetChanges(changes, applied)

	assert.True(t, forced.TeamsChanged)
	assert.True(t, forced.SilentChanged)
	assert.True(t, forced.CustomChanged)
	assert.True(t, forced.TerminalChanged)
	assert.True(t, forced.EditorChanged)
	assert.False(t, forced.TokenChanged, "token is never preset-driven")
}
