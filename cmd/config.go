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

	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/deprecation"
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

# Service to Escalation Policy mapping
# The map may have 1 or more optional PagerDuty services to Escalation Policy mappings, but
# must have a DEFAULT and SILENT_DEFAULT key.  The DEFAULT key is used for the default escalation policy
# and the SILENT_DEFAULT key is used for the default escalation policy when the user wants to suppress
# notifications.
service_escalation_policies:
  DEFAULT: <PagerDuty Escalation Policy ID 1>
  SILENT_DEFAULT: <PagerDuty Escalation Policy ID 2>
  <PagerDuty Service ID 1>: <PagerDuty Escalation Policy ID 3>

# Optional configuration options
# User to ignore - Alerts assigned to these users are ignored
ignoredusers:
  - <PagerDuty User ID 1>
  - <PagerDuty User ID 2>

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
		"token":                       "PagerDuty API token",
		"teams":                       "PagerDuty team IDs to filter on",
		"service_escalation_policies": "Service to Escalation Policy mapping (pagerduty_service_id:pagerduty_escalation_policy_id); Requires 'DEFAULT' and 'SILENT_DEFAULT' keys",
	}
	defaultOptionalKeys = map[string]string{
		"editor":                "vim",
		"terminal":              "gnome-terminal --",
		"cluster_login_command": "ocm backplane login %%CLUSTER_ID%%",
		"toolbox_mode":          "auto",
		"chord_prefix":          "ctrl+x",
	}
	optionalKeys = map[string]string{
		"ignoredusers":          fmt.Sprintf("PagerDuty user IDs to ignore (default: %v)", "None"),
		"editor":                fmt.Sprintf("Editor to use for notes (default: %v)", defaultOptionalKeys["editor"]),
		"terminal":              fmt.Sprintf("Terminal to use for exec commands (default: %v)", defaultOptionalKeys["terminal"]),
		"cluster_login_command": fmt.Sprintf("Cluster login command (default: %v)", defaultOptionalKeys["cluster-login-command"]),
		"toolbox_mode":          fmt.Sprintf("Toolbox detection mode: auto, true, false (default: %v)", defaultOptionalKeys["toolbox_mode"]),
		"chord_prefix":          fmt.Sprintf("Chord prefix key for multi-key commands (default: %v)", defaultOptionalKeys["chord_prefix"]),
		"colors":                "Custom color scheme (map of color name to hex value)",
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
			return createConfig(osFS{}, home)
		case cmd.Flag("validate").Value.String() == "true":
			err := validateConfig()
			if err != nil {
				return err
			}
			fmt.Printf("Config file is valid\n")
			return nil
		case cmd.Flag("create").Value.String() == "true" && cmd.Flag("validate").Value.String() == "true":
			return errors.New("cannot use both --create and --validate flags together")
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
	configCmd.MarkFlagsMutuallyExclusive("create", "validate")
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

func updateTeamsInConfig(configData []byte, teamIDs []string) ([]byte, error) {
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
					teamsValue.Content = append(teamsValue.Content, &yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: id,
					})
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

func writeConfigTeams(fs configFS, baseDir string, teamIDs []string) error {
	configFile := filepath.Join(baseDir, cfgFileDir, cfgFileName)

	data, err := fs.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	updated, err := updateTeamsInConfig(data, teamIDs)
	if err != nil {
		return err
	}

	if err := fs.WriteFile(configFile, updated, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
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
