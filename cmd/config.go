/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/deprecation"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/clcollins/srepd/pkg/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	cfgFileName = "srepd.yaml"
	cfgFileDir  = ".config/srepd"
)

const description = `The config command is used to create or validate the SREPD config file.
The config file is located at ~/.config/srepd/srepd.yaml and is used to store
the configuration options for the SREPD application.`

var (
	requiredKeys = map[string]string{
		"token": "PagerDuty API token",
		"teams": "PagerDuty team IDs to filter on",
	}
	defaultOptionalKeys = map[string]string{
		"editor":                "vim",
		"terminal":              "gnome-terminal --",
		"cluster_login_command": "ocm backplane login %%CLUSTER_ID%%",
		"toolbox_mode":          "auto",
		"chord_prefix":          "ctrl+x",
	}
	optionalKeys = map[string]string{
		"editor":                             fmt.Sprintf("Editor to use for notes (default: %v)", defaultOptionalKeys["editor"]),
		"terminal":                           fmt.Sprintf("Terminal to use for exec commands (default: %v)", defaultOptionalKeys["terminal"]),
		"cluster_login_command":              fmt.Sprintf("Cluster login command (default: %v)", defaultOptionalKeys["cluster_login_command"]),
		"toolbox_mode":                       fmt.Sprintf("Toolbox detection mode: auto, true, false (default: %v)", defaultOptionalKeys["toolbox_mode"]),
		"chord_prefix":                       fmt.Sprintf("Chord prefix key for multi-key commands (default: %v)", defaultOptionalKeys["chord_prefix"]),
		"colors":                             "Custom color scheme (map of color name to hex value)",
		"default_silent_escalation_policy":   "Default silent escalation policy ID (auto-discovered via srepd config)",
		"custom_service_escalation_policies": "Per-service silent policy overrides (service ID → policy ID)",
	}
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:          "config",
	Short:        "Configure SREPD interactively",
	Long:         description,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigWizard()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

type configFS interface {
	MkdirAll(path string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (io.WriteCloser, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
}

type osFS struct{} // codecov:ignore

func (osFS) MkdirAll(path string, perm os.FileMode) error { // codecov:ignore
	return os.MkdirAll(path, perm)
}

func (osFS) OpenFile(name string, flag int, perm os.FileMode) (io.WriteCloser, error) { // codecov:ignore
	return os.OpenFile(name, flag, perm)
}

func (osFS) ReadFile(name string) ([]byte, error) { // codecov:ignore
	return os.ReadFile(name)
}

func (osFS) WriteFile(name string, data []byte, perm os.FileMode) error { // codecov:ignore
	return os.WriteFile(name, data, perm)
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:4] + strings.Repeat("*", len(token)-4)
}

func runConfigWizard() error {
	if viper.GetBool("dev") {
		runDevMode()
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(home, cfgFileDir)
	configFile := filepath.Join(configDir, cfgFileName)

	existingToken := viper.GetString("token")
	existingTeams := viper.GetStringSlice("teams")
	existingSilent := viper.GetString("default_silent_escalation_policy")
	existingCustom := viper.GetStringMapString("custom_service_escalation_policies")

	var tokenInput string
	tokenDesc := "Your PagerDuty API OAuth token."
	if existingToken != "" {
		tokenDesc = fmt.Sprintf("Current: %s — leave blank to keep.", maskToken(existingToken))
	}

	var selectedTeams []string
	silentPolicyID := existingSilent
	var customMappingsInput string
	if len(existingCustom) > 0 {
		var pairs []string
		for svcID, polID := range existingCustom {
			pairs = append(pairs, svcID+":"+polID)
		}
		customMappingsInput = strings.Join(pairs, ", ")
	}

	colors := viper.GetStringMapString("colors")
	theme := tui.ThemeFromConfig(colors)
	huhTheme := buildHuhTheme(theme)

	// Phase 1: Get token
	tokenForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("PagerDuty API token").
				Description(tokenDesc).
				EchoMode(huh.EchoModePassword).
				Value(&tokenInput),
		),
	).WithTheme(huhTheme).WithShowHelp(false).WithProgramOptions(tea.WithAltScreen())

	if err := tokenForm.Run(); err != nil {
		return fmt.Errorf("config wizard failed: %w", err)
	}
	if tokenForm.State == huh.StateAborted {
		return nil
	}

	tokenInput = strings.TrimSpace(tokenInput)
	finalToken := existingToken
	if tokenInput != "" {
		finalToken = tokenInput
	}
	if finalToken == "" {
		return fmt.Errorf("a PagerDuty API token is required")
	}

	// Phase 2: Fetch teams and build the rest of the form
	client := pd.NewClient(finalToken)
	teams, err := pd.GetCurrentUserTeams(client)
	if err != nil {
		return fmt.Errorf("failed to fetch teams (is your token valid?): %w", err)
	}

	if len(teams) == 0 {
		fmt.Println("No teams found in your PagerDuty account.")
		return nil
	}

	existingTeamSet := make(map[string]bool)
	for _, id := range existingTeams {
		existingTeamSet[id] = true
	}

	var teamOptions []huh.Option[string]
	for _, team := range teams {
		opt := huh.NewOption(fmt.Sprintf("%s — %s", team.Name, team.ID), team.ID)
		if existingTeamSet[team.ID] {
			opt = opt.Selected(true)
		}
		teamOptions = append(teamOptions, opt)
	}

	submitted := false
	mainForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select your PagerDuty teams").
				Description("<space> toggle • ↑ up • ↓ down • / filter • enter submit • ctrl+a select all • ctrl+c quit").
				Options(teamOptions...).
				Value(&selectedTeams).
				Validate(func(s []string) error {
					if !submitted {
						submitted = true
						return nil
					}
					if len(s) == 0 {
						return fmt.Errorf("at least one team is required")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Default silent escalation policy").
				Description(
					"When you silence an incident, it gets reassigned to a silent\n"+
						"escalation policy — one that routes only to bot users, not\n"+
						"on-call humans. Find this ID in PagerDuty → Escalation Policies\n"+
						"(e.g., \"Silent Test\"). Leave blank to configure later.",
				).
				Value(&silentPolicyID),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Custom service-to-policy mappings").
				Description(
					"Some services use a different silent policy than the default.\n"+
						"For example, Deadmanssnitch alerts might route to a separate\n"+
						"silent policy instead of the general one.\n"+
						"Enter as SERVICE_ID:POLICY_ID separated by commas.\n"+
						"Leave blank to skip.",
				).
				Value(&customMappingsInput),
		),
	).WithTheme(huhTheme).WithShowHelp(false).WithProgramOptions(tea.WithAltScreen())

	if err := mainForm.Run(); err != nil {
		return fmt.Errorf("config wizard failed: %w", err)
	}
	if mainForm.State == huh.StateAborted {
		return nil
	}

	// Build summary
	silentPolicyID = strings.TrimSpace(silentPolicyID)
	customMappingsInput = strings.TrimSpace(customMappingsInput)

	tokenChanged := tokenInput != "" && tokenInput != existingToken
	teamsChanged := !stringSlicesEqual(selectedTeams, existingTeams)
	silentChanged := silentPolicyID != existingSilent
	customChanged := customMappingsInput != formatCustomMappings(existingCustom)

	teamNames := make(map[string]string)
	for _, team := range teams {
		teamNames[team.ID] = team.Name
	}

	fmt.Println("\nConfiguration summary:")
	if tokenChanged {
		fmt.Printf("  Token:          %s (changed)\n", maskToken(finalToken))
	} else {
		fmt.Printf("  Token:          %s (unchanged)\n", maskToken(finalToken))
	}

	teamDisplay := []string{}
	for _, id := range selectedTeams {
		if name, ok := teamNames[id]; ok {
			teamDisplay = append(teamDisplay, fmt.Sprintf("%s (%s)", name, id))
		} else {
			teamDisplay = append(teamDisplay, id)
		}
	}
	changeLabel := ""
	if teamsChanged {
		changeLabel = " (changed)"
	}
	fmt.Printf("  Teams:          %s%s\n", strings.Join(teamDisplay, ", "), changeLabel)

	if silentPolicyID != "" {
		changeLabel = ""
		if silentChanged {
			changeLabel = " (changed)"
		}
		fmt.Printf("  Silent policy:  %s%s\n", silentPolicyID, changeLabel)
	}

	if customMappingsInput != "" {
		changeLabel = ""
		if customChanged {
			changeLabel = " (changed)"
		}
		fmt.Printf("  Custom:         %s%s\n", customMappingsInput, changeLabel)
	}

	if !tokenChanged && !teamsChanged && !silentChanged && !customChanged {
		fmt.Println("\nConfig is valid. No changes needed.")
		return nil
	}

	// Confirm save
	var confirm bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save changes?").
				Value(&confirm),
		),
	).WithTheme(huhTheme).WithShowHelp(false)

	if err := confirmForm.Run(); err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}
	if !confirm || confirmForm.State == huh.StateAborted {
		fmt.Println("Changes discarded.")
		return nil
	}

	// Ensure config dir exists
	if err := (osFS{}).MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// If no config file, create one with defaults first
	if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
		if writeErr := (osFS{}).WriteFile(configFile, []byte("---\n"), 0644); writeErr != nil {
			return fmt.Errorf("failed to create config file: %w", writeErr)
		}
	}

	// Write token
	if tokenChanged {
		if err := writeConfigKey(osFS{}, home, "token", finalToken); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}
	}

	// Write teams
	if teamsChanged {
		if err := writeConfigTeams(osFS{}, home, selectedTeams, teamNames); err != nil {
			return fmt.Errorf("failed to save teams: %w", err)
		}
	}

	// Validate and write silent policy
	if silentChanged && silentPolicyID != "" {
		if _, err := pd.GetEscalationPolicy(client, silentPolicyID, pagerduty.GetEscalationPolicyOptions{}); err != nil {
			return fmt.Errorf("invalid silent policy ID %q: %w", silentPolicyID, err)
		}
		if err := writeConfigKey(osFS{}, home, "default_silent_escalation_policy", silentPolicyID); err != nil {
			return fmt.Errorf("failed to save silent policy: %w", err)
		}
	}

	// Validate and write custom mappings
	if customChanged && customMappingsInput != "" {
		customPolicies := make(map[string]string)
		for _, pair := range strings.Split(customMappingsInput, ",") {
			pair = strings.TrimSpace(pair)
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid mapping %q — expected SERVICE_ID:POLICY_ID", pair)
			}
			svcID := strings.TrimSpace(parts[0])
			polID := strings.TrimSpace(parts[1])
			if _, err := pd.GetEscalationPolicy(client, polID, pagerduty.GetEscalationPolicyOptions{}); err != nil {
				return fmt.Errorf("invalid policy %q for service %q: %w", polID, svcID, err)
			}
			customPolicies[svcID] = polID
		}
		if err := writeConfigMap(osFS{}, home, "custom_service_escalation_policies", customPolicies); err != nil {
			return fmt.Errorf("failed to save custom policies: %w", err)
		}
	}

	fmt.Println("\nConfig saved. Launching srepd...")
	launchTUI()
	return nil
}

func stringSlicesEqual(a, b []string) bool {
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

func formatCustomMappings(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	var pairs []string
	for svcID, polID := range m {
		pairs = append(pairs, svcID+":"+polID)
	}
	return strings.Join(pairs, ", ")
}

func updateTeamsInConfig(configData []byte, teamIDs []string, teamNames map[string]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected YAML mapping at root")
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

	return nil, fmt.Errorf("'teams' key not found in config")
}

func writeConfigTeams(fs configFS, baseDir string, teamIDs []string, teamNames map[string]string) error {
	configFile := filepath.Join(baseDir, cfgFileDir, cfgFileName)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := updateTeamsInConfig(data, teamIDs, teamNames)
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

func buildHuhTheme(theme tui.Theme) *huh.Theme {
	t := huh.ThemeCharm()
	t.Focused.Title = t.Focused.Title.Foreground(theme.Highlight)
	t.Focused.Description = t.Focused.Description.Foreground(theme.Muted)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(theme.Highlight)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(theme.Text)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(theme.Text)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(theme.Highlight)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(theme.Muted)
	t.Focused.Base = t.Focused.Base.BorderForeground(theme.Border)
	return t
}

func hasPlaceholderTeams(teams []string) bool {
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

// validateConfig prints the viper info passed into the program
func validateConfig() error {
	errs := []error{}
	settings := viper.GetViper().AllSettings()
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if deprecation.Deprecated(k) {
			log.Info("Found deprecated key; you may remove this from your config", "key_name", k)
			continue
		}

		var v string

		v = fmt.Sprintf("%v", settings[k])
		if strings.Contains(k, "token") {
			v = "*****"
		}

		log.Debug("Found key", k, v)

	}

	for k, v := range requiredKeys {
		if _, ok := settings[k]; !ok {
			errs = append(errs, fmt.Errorf("missing required key: %s ", k))
			log.Error("Missing required key", "key_name", k, "key_description", v)
		}
	}

	if _, ok := settings["service_escalation_policies"]; ok {

		requiredEscalationPolicyKeys := map[string]string{
			"DEFAULT":        "The default PagerDuty escalation policy to re-escalate alerts",
			"SILENT_DEFAULT": "A PagerDuty escalation policy to re-escalate alerts without notifications (silence)",
		}

		serviceEscalationPolicies, ok := settings["service_escalation_policies"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("'service_escalation_policies' is not a valid map")
		}

		for k, v := range requiredEscalationPolicyKeys {
			if _, ok := serviceEscalationPolicies[strings.ToLower(k)]; !ok {
				errs = append(errs, fmt.Errorf("'service_escalation_policies' missing required key: %s", k))
				log.Error("Missing required key", "key_name", k, "key_description", v)
			}
		}
	}

	for k := range optionalKeys {
		_, ok := settings[k]
		if !ok {
			log.Debug("cmd.validateConfig()", "msg", "missing optional key", "key", k, "default_value", defaultOptionalKeys[k])
			viper.Set(k, defaultOptionalKeys[k])
		}
	}

	return errors.Join(errs...)
}

func writeConfigKey(fs configFS, baseDir string, key string, value string) error {
	configFile := filepath.Join(baseDir, cfgFileDir, cfgFileName)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := upsertScalarInConfig(data, key, value)
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

func writeConfigMap(fs configFS, baseDir string, key string, values map[string]string) error {
	configFile := filepath.Join(baseDir, cfgFileDir, cfgFileName)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := upsertMapInConfig(data, key, values)
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

func upsertMapInConfig(configData []byte, key string, values map[string]string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected YAML mapping at root")
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

func upsertScalarInConfig(configData []byte, key string, value string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid YAML document structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected YAML mapping at root")
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
