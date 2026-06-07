/*
Copyright 2025 NAME HERE <EMAIL ADDRESS>
*/
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	CfgFileName = "srepd.yaml"
	CfgFileDir  = ".config/srepd"
)

var (
	RequiredKeys = map[string]string{
		"token": "PagerDuty API token",
		"teams": "PagerDuty team IDs to filter on",
	}
	DefaultOptionalKeys = map[string]string{
		"editor":                "vim",
		"terminal":              "gnome-terminal --",
		"cluster_login_command": "ocm backplane login %%CLUSTER_ID%%",
		"toolbox_mode":          "auto",
		"chord_prefix":          "ctrl+x",
	}
	OptionalKeys = map[string]string{
		"editor":                             fmt.Sprintf("Editor to use for notes (default: %v)", DefaultOptionalKeys["editor"]),
		"terminal":                           fmt.Sprintf("Terminal to use for exec commands (default: %v)", DefaultOptionalKeys["terminal"]),
		"cluster_login_command":              fmt.Sprintf("Cluster login command (default: %v)", DefaultOptionalKeys["cluster_login_command"]),
		"toolbox_mode":                       fmt.Sprintf("Toolbox detection mode: auto, true, false (default: %v)", DefaultOptionalKeys["toolbox_mode"]),
		"chord_prefix":                       fmt.Sprintf("Chord prefix key for multi-key commands (default: %v)", DefaultOptionalKeys["chord_prefix"]),
		"colors":                             "Custom color scheme (map of color name to hex value)",
		"default_silent_escalation_policy":   "Default silent escalation policy ID (auto-discovered via srepd config)",
		"custom_service_escalation_policies": "Per-service silent policy overrides (service ID → policy ID)",
	}
)

type ExistingConfig struct {
	Token             string
	Teams             []string
	SilentPolicy      string
	CustomPolicies    map[string]string
	OldFormatDetected bool
}

func ResolveExistingConfig(
	token string,
	teams []string,
	silentPolicy string,
	customPolicies map[string]string,
	customPoliciesRaw string,
	oldPolicies map[string]string,
) ExistingConfig {
	uppercased := make(map[string]string, len(customPolicies))
	for k, v := range customPolicies {
		uppercased[strings.ToUpper(k)] = v
	}
	cfg := ExistingConfig{
		Token:          token,
		Teams:          teams,
		SilentPolicy:   silentPolicy,
		CustomPolicies: uppercased,
	}

	if len(cfg.CustomPolicies) == 0 && customPoliciesRaw != "" {
		cfg.CustomPolicies = ParseCustomMappings(customPoliciesRaw)
	}

	if cfg.SilentPolicy == "" || len(cfg.CustomPolicies) == 0 {
		if cfg.SilentPolicy == "" {
			if sd, ok := oldPolicies["silent_default"]; ok {
				cfg.SilentPolicy = sd
				cfg.OldFormatDetected = true
			}
		}
		if len(cfg.CustomPolicies) == 0 {
			migrated := make(map[string]string)
			for k, v := range oldPolicies {
				if k != "default" && k != "silent_default" {
					migrated[strings.ToUpper(k)] = v
				}
			}
			if len(migrated) > 0 {
				cfg.CustomPolicies = migrated
				cfg.OldFormatDetected = true
			}
		}
	}

	return cfg
}

type KeepDefaults struct {
	HasValidTeams bool
	KeepTeams     bool
	HasSilent     bool
	KeepSilent    bool
	HasCustom     bool
	KeepCustom    bool
}

func ResolveKeepDefaults(teams []string, silentPolicy string, customPolicies map[string]string) KeepDefaults {
	kd := KeepDefaults{}
	kd.HasValidTeams = !HasPlaceholderTeams(teams)
	kd.KeepTeams = kd.HasValidTeams
	kd.HasSilent = silentPolicy != ""
	kd.KeepSilent = kd.HasSilent
	kd.HasCustom = len(customPolicies) > 0
	kd.KeepCustom = kd.HasCustom
	return kd
}

type WizardInputs struct {
	TokenInput          string
	SelectedTeams       []string
	SilentPolicyID      string
	CustomMappingsInput string
	KeepTeams           bool
	KeepSilent          bool
	KeepCustom          bool
}

type ResolvedValues struct {
	Token               string
	Teams               []string
	SilentPolicy        string
	CustomMappingsInput string
}

func ResolveFinalValues(existing ExistingConfig, inputs WizardInputs) (ResolvedValues, error) {
	tokenInput := strings.TrimSpace(inputs.TokenInput)
	finalToken := existing.Token
	if tokenInput != "" {
		finalToken = tokenInput
	}
	if finalToken == "" {
		return ResolvedValues{}, fmt.Errorf("a PagerDuty API token is required")
	}

	rv := ResolvedValues{Token: finalToken}

	if inputs.KeepTeams {
		rv.Teams = existing.Teams
	} else {
		rv.Teams = inputs.SelectedTeams
	}

	if inputs.KeepSilent {
		rv.SilentPolicy = existing.SilentPolicy
	} else {
		rv.SilentPolicy = strings.TrimSpace(inputs.SilentPolicyID)
	}

	if inputs.KeepCustom {
		rv.CustomMappingsInput = FormatCustomMappings(existing.CustomPolicies)
	} else {
		rv.CustomMappingsInput = strings.TrimSpace(inputs.CustomMappingsInput)
	}

	return rv, nil
}

type ConfigChanges struct {
	TokenChanged  bool
	TeamsChanged  bool
	SilentChanged bool
	CustomChanged bool
}

func (c ConfigChanges) AnyChanged() bool {
	return c.TokenChanged || c.TeamsChanged || c.SilentChanged || c.CustomChanged
}

func DetectChangesForNewFile(final ResolvedValues) ConfigChanges {
	return ConfigChanges{
		TokenChanged:  true,
		TeamsChanged:  true,
		SilentChanged: final.SilentPolicy != "",
		CustomChanged: final.CustomMappingsInput != "",
	}
}

func DetectChanges(existing ExistingConfig, final ResolvedValues, tokenInput string) ConfigChanges {
	return ConfigChanges{
		TokenChanged:  tokenInput != "" && tokenInput != existing.Token,
		TeamsChanged:  !StringSlicesEqual(final.Teams, existing.Teams),
		SilentChanged: final.SilentPolicy != existing.SilentPolicy,
		CustomChanged: final.CustomMappingsInput != FormatCustomMappings(existing.CustomPolicies),
	}
}

type ConfigFS interface {
	MkdirAll(path string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (io.WriteCloser, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
}

func MaskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:4] + strings.Repeat("*", len(token)-4)
}

func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool)
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if !set[v] {
			return false
		}
	}
	return true
}

func ParseCustomMappings(raw string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			svcID := strings.TrimSpace(parts[0])
			polID := strings.TrimSpace(parts[1])
			if svcID != "" && polID != "" {
				result[svcID] = polID
			}
		}
	}
	return result
}

func ParseCustomMappingsStrict(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	result := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping %q — expected SERVICE_ID:POLICY_ID", pair)
		}
		svcID := strings.TrimSpace(parts[0])
		polID := strings.TrimSpace(parts[1])
		if svcID == "" {
			return nil, fmt.Errorf("empty service ID in mapping %q", pair)
		}
		if polID == "" {
			return nil, fmt.Errorf("empty policy ID in mapping %q", pair)
		}
		result[svcID] = polID
	}
	return result, nil
}

func WriteConfig(fs ConfigFS, baseDir string, final ResolvedValues, changes ConfigChanges, teamNames map[string]string, customPolicies map[string]string, isNewFile bool) error {
	configFile := filepath.Join(baseDir, CfgFileDir, CfgFileName)

	if isNewFile {
		data := BuildFullConfig(final, teamNames, final.SilentPolicy, customPolicies)
		if err := fs.WriteFile(configFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
		return nil
	}

	existingData, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := MergeIntoExistingConfig(existingData, final, changes, teamNames, customPolicies)
	if err != nil {
		return err
	}

	backupFile := configFile + "~"
	if err := fs.WriteFile(backupFile, existingData, 0644); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	if err := fs.WriteFile(configFile, updated, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func MergeIntoExistingConfig(existingData []byte, final ResolvedValues, changes ConfigChanges, teamNames map[string]string, customPolicies map[string]string) ([]byte, error) {
	data := make([]byte, len(existingData))
	copy(data, existingData)

	if !changes.AnyChanged() {
		return existingData, nil
	}

	var err error
	if changes.TokenChanged {
		data, err = UpsertScalarInConfig(data, "token", final.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to update token: %w", err)
		}
	}

	if changes.TeamsChanged {
		data, err = UpdateTeamsInConfig(data, final.Teams, teamNames)
		if err != nil {
			return nil, fmt.Errorf("failed to update teams: %w", err)
		}
	}

	if changes.SilentChanged {
		data, err = UpsertScalarInConfig(data, "default_silent_escalation_policy", final.SilentPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to update silent policy: %w", err)
		}
	}

	if changes.CustomChanged && customPolicies != nil {
		data, err = UpsertMapInConfig(data, "custom_service_escalation_policies", customPolicies)
		if err != nil {
			return nil, fmt.Errorf("failed to update custom policies: %w", err)
		}
	}

	if changes.SilentChanged || changes.CustomChanged {
		data = CommentOutOldPolicies(data)
	}

	return data, nil
}

func BuildFullConfig(final ResolvedValues, teamNames map[string]string, silentPolicy string, customPolicies map[string]string) []byte {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString("# PagerDuty API token\n")
	fmt.Fprintf(&sb, "token: %s\n", final.Token)

	sb.WriteString("\n# PagerDuty team IDs\n")
	sb.WriteString("teams:\n")
	for _, id := range final.Teams {
		if name, ok := teamNames[id]; ok {
			fmt.Fprintf(&sb, "  - %s # %s\n", id, name)
		} else {
			fmt.Fprintf(&sb, "  - %s\n", id)
		}
	}

	sb.WriteString("\n# Optional settings\n")
	for _, entry := range []struct{ key, val string }{
		{"editor", "vim"},
		{"terminal", "gnome-terminal --"},
		{"cluster_login_command", "ocm backplane login %%CLUSTER_ID%%"},
		{"toolbox_mode", "auto"},
		{"chord_prefix", "ctrl+x"},
	} {
		fmt.Fprintf(&sb, "%s: %s\n", entry.key, entry.val)
	}

	if silentPolicy != "" {
		sb.WriteString("\n# Silent escalation policy\n")
		fmt.Fprintf(&sb, "default_silent_escalation_policy: %s\n", silentPolicy)
	}

	if len(customPolicies) > 0 {
		sb.WriteString("\n# Custom service-to-policy overrides\n")
		sb.WriteString("custom_service_escalation_policies:\n")
		for svcID, polID := range customPolicies {
			fmt.Fprintf(&sb, "  %s: %s\n", svcID, polID)
		}
	}

	return []byte(sb.String())
}

func BuildSummary(existing ExistingConfig, final ResolvedValues, changes ConfigChanges, teamNames map[string]string, policyNames map[string]string) string {
	var sb strings.Builder

	if changes.TokenChanged {
		fmt.Fprintf(&sb, "  Token:          %s (changed)\n", MaskToken(final.Token))
	} else {
		fmt.Fprintf(&sb, "  Token:          %s (unchanged)\n", MaskToken(final.Token))
	}

	var teamDisplay []string
	for _, id := range final.Teams {
		if name, ok := teamNames[id]; ok {
			teamDisplay = append(teamDisplay, fmt.Sprintf("%s (%s)", name, id))
		} else {
			teamDisplay = append(teamDisplay, id)
		}
	}
	changeLabel := " (unchanged)"
	if changes.TeamsChanged {
		changeLabel = " (changed)"
	}
	fmt.Fprintf(&sb, "  Teams:          %s%s\n", strings.Join(teamDisplay, ", "), changeLabel)

	if final.SilentPolicy != "" {
		changeLabel = " (unchanged)"
		if changes.SilentChanged {
			changeLabel = " (changed)"
		}
		silentDisplay := final.SilentPolicy
		if name, ok := policyNames[final.SilentPolicy]; ok {
			silentDisplay = fmt.Sprintf("%s (%s)", name, final.SilentPolicy)
		}
		fmt.Fprintf(&sb, "  Silent policy:  %s%s\n", silentDisplay, changeLabel)
	}

	if final.CustomMappingsInput != "" {
		changeLabel = " (unchanged)"
		if changes.CustomChanged {
			changeLabel = " (changed)"
		}
		customDisplay := final.CustomMappingsInput
		parsed := ParseCustomMappings(final.CustomMappingsInput)
		if len(parsed) > 0 && len(policyNames) > 0 {
			var parts []string
			for svcID, polID := range parsed {
				polDisplay := polID
				if name, ok := policyNames[polID]; ok {
					polDisplay = fmt.Sprintf("%s (%s)", name, polID)
				}
				parts = append(parts, fmt.Sprintf("%s → %s", svcID, polDisplay))
			}
			customDisplay = strings.Join(parts, ", ")
		}
		fmt.Fprintf(&sb, "  Custom:         %s%s\n", customDisplay, changeLabel)
	}

	return sb.String()
}

func FormatCustomMappings(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	var pairs []string
	for svcID, polID := range m {
		pairs = append(pairs, svcID+":"+polID)
	}
	return strings.Join(pairs, ", ")
}

func ensureYAMLMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind != yaml.DocumentNode {
		return nil
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Content = []*yaml.Node{root}
	}
	return doc.Content[0]
}

func UpdateTeamsInConfig(configData []byte, teamIDs []string, teamNames map[string]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	root := ensureYAMLMapping(&doc)
	if root == nil {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "teams" {
			teamsValue := root.Content[i+1]
			teamsValue.Content = nil
			if len(teamIDs) == 0 {
				teamsValue.Kind = yaml.SequenceNode
				teamsValue.Tag = "!!seq"
				teamsValue.Style = yaml.FlowStyle
			} else {
				teamsValue.Kind = yaml.SequenceNode
				teamsValue.Tag = "!!seq"
				teamsValue.Style = 0
				for _, id := range teamIDs {
					node := &yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: id,
					}
					if name, ok := teamNames[id]; ok {
						node.LineComment = name
					}
					teamsValue.Content = append(teamsValue.Content, node)
				}
			}

			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(&doc); err != nil {
				return nil, fmt.Errorf("failed to encode config YAML: %w", err)
			}
			if err := enc.Close(); err != nil {
				return nil, fmt.Errorf("failed to close YAML encoder: %w", err)
			}
			return buf.Bytes(), nil
		}
	}

	// teams key not found — append it
	teamsKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "teams"}
	teamsValue := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, id := range teamIDs {
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id}
		if name, ok := teamNames[id]; ok {
			node.LineComment = name
		}
		teamsValue.Content = append(teamsValue.Content, node)
	}
	root.Content = append(root.Content, teamsKey, teamsValue)
	return encodeYAMLDoc(&doc)
}

func WriteConfigTeams(fs ConfigFS, baseDir string, teamIDs []string, teamNames map[string]string) error {
	configFile := filepath.Join(baseDir, CfgFileDir, CfgFileName)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := UpdateTeamsInConfig(data, teamIDs, teamNames)
	if err != nil {
		return err
	}

	backupFile := configFile + "~"
	if err := fs.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	if err := fs.WriteFile(configFile, updated, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func WriteConfigKey(fs ConfigFS, baseDir string, key string, value string) error {
	configFile := filepath.Join(baseDir, CfgFileDir, CfgFileName)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := UpsertScalarInConfig(data, key, value)
	if err != nil {
		return err
	}

	backupFile := configFile + "~"
	if err := fs.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	if err := fs.WriteFile(configFile, updated, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func WriteConfigMap(fs ConfigFS, baseDir string, key string, values map[string]string) error {
	configFile := filepath.Join(baseDir, CfgFileDir, CfgFileName)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := UpsertMapInConfig(data, key, values)
	if err != nil {
		return err
	}

	backupFile := configFile + "~"
	if err := fs.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	if err := fs.WriteFile(configFile, updated, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func UpsertMapInConfig(configData []byte, key string, values map[string]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	root := ensureYAMLMapping(&doc)
	if root == nil {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	mapNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for k, v := range values {
		mapNode.Content = append(mapNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
		)
	}

	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1] = mapNode
			return encodeYAMLDoc(&doc)
		}
	}

	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		mapNode,
	)
	return encodeYAMLDoc(&doc)
}

func encodeYAMLDoc(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("failed to encode config YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("failed to close YAML encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func UpsertScalarInConfig(configData []byte, key string, value string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	root := ensureYAMLMapping(&doc)
	if root == nil {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	// Update existing key or append new one
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1].Value = value
			root.Content[i+1].Tag = "!!str"
			root.Content[i+1].Kind = yaml.ScalarNode

			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(&doc); err != nil {
				return nil, fmt.Errorf("failed to encode config YAML: %w", err)
			}
			if err := enc.Close(); err != nil {
				return nil, fmt.Errorf("failed to close YAML encoder: %w", err)
			}
			return buf.Bytes(), nil
		}
	}

	// Key not found — append it
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("failed to encode config YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("failed to close YAML encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func CommentOutOldPolicies(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	var result []string
	inBlock := false
	found := false

	for _, line := range lines {
		if strings.HasPrefix(line, "service_escalation_policies:") {
			inBlock = true
			found = true
			result = append(result, "# Deprecated: migrated to default_silent_escalation_policy + custom_service_escalation_policies")
			result = append(result, "# "+line)
			continue
		}
		if inBlock {
			if strings.HasPrefix(line, "  ") || line == "" {
				result = append(result, "# "+line)
				continue
			}
			inBlock = false
		}
		result = append(result, line)
	}

	if !found {
		return data
	}
	return []byte(strings.Join(result, "\n"))
}

func HasPlaceholderTeams(teams []string) bool {
	if len(teams) == 0 {
		return true
	}
	for _, team := range teams {
		trimmed := strings.TrimSpace(team)
		if trimmed != "" && !strings.HasPrefix(trimmed, "<PagerDuty Team ID") {
			return false
		}
	}
	return true
}
