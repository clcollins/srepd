package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// presetMaxBytes caps fetched preset size: a team preset is a handful of
// IDs, never megabytes.
const presetMaxBytes = 64 * 1024

// Preset is a team-published YAML fragment carrying the "have to know to
// know" policy decisions (custom service→policy mappings, org cluster login
// command, the team's silent policy) so new SREs inherit them instead of
// hunting IDs. A preset only pre-seeds the wizard — every value is still
// shown to and confirmed by the user, and it can never carry credentials.
type Preset struct {
	Teams               []string
	SilentPolicy        string
	CustomPolicies      map[string]string
	ClusterLoginCommand string
	Terminal            string
	Editor              string
	Source              string
}

// PresetApplied records which fields a preset actually seeded, so the
// wizard can tag them and force them into the write set.
type PresetApplied struct {
	Teams        bool
	Silent       bool
	Custom       bool
	ClusterLogin bool
	Terminal     bool
	Editor       bool
	Source       string
}

func (p PresetApplied) Any() bool {
	return p.Teams || p.Silent || p.Custom || p.ClusterLogin || p.Terminal || p.Editor
}

// ExecutableAny reports whether the preset seeded any field that maps to a
// command srepd executes (terminal, editor, cluster login). These get an
// extra bold-red safety confirmation in the wizard: a preset fetched from a
// URL is remote input, and a malicious one could otherwise plant arbitrary
// commands. Team/policy IDs are only ever sent to the PagerDuty API, so
// they are excluded.
func (p PresetApplied) ExecutableAny() bool {
	return p.ClusterLogin || p.Terminal || p.Editor
}

// presetAllowedKeys is the strict allowlist of keys a preset may set.
// Everything else — above all token and llm_api credentials — is rejected
// loudly rather than silently ignored.
//
// SECURITY: if a key that maps to an executed binary or an agentic prompt
// (e.g. agent_cli_command, prompt templates) is ever added here, it MUST
// also be covered by PresetApplied.ExecutableAny so the wizard's bold-red
// preset safety confirmation includes it.
var presetAllowedKeys = map[string]bool{
	"teams":                              true,
	"default_silent_escalation_policy":   true,
	"custom_service_escalation_policies": true,
	"cluster_login_command":              true,
	"terminal":                           true,
	"editor":                             true,
}

// ParsePreset parses and validates a preset document. source is recorded for
// display ("(from preset: <source>)").
func ParsePreset(data []byte, source string) (*Preset, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid preset YAML: %w", err)
	}

	var rejected []string
	for key := range raw {
		if !presetAllowedKeys[key] {
			rejected = append(rejected, key)
		}
	}
	if len(rejected) > 0 {
		sort.Strings(rejected)
		return nil, fmt.Errorf("preset may not set %s — presets carry team policy only, never credentials or personal settings", strings.Join(rejected, ", "))
	}

	p := &Preset{Source: source}
	if teams, ok := raw["teams"].([]any); ok {
		for _, t := range teams {
			p.Teams = append(p.Teams, fmt.Sprintf("%v", t))
		}
	}
	if v, ok := raw["default_silent_escalation_policy"].(string); ok {
		p.SilentPolicy = v
	}
	if mappings, ok := raw["custom_service_escalation_policies"].(map[string]any); ok {
		p.CustomPolicies = make(map[string]string, len(mappings))
		for k, v := range mappings {
			p.CustomPolicies[k] = fmt.Sprintf("%v", v)
		}
	}
	if v, ok := raw["cluster_login_command"].(string); ok {
		p.ClusterLoginCommand = v
	}
	if v, ok := raw["terminal"].(string); ok {
		p.Terminal = v
	}
	if v, ok := raw["editor"].(string); ok {
		p.Editor = v
	}
	return p, nil
}

// LoadPreset loads a preset from a local file path or an HTTPS URL.
// URL fetches are HTTPS-only, size-capped, and never automatic — the user
// passed --preset explicitly. client is injectable for tests; nil means
// http.DefaultClient.
func LoadPreset(ref string, client *http.Client) (*Preset, error) {
	if strings.Contains(ref, "://") {
		return fetchPresetURL(ref, client)
	}

	data, err := os.ReadFile(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to read preset file: %w", err)
	}
	return ParsePreset(data, ref)
}

func fetchPresetURL(url string, client *http.Client) (*Preset, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("preset URLs must use https, got %q", url)
	}
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch preset: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch preset: %s returned %s", url, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, presetMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read preset: %w", err)
	}
	if len(data) > presetMaxBytes {
		return nil, fmt.Errorf("preset exceeds the %d byte limit", presetMaxBytes)
	}

	return ParsePreset(data, url)
}

// ApplyPreset overlays preset values onto existing where existing is empty:
// the user's real config always wins over team defaults. Returns the merged
// config and which fields the preset seeded.
func ApplyPreset(existing ExistingConfig, p *Preset) (ExistingConfig, PresetApplied) {
	if p == nil {
		return existing, PresetApplied{}
	}

	applied := PresetApplied{Source: p.Source}

	if HasPlaceholderTeams(existing.Teams) && len(p.Teams) > 0 {
		existing.Teams = p.Teams
		applied.Teams = true
	}
	if existing.SilentPolicy == "" && p.SilentPolicy != "" {
		existing.SilentPolicy = p.SilentPolicy
		applied.Silent = true
	}
	if len(existing.CustomPolicies) == 0 && len(p.CustomPolicies) > 0 {
		existing.CustomPolicies = p.CustomPolicies
		applied.Custom = true
	}
	if existing.ClusterLoginCommand == "" && p.ClusterLoginCommand != "" {
		existing.ClusterLoginCommand = p.ClusterLoginCommand
		applied.ClusterLogin = true
	}
	if existing.Terminal == "" && p.Terminal != "" {
		existing.Terminal = p.Terminal
		applied.Terminal = true
	}
	if existing.Editor == "" && p.Editor != "" {
		existing.Editor = p.Editor
		applied.Editor = true
	}

	return existing, applied
}

// ForcePresetChanges marks preset-seeded fields as changed so they are
// persisted even though they equal the seeded "existing" values (the config
// file does not actually contain them yet).
func ForcePresetChanges(changes ConfigChanges, applied PresetApplied) ConfigChanges {
	if applied.Teams {
		changes.TeamsChanged = true
	}
	if applied.Silent {
		changes.SilentChanged = true
	}
	if applied.Custom {
		changes.CustomChanged = true
	}
	if applied.ClusterLogin {
		changes.ClusterLoginChanged = true
	}
	if applied.Terminal {
		changes.TerminalChanged = true
	}
	if applied.Editor {
		changes.EditorChanged = true
	}
	return changes
}
