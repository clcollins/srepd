/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/deprecation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
cluster-login-command: ocm backplane login %%CLUSTER_ID%%`
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
	}
	optionalKeys = map[string]string{
		"ignoredusers":          fmt.Sprintf("PagerDuty user IDs to ignore (default: %v)", "None"),
		"editor":                fmt.Sprintf("Editor to use for notes (default: %v)", defaultOptionalKeys["editor"]),
		"terminal":              fmt.Sprintf("Terminal to use for exec commands (default: %v)", defaultOptionalKeys["terminal"]),
		"cluster_login_command": fmt.Sprintf("Cluster login command (default: %v)", defaultOptionalKeys["cluster-login-command"]),
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
			fmt.Println(exampleConfig)
			return nil
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

	configCmd.Flags().BoolP("create", "c", false, "print a sample config file")
	configCmd.Flags().BoolP("validate", "v", false, "validate the config file")
	configCmd.MarkFlagsMutuallyExclusive("create", "validate")
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
			log.Warn("missing optional key: " + k + "; using default value " + defaultOptionalKeys[k])
			viper.Set(k, defaultOptionalKeys[k])
		}
	}

	return errors.Join(errs...)
}
