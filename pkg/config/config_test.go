package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// mockFS is a fully in-memory mock of the ConfigFS interface.
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

func TestMaskToken(t *testing.T) {
	assert.Equal(t, "PCGX****", MaskToken("PCGXUDY1"))
	assert.Equal(t, "****", MaskToken("ab"))
	assert.Equal(t, "****", MaskToken(""))
}

func TestStringSlicesEqual(t *testing.T) {
	assert.True(t, StringSlicesEqual([]string{"a", "b"}, []string{"b", "a"}))
	assert.False(t, StringSlicesEqual([]string{"a"}, []string{"a", "b"}))
	assert.True(t, StringSlicesEqual([]string{}, []string{}))
}

func TestFormatCustomMappings(t *testing.T) {
	assert.Equal(t, "", FormatCustomMappings(nil))
	result := FormatCustomMappings(map[string]string{"SVC1": "POL1"})
	assert.Contains(t, result, "SVC1:POL1")
}

func TestHasPlaceholderTeams_Empty(t *testing.T) {
	assert.True(t, HasPlaceholderTeams([]string{}))
}

func TestHasPlaceholderTeams_Nil(t *testing.T) {
	assert.True(t, HasPlaceholderTeams(nil))
}

func TestHasPlaceholderTeams_AllPlaceholders(t *testing.T) {
	teams := []string{"<PagerDuty Team ID 1>", "<PagerDuty Team ID 2>"}
	assert.True(t, HasPlaceholderTeams(teams))
}

func TestHasPlaceholderTeams_OneReal(t *testing.T) {
	teams := []string{"P1ABC23"}
	assert.False(t, HasPlaceholderTeams(teams))
}

func TestHasPlaceholderTeams_MixedRealAndPlaceholder(t *testing.T) {
	teams := []string{"<PagerDuty Team ID 1>", "P1ABC23"}
	assert.False(t, HasPlaceholderTeams(teams))
}

func TestHasPlaceholderTeams_WhitespaceOnly(t *testing.T) {
	teams := []string{"  ", ""}
	assert.True(t, HasPlaceholderTeams(teams))
}

func TestUpdateTeamsInConfig_ReplacesPlaceholders(t *testing.T) {
	input := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
  - <PagerDuty Team ID 2>
editor: vim
`)
	result, err := UpdateTeamsInConfig(input, []string{"P1ABC23", "P4DEF56"}, map[string]string{"P1ABC23": "Team Alpha", "P4DEF56": "Team Beta"})

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
	result, err := UpdateTeamsInConfig(input, []string{"PASPK4G"}, map[string]string{"PASPK4G": "Platform SRE"})

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
	result, err := UpdateTeamsInConfig(input, []string{"PREAL1"}, map[string]string{"PREAL1": "Real Team"})

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
	result, err := UpdateTeamsInConfig(input, []string{}, nil)

	require.NoError(t, err)
	assert.Contains(t, string(result), "teams: []")
}

func TestUpdateTeamsInConfig_NoTeamsKey(t *testing.T) {
	input := []byte(`token: my-token
editor: vim
`)
	result, err := UpdateTeamsInConfig(input, []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "- P1ABC23")
	assert.Contains(t, string(result), "token: my-token")
	assert.Contains(t, string(result), "editor: vim")
}

func TestWriteConfigTeams_Success(t *testing.T) {
	configData := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
`)
	m := &mockFS{readData: configData}

	err := WriteConfigTeams(m, "/fake/home", []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

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

	err := WriteConfigTeams(m, "/fake/home", []string{"NEW_TEAM"}, map[string]string{"NEW_TEAM": "New"})

	require.NoError(t, err)
	assert.Equal(t, string(configData), string(m.backupData))
}

func TestWriteConfigTeams_ReadError(t *testing.T) {
	m := &mockFS{readErr: errors.New("no such file")}

	err := WriteConfigTeams(m, "/fake/home", []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestWriteConfigTeams_WriteError(t *testing.T) {
	configData := []byte(`token: my-token
teams:
  - <PagerDuty Team ID 1>
`)
	m := &mockFS{readData: configData, writeErr: errors.New("disk full")}

	err := WriteConfigTeams(m, "/fake/home", []string{"P1ABC23"}, map[string]string{"P1ABC23": "Team Alpha"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

// --- ParseCustomMappings tests ---

func TestParseCustomMappings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{"single pair", "P5LAB5Y:PVBANNN", map[string]string{"P5LAB5Y": "PVBANNN"}},
		{"multiple pairs", "SVC1:POL1,SVC2:POL2", map[string]string{"SVC1": "POL1", "SVC2": "POL2"}},
		{"with spaces", " SVC1 : POL1 , SVC2 : POL2 ", map[string]string{"SVC1": "POL1", "SVC2": "POL2"}},
		{"empty string", "", map[string]string{}},
		{"malformed skipped", "SVC1POL1,SVC2:POL2", map[string]string{"SVC2": "POL2"}},
		{"empty key skipped", ":POL1", map[string]string{}},
		{"empty value skipped", "SVC1:", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCustomMappings(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- UpsertScalarInConfig tests ---

func TestUpsertScalarInConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		key      string
		value    string
		contains []string
		wantErr  bool
	}{
		{
			name:     "update existing key",
			input:    "token: old-token\neditor: vim\n",
			key:      "token",
			value:    "new-token",
			contains: []string{"token: new-token", "editor: vim"},
		},
		{
			name:     "append new key",
			input:    "token: my-token\n",
			key:      "editor",
			value:    "vim",
			contains: []string{"token: my-token", "editor: vim"},
		},
		{
			name:     "preserves other keys",
			input:    "token: my-token\nteams:\n  - TEAM1\neditor: vim\n",
			key:      "editor",
			value:    "nano",
			contains: []string{"token: my-token", "editor: nano", "- TEAM1"},
		},
		{
			name:     "preserves comments",
			input:    "# Main config\ntoken: my-token\n# Editor setting\neditor: vim\n",
			key:      "editor",
			value:    "nano",
			contains: []string{"# Main config", "# Editor setting", "editor: nano"},
		},
		{
			name:    "invalid YAML",
			input:   "\t\x00invalid",
			key:     "token",
			value:   "val",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UpsertScalarInConfig([]byte(tt.input), tt.key, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, s := range tt.contains {
				assert.Contains(t, string(result), s)
			}
		})
	}
}

// --- UpsertMapInConfig tests ---

func TestUpsertMapInConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		key      string
		values   map[string]string
		contains []string
		wantErr  bool
	}{
		{
			name:     "update existing map",
			input:    "token: my-token\ncustom_policies:\n  SVC1: POL1\n",
			key:      "custom_policies",
			values:   map[string]string{"SVC2": "POL2"},
			contains: []string{"token: my-token", "SVC2: POL2"},
		},
		{
			name:     "append new map",
			input:    "token: my-token\n",
			key:      "custom_policies",
			values:   map[string]string{"SVC1": "POL1"},
			contains: []string{"token: my-token", "custom_policies:", "SVC1: POL1"},
		},
		{
			name:     "preserves other keys",
			input:    "token: my-token\neditor: vim\n",
			key:      "custom_policies",
			values:   map[string]string{"SVC1": "POL1"},
			contains: []string{"token: my-token", "editor: vim", "SVC1: POL1"},
		},
		{
			name:    "invalid YAML",
			input:   "\t\x00invalid",
			key:     "custom_policies",
			values:  map[string]string{"SVC1": "POL1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UpsertMapInConfig([]byte(tt.input), tt.key, tt.values)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, s := range tt.contains {
				assert.Contains(t, string(result), s)
			}
		})
	}
}

// --- WriteConfigKey tests ---

func TestWriteConfigKey_Success(t *testing.T) {
	configData := []byte("token: old-token\neditor: vim\n")
	m := &mockFS{readData: configData}

	err := WriteConfigKey(m, "/fake/home", "token", "new-token")

	require.NoError(t, err)
	assert.Contains(t, string(m.writeData), "token: new-token")
	assert.Contains(t, string(m.writeData), "editor: vim")
}

func TestWriteConfigKey_CreatesBackup(t *testing.T) {
	configData := []byte("token: old-token\n")
	m := &mockFS{readData: configData}

	err := WriteConfigKey(m, "/fake/home", "token", "new-token")

	require.NoError(t, err)
	assert.Equal(t, string(configData), string(m.backupData))
}

func TestWriteConfigKey_ReadError(t *testing.T) {
	m := &mockFS{readErr: errors.New("no such file")}

	err := WriteConfigKey(m, "/fake/home", "token", "new-token")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

// --- WriteConfigMap tests ---

func TestWriteConfigMap_Success(t *testing.T) {
	configData := []byte("token: my-token\n")
	m := &mockFS{readData: configData}

	err := WriteConfigMap(m, "/fake/home", "custom_policies", map[string]string{"SVC1": "POL1"})

	require.NoError(t, err)
	assert.Contains(t, string(m.writeData), "token: my-token")
	assert.Contains(t, string(m.writeData), "SVC1: POL1")
}

func TestWriteConfigMap_CreatesBackup(t *testing.T) {
	configData := []byte("token: my-token\n")
	m := &mockFS{readData: configData}

	err := WriteConfigMap(m, "/fake/home", "custom_policies", map[string]string{"SVC1": "POL1"})

	require.NoError(t, err)
	assert.Equal(t, string(configData), string(m.backupData))
}

func TestWriteConfigMap_ReadError(t *testing.T) {
	m := &mockFS{readErr: errors.New("no such file")}

	err := WriteConfigMap(m, "/fake/home", "custom_policies", map[string]string{"SVC1": "POL1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

// --- ResolveExistingConfig tests ---

func TestResolveExistingConfig(t *testing.T) {
	tests := []struct {
		name              string
		token             string
		teams             []string
		silentPolicy      string
		customPolicies    map[string]string
		customPoliciesRaw string
		oldPolicies       map[string]string
		expected          ExistingConfig
	}{
		{
			name:           "all new format populated",
			token:          "my-token",
			teams:          []string{"TEAM1"},
			silentPolicy:   "PCGXUDY",
			customPolicies: map[string]string{"P5LAB5Y": "PVBANNN"},
			expected: ExistingConfig{
				Token:          "my-token",
				Teams:          []string{"TEAM1"},
				SilentPolicy:   "PCGXUDY",
				CustomPolicies: map[string]string{"P5LAB5Y": "PVBANNN"},
			},
		},
		{
			name:              "custom from env var string",
			token:             "my-token",
			teams:             []string{"TEAM1"},
			silentPolicy:      "PCGXUDY",
			customPolicies:    map[string]string{},
			customPoliciesRaw: "P5LAB5Y:PVBANNN",
			expected: ExistingConfig{
				Token:          "my-token",
				Teams:          []string{"TEAM1"},
				SilentPolicy:   "PCGXUDY",
				CustomPolicies: map[string]string{"P5LAB5Y": "PVBANNN"},
			},
		},
		{
			name:        "silent from old format",
			token:       "my-token",
			teams:       []string{"TEAM1"},
			oldPolicies: map[string]string{"silent_default": "PCGXUDY", "default": "PA4586M"},
			expected: ExistingConfig{
				Token:        "my-token",
				Teams:        []string{"TEAM1"},
				SilentPolicy: "PCGXUDY",
			},
		},
		{
			name:        "custom from old format with uppercase conversion",
			token:       "my-token",
			teams:       []string{"TEAM1"},
			oldPolicies: map[string]string{"silent_default": "PCGXUDY", "default": "PA4586M", "p5lab5y": "PVBANNN"},
			expected: ExistingConfig{
				Token:          "my-token",
				Teams:          []string{"TEAM1"},
				SilentPolicy:   "PCGXUDY",
				CustomPolicies: map[string]string{"P5LAB5Y": "PVBANNN"},
			},
		},
		{
			name:        "old format skips reserved keys",
			token:       "my-token",
			teams:       []string{"TEAM1"},
			oldPolicies: map[string]string{"default": "PA4586M", "silent_default": "PCGXUDY"},
			expected: ExistingConfig{
				Token:        "my-token",
				Teams:        []string{"TEAM1"},
				SilentPolicy: "PCGXUDY",
			},
		},
		{
			name:           "new format takes precedence over old",
			token:          "my-token",
			teams:          []string{"TEAM1"},
			silentPolicy:   "NEW_SILENT",
			customPolicies: map[string]string{"SVC1": "POL1"},
			oldPolicies:    map[string]string{"silent_default": "OLD_SILENT", "svc2": "POL2"},
			expected: ExistingConfig{
				Token:          "my-token",
				Teams:          []string{"TEAM1"},
				SilentPolicy:   "NEW_SILENT",
				CustomPolicies: map[string]string{"SVC1": "POL1"},
			},
		},
		{
			name: "all empty",
			expected: ExistingConfig{
				Token: "",
				Teams: nil,
			},
		},
		{
			name:              "multi-pair env var",
			token:             "my-token",
			teams:             []string{"TEAM1"},
			silentPolicy:      "PCGXUDY",
			customPolicies:    map[string]string{},
			customPoliciesRaw: "SVC1:POL1, SVC2:POL2",
			expected: ExistingConfig{
				Token:          "my-token",
				Teams:          []string{"TEAM1"},
				SilentPolicy:   "PCGXUDY",
				CustomPolicies: map[string]string{"SVC1": "POL1", "SVC2": "POL2"},
			},
		},
		{
			name:         "silent from new but custom from old",
			token:        "my-token",
			teams:        []string{"TEAM1"},
			silentPolicy: "NEW_SILENT",
			oldPolicies:  map[string]string{"default": "PA4586M", "silent_default": "OLD_SILENT", "p5lab5y": "PVBANNN"},
			expected: ExistingConfig{
				Token:          "my-token",
				Teams:          []string{"TEAM1"},
				SilentPolicy:   "NEW_SILENT",
				CustomPolicies: map[string]string{"P5LAB5Y": "PVBANNN"},
			},
		},
		{
			name:           "new format with lowercase keys from Viper uppercased",
			token:          "my-token",
			teams:          []string{"TEAM1"},
			silentPolicy:   "PCGXUDY",
			customPolicies: map[string]string{"p5lab5y": "PVBANNN"},
			expected: ExistingConfig{
				Token:          "my-token",
				Teams:          []string{"TEAM1"},
				SilentPolicy:   "PCGXUDY",
				CustomPolicies: map[string]string{"P5LAB5Y": "PVBANNN"},
			},
		},
		{
			name:        "old format with only reserved keys yields no custom",
			token:       "my-token",
			teams:       []string{"TEAM1"},
			oldPolicies: map[string]string{"default": "PA4586M", "silent_default": "PCGXUDY"},
			expected: ExistingConfig{
				Token:        "my-token",
				Teams:        []string{"TEAM1"},
				SilentPolicy: "PCGXUDY",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveExistingConfig(
				tt.token, tt.teams, tt.silentPolicy,
				tt.customPolicies, tt.customPoliciesRaw, tt.oldPolicies,
			)
			assert.Equal(t, tt.expected.Token, result.Token)
			assert.Equal(t, tt.expected.Teams, result.Teams)
			assert.Equal(t, tt.expected.SilentPolicy, result.SilentPolicy)
			if tt.expected.CustomPolicies == nil {
				assert.Empty(t, result.CustomPolicies)
			} else {
				assert.Equal(t, tt.expected.CustomPolicies, result.CustomPolicies)
			}
		})
	}
}

func TestResolveExistingConfig_OldFormatSetsFlag(t *testing.T) {
	result := ResolveExistingConfig(
		"my-token", []string{"TEAM1"}, "",
		map[string]string{}, "",
		map[string]string{"silent_default": "PCGXUDY", "default": "PA4586M", "p5lab5y": "PVBANNN"},
	)

	assert.True(t, result.OldFormatDetected)
	assert.Equal(t, "PCGXUDY", result.SilentPolicy)
	assert.Equal(t, map[string]string{"P5LAB5Y": "PVBANNN"}, result.CustomPolicies)
}

func TestResolveExistingConfig_NewFormatNoFlag(t *testing.T) {
	result := ResolveExistingConfig(
		"my-token", []string{"TEAM1"}, "PCGXUDY",
		map[string]string{"P5LAB5Y": "PVBANNN"}, "",
		map[string]string{},
	)

	assert.False(t, result.OldFormatDetected)
}

// --- ResolveKeepDefaults tests ---

func TestResolveKeepDefaults(t *testing.T) {
	tests := []struct {
		name           string
		teams          []string
		silentPolicy   string
		customPolicies map[string]string
		expected       KeepDefaults
	}{
		{
			name:           "all populated",
			teams:          []string{"TEAM1"},
			silentPolicy:   "PCGXUDY",
			customPolicies: map[string]string{"SVC1": "POL1"},
			expected:       KeepDefaults{HasValidTeams: true, KeepTeams: true, HasSilent: true, KeepSilent: true, HasCustom: true, KeepCustom: true},
		},
		{
			name:     "placeholder teams",
			teams:    []string{"<PagerDuty Team ID 1>"},
			expected: KeepDefaults{},
		},
		{
			name:     "empty teams",
			teams:    []string{},
			expected: KeepDefaults{},
		},
		{
			name:         "no silent policy",
			teams:        []string{"TEAM1"},
			silentPolicy: "",
			expected:     KeepDefaults{HasValidTeams: true, KeepTeams: true},
		},
		{
			name:           "no custom mappings",
			teams:          []string{"TEAM1"},
			silentPolicy:   "PCGXUDY",
			customPolicies: nil,
			expected:       KeepDefaults{HasValidTeams: true, KeepTeams: true, HasSilent: true, KeepSilent: true},
		},
		{
			name:           "partial config",
			teams:          []string{"TEAM1"},
			customPolicies: map[string]string{"SVC1": "POL1"},
			expected:       KeepDefaults{HasValidTeams: true, KeepTeams: true, HasCustom: true, KeepCustom: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveKeepDefaults(tt.teams, tt.silentPolicy, tt.customPolicies)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- ResolveFinalValues tests ---

func TestResolveFinalValues(t *testing.T) {
	baseExisting := ExistingConfig{
		Token:          "existing-token",
		Teams:          []string{"TEAM1", "TEAM2"},
		SilentPolicy:   "PCGXUDY",
		CustomPolicies: map[string]string{"SVC1": "POL1"},
	}

	tests := []struct {
		name     string
		existing ExistingConfig
		inputs   WizardInputs
		wantErr  string
		check    func(t *testing.T, rv ResolvedValues)
	}{
		{
			name:     "new token overrides existing",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: "new-token"},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, "new-token", rv.Token)
			},
		},
		{
			name:     "empty input keeps existing token",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: ""},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, "existing-token", rv.Token)
			},
		},
		{
			name:     "no token at all returns error",
			existing: ExistingConfig{},
			inputs:   WizardInputs{TokenInput: ""},
			wantErr:  "token is required",
		},
		{
			name:     "token whitespace trimmed",
			existing: ExistingConfig{},
			inputs:   WizardInputs{TokenInput: "  my-token  "},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, "my-token", rv.Token)
			},
		},
		{
			name:     "keepTeams true uses existing",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: "t", KeepTeams: true, SelectedTeams: []string{"NEW_TEAM"}},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, []string{"TEAM1", "TEAM2"}, rv.Teams)
			},
		},
		{
			name:     "keepTeams false uses selected",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: "t", KeepTeams: false, SelectedTeams: []string{"NEW_TEAM"}},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, []string{"NEW_TEAM"}, rv.Teams)
			},
		},
		{
			name:     "keepSilent true uses existing",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: "t", KeepSilent: true, SilentPolicyID: "NEW_POLICY"},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, "PCGXUDY", rv.SilentPolicy)
			},
		},
		{
			name:     "keepSilent false uses input",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: "t", KeepSilent: false, SilentPolicyID: " NEW_POLICY "},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, "NEW_POLICY", rv.SilentPolicy)
			},
		},
		{
			name:     "keepCustom true uses formatted existing",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: "t", KeepCustom: true, CustomMappingsInput: "IGNORED"},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Contains(t, rv.CustomMappingsInput, "SVC1:POL1")
			},
		},
		{
			name:     "keepCustom false uses input",
			existing: baseExisting,
			inputs:   WizardInputs{TokenInput: "t", KeepCustom: false, CustomMappingsInput: " SVC2:POL2 "},
			check: func(t *testing.T, rv ResolvedValues) {
				assert.Equal(t, "SVC2:POL2", rv.CustomMappingsInput)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveFinalValues(tt.existing, tt.inputs)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, result)
		})
	}
}

// --- DetectChanges tests ---

func TestDetectChanges(t *testing.T) {
	existing := ExistingConfig{
		Token:          "my-token",
		Teams:          []string{"TEAM1", "TEAM2"},
		SilentPolicy:   "PCGXUDY",
		CustomPolicies: map[string]string{"SVC1": "POL1"},
	}

	tests := []struct {
		name       string
		existing   ExistingConfig
		final      ResolvedValues
		tokenInput string
		expected   ConfigChanges
	}{
		{
			name:     "nothing changed",
			existing: existing,
			final: ResolvedValues{
				Token: "my-token", Teams: []string{"TEAM1", "TEAM2"},
				SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC1:POL1",
			},
			tokenInput: "",
			expected:   ConfigChanges{},
		},
		{
			name:     "token changed",
			existing: existing,
			final: ResolvedValues{
				Token: "new-token", Teams: []string{"TEAM1", "TEAM2"},
				SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC1:POL1",
			},
			tokenInput: "new-token",
			expected:   ConfigChanges{TokenChanged: true},
		},
		{
			name:     "token input empty means kept",
			existing: existing,
			final: ResolvedValues{
				Token: "my-token", Teams: []string{"TEAM1", "TEAM2"},
				SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC1:POL1",
			},
			tokenInput: "",
			expected:   ConfigChanges{TokenChanged: false},
		},
		{
			name:     "token input same as existing",
			existing: existing,
			final: ResolvedValues{
				Token: "my-token", Teams: []string{"TEAM1", "TEAM2"},
				SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC1:POL1",
			},
			tokenInput: "my-token",
			expected:   ConfigChanges{TokenChanged: false},
		},
		{
			name:     "teams changed",
			existing: existing,
			final: ResolvedValues{
				Token: "my-token", Teams: []string{"TEAM3"},
				SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC1:POL1",
			},
			expected: ConfigChanges{TeamsChanged: true},
		},
		{
			name:     "teams reordered is not changed",
			existing: existing,
			final: ResolvedValues{
				Token: "my-token", Teams: []string{"TEAM2", "TEAM1"},
				SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC1:POL1",
			},
			expected: ConfigChanges{TeamsChanged: false},
		},
		{
			name:     "silent changed",
			existing: existing,
			final: ResolvedValues{
				Token: "my-token", Teams: []string{"TEAM1", "TEAM2"},
				SilentPolicy: "NEW_POLICY", CustomMappingsInput: "SVC1:POL1",
			},
			expected: ConfigChanges{SilentChanged: true},
		},
		{
			name:     "custom changed",
			existing: existing,
			final: ResolvedValues{
				Token: "my-token", Teams: []string{"TEAM1", "TEAM2"},
				SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC2:POL2",
			},
			expected: ConfigChanges{CustomChanged: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectChanges(tt.existing, tt.final, tt.tokenInput)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigChanges_AnyChanged(t *testing.T) {
	assert.False(t, ConfigChanges{}.AnyChanged())
	assert.True(t, ConfigChanges{TokenChanged: true}.AnyChanged())
	assert.True(t, ConfigChanges{TeamsChanged: true}.AnyChanged())
	assert.True(t, ConfigChanges{SilentChanged: true}.AnyChanged())
	assert.True(t, ConfigChanges{CustomChanged: true}.AnyChanged())
}

// --- ParseCustomMappingsStrict tests ---

func TestParseCustomMappingsStrict(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
		wantErr  string
	}{
		{"single pair", "SVC1:POL1", map[string]string{"SVC1": "POL1"}, ""},
		{"multiple pairs", "SVC1:POL1, SVC2:POL2", map[string]string{"SVC1": "POL1", "SVC2": "POL2"}, ""},
		{"whitespace trimmed", " SVC1 : POL1 ", map[string]string{"SVC1": "POL1"}, ""},
		{"missing colon", "SVC1POL1", nil, "invalid mapping"},
		{"empty service ID", ":POL1", nil, "empty service ID"},
		{"empty policy ID", "SVC1:", nil, "empty policy ID"},
		{"empty string", "", map[string]string{}, ""},
		{"extra colons in value", "SVC1:POL:EXTRA", map[string]string{"SVC1": "POL:EXTRA"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCustomMappingsStrict(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- new config file initialization tests ---

func TestUpdateTeamsInConfig_InitialEmptyTeams(t *testing.T) {
	input := []byte("---\ntoken: \"\"\nteams: []\n")
	result, err := UpdateTeamsInConfig(input, []string{"PASPK4G"}, map[string]string{"PASPK4G": "Platform SRE"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "- PASPK4G")
	assert.Contains(t, string(result), "# Platform SRE")
}

func TestUpsertScalarInConfig_InitialEmptyToken(t *testing.T) {
	input := []byte("---\ntoken: \"\"\nteams: []\n")
	result, err := UpsertScalarInConfig(input, "token", "my-new-token")

	require.NoError(t, err)
	assert.Contains(t, string(result), "my-new-token")
	assert.Contains(t, string(result), "teams:")
}

func TestWriteConfigTeams_NewConfigFile(t *testing.T) {
	configData := []byte("---\ntoken: \"\"\nteams: []\n")
	m := &mockFS{readData: configData}

	err := WriteConfigTeams(m, "/fake/home", []string{"TEAM1"}, map[string]string{"TEAM1": "My Team"})

	require.NoError(t, err)
	assert.Contains(t, string(m.writeData), "- TEAM1")
}

// --- bare YAML document tests (---\n only, no mapping) ---

func TestUpdateTeamsInConfig_BareYAMLDoc(t *testing.T) {
	input := []byte("---\n")
	result, err := UpdateTeamsInConfig(input, []string{"PASPK4G"}, map[string]string{"PASPK4G": "Platform SRE"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "- PASPK4G")
	assert.Contains(t, string(result), "# Platform SRE")
}

func TestUpdateTeamsInConfig_BareYAMLDocNoTeamsKey(t *testing.T) {
	input := []byte("---\ntoken: my-token\n")
	result, err := UpdateTeamsInConfig(input, []string{"TEAM1"}, map[string]string{"TEAM1": "My Team"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "- TEAM1")
	assert.Contains(t, string(result), "token: my-token")
}

func TestUpsertScalarInConfig_BareYAMLDoc(t *testing.T) {
	input := []byte("---\n")
	result, err := UpsertScalarInConfig(input, "token", "my-token")

	require.NoError(t, err)
	assert.Contains(t, string(result), "my-token")
}

func TestUpsertMapInConfig_BareYAMLDoc(t *testing.T) {
	input := []byte("---\n")
	result, err := UpsertMapInConfig(input, "custom_policies", map[string]string{"SVC1": "POL1"})

	require.NoError(t, err)
	assert.Contains(t, string(result), "SVC1: POL1")
}

// --- BuildSummary tests ---

func TestBuildSummary_AllChanged(t *testing.T) {
	existing := ExistingConfig{Token: "old-token", Teams: []string{"TEAM1"}, SilentPolicy: "OLD_POL", CustomPolicies: map[string]string{"SVC1": "POL1"}}
	final := ResolvedValues{Token: "new-token", Teams: []string{"TEAM2"}, SilentPolicy: "NEW_POL", CustomMappingsInput: "SVC2:POL2"}
	changes := ConfigChanges{TokenChanged: true, TeamsChanged: true, SilentChanged: true, CustomChanged: true}
	teamNames := map[string]string{"TEAM2": "Beta Team"}

	result := BuildSummary(existing, final, changes, teamNames, nil)

	assert.Contains(t, result, "new-")
	assert.Contains(t, result, "(changed)")
	assert.Contains(t, result, "Beta Team")
}

func TestBuildSummary_NothingChanged(t *testing.T) {
	existing := ExistingConfig{Token: "my-token", Teams: []string{"TEAM1"}, SilentPolicy: "POL1", CustomPolicies: map[string]string{"SVC1": "POL1"}}
	final := ResolvedValues{Token: "my-token", Teams: []string{"TEAM1"}, SilentPolicy: "POL1", CustomMappingsInput: "SVC1:POL1"}
	changes := ConfigChanges{}
	teamNames := map[string]string{"TEAM1": "Alpha"}

	result := BuildSummary(existing, final, changes, teamNames, nil)

	assert.NotContains(t, result, "(changed)")
	assert.Contains(t, result, "Token:")
	assert.Contains(t, result, "Teams:")
	assert.Contains(t, result, "Silent policy:")
	assert.Contains(t, result, "Custom:")
	lines := strings.Split(strings.TrimSpace(result), "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			assert.Contains(t, line, "(unchanged)", "every field should show (unchanged): %s", line)
		}
	}
}

func TestBuildSummary_CustomKeyUppercased(t *testing.T) {
	existing := ExistingConfig{Token: "tok", CustomPolicies: map[string]string{"svc1": "POL1"}}
	final := ResolvedValues{Token: "tok", Teams: []string{}, CustomMappingsInput: "SVC1:POL1"}
	changes := ConfigChanges{}

	result := BuildSummary(existing, final, changes, nil, nil)

	assert.Contains(t, result, "SVC1:POL1")
	assert.NotContains(t, result, "svc1")
}

func TestBuildSummary_TokenMasked(t *testing.T) {
	existing := ExistingConfig{Token: "longtoken123"}
	final := ResolvedValues{Token: "longtoken123", Teams: []string{}}
	changes := ConfigChanges{}

	result := BuildSummary(existing, final, changes, nil, nil)

	assert.Contains(t, result, "long********")
	assert.NotContains(t, result, "longtoken123")
}

// --- BuildFullConfig tests ---

func TestDefaultOptionalKeys_RosaBoundaryCommand(t *testing.T) {
	val, ok := DefaultOptionalKeys["rosa_boundary_command"]
	assert.True(t, ok, "rosa_boundary_command must be in DefaultOptionalKeys")
	assert.Contains(t, val, "%%CLUSTER_ID%%", "default must contain %%CLUSTER_ID%% placeholder")
	assert.Contains(t, val, "rosa-boundary", "default must reference the rosa-boundary CLI")
}

func TestBuildFullConfig_AllFields(t *testing.T) {
	final := ResolvedValues{
		Token: "test-token-value",
		Teams: []string{"PASPK4G", "P494195"},
	}
	teamNames := map[string]string{"PASPK4G": "Platform SRE", "P494195": "RHOBS Team"}
	customPolicies := map[string]string{"P5LAB5Y": "PVBANNN"}

	result := BuildFullConfig(final, teamNames, "PCGXUDY", customPolicies)
	output := string(result)

	assert.Contains(t, output, "token: test-token-value")
	assert.Contains(t, output, "- PASPK4G # Platform SRE")
	assert.Contains(t, output, "- P494195 # RHOBS Team")
	assert.Contains(t, output, "default_silent_escalation_policy: PCGXUDY")
	assert.Contains(t, output, "P5LAB5Y: PVBANNN")
	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "terminal: gnome-terminal --")
	assert.Contains(t, output, "cluster_login_command: ocm backplane login %%CLUSTER_ID%%")
	assert.Contains(t, output, "toolbox_mode: auto")
	assert.Contains(t, output, "chord_prefix: ctrl+x")
	assert.Contains(t, output, "rosa_boundary_command: rosa-boundary start-task --cluster-id %%CLUSTER_ID%% --connect")
}

func TestBuildFullConfig_MinimalNoOptionals(t *testing.T) {
	final := ResolvedValues{
		Token: "test-token-value",
		Teams: []string{"TEAM1"},
	}
	teamNames := map[string]string{"TEAM1": "My Team"}

	result := BuildFullConfig(final, teamNames, "", nil)
	output := string(result)

	assert.Contains(t, output, "token: test-token-value")
	assert.Contains(t, output, "- TEAM1 # My Team")
	assert.NotContains(t, output, "default_silent_escalation_policy")
	assert.NotContains(t, output, "custom_service_escalation_policies")
	assert.Contains(t, output, "editor: vim")
}

func TestBuildFullConfig_TeamNamesAsComments(t *testing.T) {
	final := ResolvedValues{
		Token: "tok",
		Teams: []string{"ABC123"},
	}
	teamNames := map[string]string{"ABC123": "Alpha Team"}

	result := BuildFullConfig(final, teamNames, "", nil)

	assert.Contains(t, string(result), "- ABC123 # Alpha Team")
}

func TestBuildFullConfig_TeamWithoutName(t *testing.T) {
	final := ResolvedValues{
		Token: "tok",
		Teams: []string{"UNKNOWN"},
	}

	result := BuildFullConfig(final, nil, "", nil)

	assert.Contains(t, string(result), "  - UNKNOWN\n")
	assert.NotContains(t, string(result), "UNKNOWN #")
}

func TestBuildFullConfig_IsValidYAML(t *testing.T) {
	final := ResolvedValues{
		Token: "test-token",
		Teams: []string{"TEAM1"},
	}

	result := BuildFullConfig(final, map[string]string{"TEAM1": "Team"}, "POL1", map[string]string{"SVC1": "POL2"})

	var parsed map[string]interface{}
	err := yaml.Unmarshal(result, &parsed)
	require.NoError(t, err, "output must be valid YAML")
	assert.Equal(t, "test-token", parsed["token"])
	teams, ok := parsed["teams"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, "TEAM1", teams[0])
	assert.Equal(t, "POL1", parsed["default_silent_escalation_policy"])
}

// --- MergeIntoExistingConfig tests ---

const existingFullConfig = `# Main config
token: existing-token
# Teams to filter on
teams:
  - TEAM1 # Alpha Team
editor: vim
terminal: ptyxis
cluster_login_command: ocm-container --cluster-id %%CLUSTER_ID%%
toolbox_mode: auto
log_to_journal: false
default_silent_escalation_policy: PCGXUDY
custom_service_escalation_policies:
  P5LAB5Y: PVBANNN
`

func TestMergeIntoExistingConfig_TokenChangedOnly(t *testing.T) {
	changes := ConfigChanges{TokenChanged: true}
	final := ResolvedValues{Token: "new-token", Teams: []string{"TEAM1"}, SilentPolicy: "PCGXUDY", CustomMappingsInput: "P5LAB5Y:PVBANNN"}

	result, err := MergeIntoExistingConfig([]byte(existingFullConfig), final, changes, nil, nil)

	require.NoError(t, err)
	output := string(result)
	assert.Contains(t, output, "new-token")
	assert.Contains(t, output, "- TEAM1")
	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "terminal: ptyxis")
	assert.Contains(t, output, "log_to_journal")
	assert.Contains(t, output, "# Main config")
}

func TestMergeIntoExistingConfig_TeamsChanged(t *testing.T) {
	changes := ConfigChanges{TeamsChanged: true}
	final := ResolvedValues{Token: "existing-token", Teams: []string{"TEAM2", "TEAM3"}, SilentPolicy: "PCGXUDY"}
	teamNames := map[string]string{"TEAM2": "Beta Team", "TEAM3": "Gamma Team"}

	result, err := MergeIntoExistingConfig([]byte(existingFullConfig), final, changes, teamNames, nil)

	require.NoError(t, err)
	output := string(result)
	assert.Contains(t, output, "- TEAM2 # Beta Team")
	assert.Contains(t, output, "- TEAM3 # Gamma Team")
	assert.NotContains(t, output, "TEAM1")
	assert.Contains(t, output, "editor: vim")
}

func TestMergeIntoExistingConfig_SilentChanged(t *testing.T) {
	changes := ConfigChanges{SilentChanged: true}
	final := ResolvedValues{Token: "existing-token", Teams: []string{"TEAM1"}, SilentPolicy: "NEW_POL"}

	result, err := MergeIntoExistingConfig([]byte(existingFullConfig), final, changes, nil, nil)

	require.NoError(t, err)
	output := string(result)
	assert.Contains(t, output, "default_silent_escalation_policy: NEW_POL")
	assert.NotContains(t, output, "PCGXUDY")
}

func TestMergeIntoExistingConfig_CustomChanged(t *testing.T) {
	changes := ConfigChanges{CustomChanged: true}
	final := ResolvedValues{Token: "existing-token", Teams: []string{"TEAM1"}, SilentPolicy: "PCGXUDY", CustomMappingsInput: "SVC2:POL2"}
	customPolicies := map[string]string{"SVC2": "POL2"}

	result, err := MergeIntoExistingConfig([]byte(existingFullConfig), final, changes, nil, customPolicies)

	require.NoError(t, err)
	output := string(result)
	assert.Contains(t, output, "SVC2: POL2")
}

func TestMergeIntoExistingConfig_NothingChanged(t *testing.T) {
	changes := ConfigChanges{}
	final := ResolvedValues{Token: "existing-token", Teams: []string{"TEAM1"}, SilentPolicy: "PCGXUDY"}

	result, err := MergeIntoExistingConfig([]byte(existingFullConfig), final, changes, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, existingFullConfig, string(result))
}

func TestMergeIntoExistingConfig_PreservesComments(t *testing.T) {
	changes := ConfigChanges{TokenChanged: true}
	final := ResolvedValues{Token: "new-token", Teams: []string{"TEAM1"}}

	result, err := MergeIntoExistingConfig([]byte(existingFullConfig), final, changes, nil, nil)

	require.NoError(t, err)
	output := string(result)
	assert.Contains(t, output, "# Main config")
	assert.Contains(t, output, "# Teams to filter on")
}

func TestMergeIntoExistingConfig_PreservesUnknownKeys(t *testing.T) {
	changes := ConfigChanges{TokenChanged: true}
	final := ResolvedValues{Token: "new-token", Teams: []string{"TEAM1"}}

	result, err := MergeIntoExistingConfig([]byte(existingFullConfig), final, changes, nil, nil)

	require.NoError(t, err)
	output := string(result)
	assert.Contains(t, output, "log_to_journal")
	assert.Contains(t, output, "toolbox_mode: auto")
}

func TestMergeIntoExistingConfig_BareYAMLDoc(t *testing.T) {
	changes := ConfigChanges{TokenChanged: true, TeamsChanged: true}
	final := ResolvedValues{Token: "new-token", Teams: []string{"TEAM1"}}
	teamNames := map[string]string{"TEAM1": "My Team"}

	result, err := MergeIntoExistingConfig([]byte("---\n"), final, changes, teamNames, nil)

	require.NoError(t, err)
	output := string(result)
	assert.Contains(t, output, "new-token")
	assert.Contains(t, output, "- TEAM1")
}

// --- WriteConfig tests ---

func TestWriteConfig_NewFile(t *testing.T) {
	m := &mockFS{}
	final := ResolvedValues{Token: "my-token", Teams: []string{"TEAM1"}}
	changes := ConfigChanges{TokenChanged: true, TeamsChanged: true}
	teamNames := map[string]string{"TEAM1": "My Team"}

	err := WriteConfig(m, "/fake/home", final, changes, teamNames, nil, true)

	require.NoError(t, err)
	output := string(m.writeData)
	assert.Contains(t, output, "token: my-token")
	assert.Contains(t, output, "- TEAM1 # My Team")
	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "terminal: gnome-terminal --")
	assert.Contains(t, output, "cluster_login_command:")
}

func TestWriteConfig_NewFileNoBackup(t *testing.T) {
	m := &mockFS{}
	final := ResolvedValues{Token: "my-token", Teams: []string{"TEAM1"}}
	changes := ConfigChanges{TokenChanged: true, TeamsChanged: true}

	err := WriteConfig(m, "/fake/home", final, changes, nil, nil, true)

	require.NoError(t, err)
	assert.Empty(t, m.backupData)
}

func TestWriteConfig_ExistingFileCreatesBackup(t *testing.T) {
	existingData := []byte("token: old-token\nteams:\n  - TEAM1\n")
	m := &mockFS{readData: existingData}
	final := ResolvedValues{Token: "new-token", Teams: []string{"TEAM1"}}
	changes := ConfigChanges{TokenChanged: true}

	err := WriteConfig(m, "/fake/home", final, changes, nil, nil, false)

	require.NoError(t, err)
	assert.Equal(t, string(existingData), string(m.backupData))
	assert.Contains(t, string(m.writeData), "new-token")
}

func TestWriteConfig_ExistingFilePreservesStructure(t *testing.T) {
	existingData := []byte(existingFullConfig)
	m := &mockFS{readData: existingData}
	final := ResolvedValues{Token: "new-token", Teams: []string{"TEAM1"}, SilentPolicy: "PCGXUDY"}
	changes := ConfigChanges{TokenChanged: true}

	err := WriteConfig(m, "/fake/home", final, changes, nil, nil, false)

	require.NoError(t, err)
	output := string(m.writeData)
	assert.Contains(t, output, "new-token")
	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "terminal: ptyxis")
	assert.Contains(t, output, "log_to_journal")
	assert.Contains(t, output, "# Main config")
}

func TestWriteConfig_ReadError(t *testing.T) {
	m := &mockFS{readErr: errors.New("disk error")}
	final := ResolvedValues{Token: "tok", Teams: []string{"T1"}}
	changes := ConfigChanges{TokenChanged: true}

	err := WriteConfig(m, "/fake/home", final, changes, nil, nil, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

func TestWriteConfig_WriteError(t *testing.T) {
	m := &mockFS{writeErr: errors.New("disk full")}
	final := ResolvedValues{Token: "tok", Teams: []string{"T1"}}
	changes := ConfigChanges{TokenChanged: true}

	err := WriteConfig(m, "/fake/home", final, changes, nil, nil, true)

	require.Error(t, err)
}

// --- end-to-end config flow tests ---

func TestEndToEnd_NewUserAllFields(t *testing.T) {
	m := &mockFS{}
	final := ResolvedValues{
		Token:               "brand-new-token",
		Teams:               []string{"PASPK4G"},
		SilentPolicy:        "PCGXUDY",
		CustomMappingsInput: "P5LAB5Y:PVBANNN",
	}
	changes := ConfigChanges{TokenChanged: true, TeamsChanged: true, SilentChanged: true, CustomChanged: true}
	teamNames := map[string]string{"PASPK4G": "Platform SRE"}
	customPolicies := map[string]string{"P5LAB5Y": "PVBANNN"}

	err := WriteConfig(m, "/fake/home", final, changes, teamNames, customPolicies, true)

	require.NoError(t, err)
	output := string(m.writeData)

	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(m.writeData, &parsed))
	assert.Equal(t, "brand-new-token", parsed["token"])
	teams, ok := parsed["teams"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, "PASPK4G", teams[0])
	assert.Equal(t, "PCGXUDY", parsed["default_silent_escalation_policy"])

	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "terminal: gnome-terminal --")
	assert.Contains(t, output, "cluster_login_command:")
	assert.Contains(t, output, "toolbox_mode: auto")
	assert.Contains(t, output, "chord_prefix: ctrl+x")
}

func TestEndToEnd_ExistingUserChangesToken(t *testing.T) {
	existingData := []byte(existingFullConfig)
	m := &mockFS{readData: existingData}
	final := ResolvedValues{Token: "changed-token", Teams: []string{"TEAM1"}, SilentPolicy: "PCGXUDY"}
	changes := ConfigChanges{TokenChanged: true}

	err := WriteConfig(m, "/fake/home", final, changes, nil, nil, false)

	require.NoError(t, err)
	output := string(m.writeData)
	assert.Contains(t, output, "changed-token")
	assert.Contains(t, output, "- TEAM1")
	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "terminal: ptyxis")
}

func TestEndToEnd_ExistingUserChangesTeams(t *testing.T) {
	existingData := []byte(existingFullConfig)
	m := &mockFS{readData: existingData}
	final := ResolvedValues{Token: "existing-token", Teams: []string{"TEAM2", "TEAM3"}, SilentPolicy: "PCGXUDY"}
	changes := ConfigChanges{TeamsChanged: true}
	teamNames := map[string]string{"TEAM2": "Beta", "TEAM3": "Gamma"}

	err := WriteConfig(m, "/fake/home", final, changes, teamNames, nil, false)

	require.NoError(t, err)
	output := string(m.writeData)
	assert.Contains(t, output, "- TEAM2 # Beta")
	assert.Contains(t, output, "- TEAM3 # Gamma")
	assert.NotContains(t, output, "TEAM1")
	assert.Contains(t, output, "existing-token")
}

func TestEndToEnd_EnvVarUserFirstSave(t *testing.T) {
	m := &mockFS{}
	final := ResolvedValues{
		Token:        "env-token",
		Teams:        []string{"ENV_TEAM"},
		SilentPolicy: "ENV_POL",
	}
	changes := ConfigChanges{TokenChanged: true, TeamsChanged: true, SilentChanged: true}
	teamNames := map[string]string{"ENV_TEAM": "Env Team"}

	err := WriteConfig(m, "/fake/home", final, changes, teamNames, nil, true)

	require.NoError(t, err)
	output := string(m.writeData)
	assert.Contains(t, output, "token: env-token")
	assert.Contains(t, output, "- ENV_TEAM # Env Team")
	assert.Contains(t, output, "default_silent_escalation_policy: ENV_POL")
	assert.Contains(t, output, "editor: vim")
}

// --- Issue: new config must have real values for optional keys ---

func TestBuildFullConfig_OptionalKeysAreRealValues(t *testing.T) {
	final := ResolvedValues{Token: "tok", Teams: []string{"T1"}}
	result := BuildFullConfig(final, nil, "", nil)

	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(result, &parsed))

	assert.Equal(t, "vim", parsed["editor"])
	assert.Equal(t, "gnome-terminal --", parsed["terminal"])
	assert.Equal(t, "ocm backplane login %%CLUSTER_ID%%", parsed["cluster_login_command"])
	assert.Equal(t, "auto", parsed["toolbox_mode"])
	assert.Equal(t, "ctrl+x", parsed["chord_prefix"])
	assert.Equal(t, "rosa-boundary start-task --cluster-id %%CLUSTER_ID%% --connect", parsed["rosa_boundary_command"])
}

func TestBuildFullConfig_OptionalKeysHaveDescriptionComments(t *testing.T) {
	final := ResolvedValues{Token: "tok", Teams: []string{"T1"}}
	result := BuildFullConfig(final, nil, "", nil)
	output := string(result)

	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "terminal: gnome-terminal --")
	assert.Contains(t, output, "cluster_login_command: ocm backplane login %%CLUSTER_ID%%")
}

// --- Issue: no config file means everything is "changed" ---

func TestDetectChanges_NewFileAlwaysChanged(t *testing.T) {
	existing := ExistingConfig{
		Token:          "from-env",
		Teams:          []string{"TEAM1"},
		SilentPolicy:   "POL1",
		CustomPolicies: map[string]string{"SVC1": "POL2"},
	}
	final := ResolvedValues{
		Token:               "from-env",
		Teams:               []string{"TEAM1"},
		SilentPolicy:        "POL1",
		CustomMappingsInput: "SVC1:POL2",
	}

	changes := DetectChanges(existing, final, "")
	assert.False(t, changes.AnyChanged(), "without isNewFile, env-var-only should show no changes")

	changes = DetectChangesForNewFile(final)
	assert.True(t, changes.AnyChanged(), "new file must always report changes")
	assert.True(t, changes.TokenChanged)
	assert.True(t, changes.TeamsChanged)
}

func TestEndToEnd_NewFileWritesRealDefaults(t *testing.T) {
	m := &mockFS{}
	final := ResolvedValues{
		Token: "new-user-token",
		Teams: []string{"TEAM1"},
	}
	changes := DetectChangesForNewFile(final)
	teamNames := map[string]string{"TEAM1": "My Team"}

	err := WriteConfig(m, "/fake/home", final, changes, teamNames, nil, true)

	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal(m.writeData, &parsed))
	assert.Equal(t, "new-user-token", parsed["token"])
	assert.Equal(t, "vim", parsed["editor"])
	assert.Equal(t, "gnome-terminal --", parsed["terminal"])
	assert.Contains(t, parsed["cluster_login_command"], "%%CLUSTER_ID%%")
	assert.Equal(t, "auto", parsed["toolbox_mode"])
	assert.Equal(t, "ctrl+x", parsed["chord_prefix"])
	assert.Contains(t, parsed["rosa_boundary_command"], "%%CLUSTER_ID%%")
}

// --- CommentOutOldPolicies tests ---

func TestCommentOutOldPolicies_CommentsBlock(t *testing.T) {
	input := "token: my-token\nservice_escalation_policies:\n  SILENT_DEFAULT: PCGXUDY\n  DEFAULT: PA4586M\n  P5LAB5Y: PVBANNN\neditor: vim\n"

	result := CommentOutOldPolicies([]byte(input))
	output := string(result)

	assert.NotContains(t, output, "\nservice_escalation_policies:")
	assert.Contains(t, output, "# service_escalation_policies:")
	assert.Contains(t, output, "#   SILENT_DEFAULT: PCGXUDY")
	assert.Contains(t, output, "#   P5LAB5Y: PVBANNN")
	assert.Contains(t, output, "editor: vim")
	assert.Contains(t, output, "token: my-token")
}

func TestCommentOutOldPolicies_NoOldBlock(t *testing.T) {
	input := "token: my-token\neditor: vim\n"

	result := CommentOutOldPolicies([]byte(input))

	assert.Equal(t, input, string(result))
}

func TestCommentOutOldPolicies_AddsDeprecatedNote(t *testing.T) {
	input := "service_escalation_policies:\n  SILENT_DEFAULT: PCGXUDY\n"

	result := CommentOutOldPolicies([]byte(input))
	output := string(result)

	assert.Contains(t, output, "# Deprecated")
}
