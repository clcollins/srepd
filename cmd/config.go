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

const (
	exampleConfig = `
# Example srepd configuration file
---
# This is an example configuration file for srepd.  It is intended to be used
# as a reference for the configuration options available to the user.  The
# configuration file is located at ~/.config/srepd/srepd.yaml

# Required configuration options

# PagerDuty API token
token: <PagerDuty API token>

# Teams to filter on
teams:
  - <PagerDuty Team ID 1>
  - <PagerDuty Team ID 2>

# Default silent escalation policy — auto-discovered via --pick-teams
# or set manually. Used when silencing incidents.
# default_silent_escalation_policy: <PagerDuty Escalation Policy ID>

# Optional configuration options

# Per-service silent policy overrides (service ID → silent policy ID)
# custom_service_escalation_policies:
#   <PagerDuty Service ID>: <PagerDuty Escalation Policy ID>

# Editor to use for notes
editor: vim

# Terminal to use for exec commands
terminal: gnome-terminal --

# Cluster login command
cluster-login-command: ocm backplane login %%CLUSTER_ID%%

# Toolbox mode: auto-detect Fedora Toolbox and prefix terminal commands
# with flatpak-spawn --host. Values: "auto" (default), "true", "false"
toolbox_mode: auto

# Custom color scheme (all optional, hex values)
# colors:
#   text: "#778da9"
#   border: "#415a77"
#   highlight: "#ffffff"
#   selected: "#415a77"
#   warning: "#a4133c"
#   error: "#0d1b2a"
#   muted: "#5C5C5C"
#   tab: "#7D56F4"`
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
		"cluster_login_command":              fmt.Sprintf("Cluster login command (default: %v)", defaultOptionalKeys["cluster-login-command"]),
		"toolbox_mode":                       fmt.Sprintf("Toolbox detection mode: auto, true, false (default: %v)", defaultOptionalKeys["toolbox_mode"]),
		"chord_prefix":                       fmt.Sprintf("Chord prefix key for multi-key commands (default: %v)", defaultOptionalKeys["chord_prefix"]),
		"colors":                             "Custom color scheme (map of color name to hex value)",
		"default_silent_escalation_policy":   "Default silent escalation policy ID (auto-discovered via --pick-teams)",
		"custom_service_escalation_policies": "Per-service silent policy overrides (service ID → policy ID)",
	}
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:          "config",
	Short:        "Create or validate the SREPD config file",
	Long:         description + "\n\n" + exampleConfig,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch {
		case cmd.Flag("create").Value.String() == "true":
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}
			if err := createConfig(osFS{}, home); err != nil {
				return err
			}
			fmt.Println("\nTip: After adding your token, run 'srepd config --pick-teams' to see available teams.")
			return nil
		case cmd.Flag("validate").Value.String() == "true":
			err := validateConfig()
			if err != nil {
				return err
			}
			fmt.Printf("Config file is valid\n")
			return nil
		case cmd.Flag("pick-teams").Value.String() == "true":
			return runPickTeams()
		default:
			err := cmd.Usage()
			return err
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// configCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// configCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	configCmd.Flags().BoolP("create", "c", false, "create a sample config file at ~/.config/srepd/srepd.yaml")
	configCmd.Flags().BoolP("validate", "v", false, "validate the config file")
	configCmd.Flags().BoolP("pick-teams", "p", false, "select your PagerDuty teams interactively")
	configCmd.MarkFlagsMutuallyExclusive("create", "validate", "pick-teams")
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

func createConfig(fs configFS, baseDir string) (retErr error) {
	configDir := filepath.Join(baseDir, cfgFileDir)

	if err := fs.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile := filepath.Join(configDir, cfgFileName)

	f, err := fs.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("config file already exists at %s", configFile)
		}
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close config file: %w", cerr)
		}
	}()

	if _, err := f.Write([]byte(strings.TrimLeft(exampleConfig, "\n"))); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Config file created at %s\n", configFile)
	fmt.Println("Please edit the file to add your PagerDuty credentials and team information.")
	return nil
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

func runPickTeams() error {
	if viper.GetBool("dev") {
		fmt.Println("Dev mode: skipping team selection, launching with fixture data...")
		runDevMode()
		return nil
	}

	token := viper.GetString("token")
	if token == "" {
		return fmt.Errorf("no PagerDuty API token found; set 'token' in config or SREPD_TOKEN env var")
	}

	client := pd.NewClient(token)
	teams, err := pd.GetCurrentUserTeams(client)
	if err != nil {
		return err
	}

	if len(teams) == 0 {
		fmt.Println("No teams found in your PagerDuty account.")
		return nil
	}

	selectedIDs, err := runTeamSelector(teams)
	if err != nil {
		return err
	}

	if len(selectedIDs) == 0 {
		fmt.Println("No teams selected.")
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	teamNames := make(map[string]string)
	for _, team := range teams {
		teamNames[team.ID] = team.Name
	}

	if err := writeConfigTeams(osFS{}, home, selectedIDs, teamNames); err != nil {
		return err
	}

	fmt.Printf("\nSaved %d team(s) to config.\n", len(selectedIDs))

	silentPolicyID, err := promptForSilentPolicy(client)
	if err != nil {
		return err
	}
	if silentPolicyID != "" {
		if err := writeConfigKey(osFS{}, home, "default_silent_escalation_policy", silentPolicyID); err != nil {
			return fmt.Errorf("failed to save silent policy to config: %w", err)
		}
		fmt.Println("Saved default silent policy to config.")
	}

	customPolicies, err := promptForCustomPolicies(client)
	if err != nil {
		return err
	}
	if len(customPolicies) > 0 {
		if err := writeConfigMap(osFS{}, home, "custom_service_escalation_policies", customPolicies); err != nil {
			return fmt.Errorf("failed to save custom policies to config: %w", err)
		}
		fmt.Printf("Saved %d custom service policy mapping(s) to config.\n", len(customPolicies))
	}

	fmt.Println("\nSetup complete. Run 'srepd' to start.")
	return nil
}

func runTeamSelector(teams []pagerduty.Team) ([]string, error) {
	var selected []string
	var options []huh.Option[string]
	for _, team := range teams {
		options = append(options, huh.NewOption(fmt.Sprintf("%s — %s", team.Name, team.ID), team.ID))
	}

	colors := viper.GetStringMapString("colors")
	theme := tui.ThemeFromConfig(colors)

	huhTheme := huh.ThemeCharm()
	huhTheme.Focused.Title = huhTheme.Focused.Title.Foreground(theme.Highlight)
	huhTheme.Focused.Description = huhTheme.Focused.Description.Foreground(theme.Muted)
	huhTheme.Focused.SelectedOption = huhTheme.Focused.SelectedOption.Foreground(theme.Highlight)
	huhTheme.Focused.UnselectedOption = huhTheme.Focused.UnselectedOption.Foreground(theme.Text)
	huhTheme.Focused.MultiSelectSelector = huhTheme.Focused.MultiSelectSelector.Foreground(theme.Text)
	huhTheme.Focused.SelectedPrefix = huhTheme.Focused.SelectedPrefix.Foreground(theme.Highlight)
	huhTheme.Focused.UnselectedPrefix = huhTheme.Focused.UnselectedPrefix.Foreground(theme.Muted)
	huhTheme.Focused.Base = huhTheme.Focused.Base.BorderForeground(theme.Border)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select your PagerDuty teams").
				Description("Use space to toggle, enter to confirm, esc to skip").
				Options(options...).
				Value(&selected),
		),
	).WithTheme(huhTheme).WithProgramOptions(tea.WithAltScreen())

	err := form.Run()
	if err != nil {
		return nil, fmt.Errorf("team selection failed: %w", err)
	}

	if form.State == huh.StateAborted {
		return nil, nil
	}

	return selected, nil
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

func promptForSilentPolicy(client pd.PagerDutyClient) (string, error) {
	var policyID string

	fmt.Println("\n--- Default Silent Escalation Policy ---")
	fmt.Println("When you silence an incident, it gets reassigned to a silent escalation")
	fmt.Println("policy (one that routes only to bot users, not on-call humans).")
	fmt.Println("Find this ID in PagerDuty → Escalation Policies (e.g., \"Silent Test\").")
	fmt.Println("Leave blank to configure later in your config file.")

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter the ID of your default silent escalation policy").
				Description("Leave blank to skip").
				Value(&policyID),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("silent policy prompt failed: %w", err)
	}

	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		fmt.Println("Skipped — you can set 'default_silent_escalation_policy' in your config later.")
		return "", nil
	}

	policy, err := pd.GetEscalationPolicy(client, policyID, pagerduty.GetEscalationPolicyOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to validate policy ID %q: %w", policyID, err)
	}

	fmt.Printf("Confirmed: %s — %s\n", policy.ID, policy.Name)
	return policyID, nil
}

func promptForCustomPolicies(client pd.PagerDutyClient) (map[string]string, error) {
	var input string

	fmt.Println("\n--- Custom Service-to-Policy Mappings ---")
	fmt.Println("Some services may use a different silent policy than the default.")
	fmt.Println("For example, DMS alerts might route to a DMS-specific silent policy")
	fmt.Println("instead of the general one.")
	fmt.Println("Enter mappings as SERVICE_ID:POLICY_ID separated by commas.")
	fmt.Println("Leave blank to skip.")

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Custom mappings (SERVICE_ID:POLICY_ID, ...)").
				Description("Leave blank to skip").
				Value(&input),
		),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("custom policy prompt failed: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	result := make(map[string]string)
	for _, pair := range strings.Split(input, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping %q — expected SERVICE_ID:POLICY_ID", pair)
		}
		svcID := strings.TrimSpace(parts[0])
		polID := strings.TrimSpace(parts[1])

		policy, err := pd.GetEscalationPolicy(client, polID, pagerduty.GetEscalationPolicyOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to validate policy %q for service %q: %w", polID, svcID, err)
		}
		fmt.Printf("  %s → %s — %s\n", svcID, policy.ID, policy.Name)
		result[svcID] = polID
	}

	return result, nil
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
