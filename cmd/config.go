/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
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
			err := createConfig()
			if err != nil {
				return err
			}
			return nil
		case cmd.Flag("validate").Value.String() == "true":
			err := validateConfig()
			if err != nil {
				return err
			}
			fmt.Printf("Config file is valid\n")
			return nil
		case cmd.Flag("list-teams").Value.String() == "true":
			err := listPagerDutyTeams()
			if err != nil {
				return err
			}
			return nil
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
	configCmd.Flags().BoolP("list-teams", "l", false, "list your PagerDuty teams (requires valid token in config or SREPD_TOKEN env var)")
	configCmd.MarkFlagsMutuallyExclusive("create", "validate", "list-teams")
}

// createConfig creates the config directory and writes an example config file
func createConfig() error {
	const (
		cfgFile     = "srepd.yaml"
		cfgFilePath = ".config/srepd"
	)

	// Find home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Construct full config directory path
	configDir := filepath.Join(home, cfgFilePath)

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Construct full config file path
	configFile := filepath.Join(configDir, cfgFile)

	// Check if config file already exists
	if _, err := os.Stat(configFile); err == nil {
		return fmt.Errorf("config file already exists at %s", configFile)
	}

	// Write example config to file
	if err := os.WriteFile(configFile, []byte(exampleConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Config file created at %s\n", configFile)
	fmt.Println("Please edit the file to add your PagerDuty credentials and team information.")
	fmt.Println("\nTip: After adding your token, run 'srepd config --list-teams' to see available teams.")
	return nil
}

// listPagerDutyTeams fetches and displays all available PagerDuty teams
func listPagerDutyTeams() error {
	// Get token from config or environment variable
	token := viper.GetString("token")
	if token == "" || token == "<PagerDuty API token>" {
		return fmt.Errorf("no valid PagerDuty API token found. Please set 'token' in %s/.config/srepd/srepd.yaml or set SREPD_TOKEN environment variable", os.Getenv("HOME"))
	}

	fmt.Println("Fetching teams from PagerDuty...")

	teams, err := fetchPagerDutyTeams(token)
	if err != nil {
		return err
	}

	if len(teams) == 0 {
		fmt.Println("No teams found in your PagerDuty account.")
		return nil
	}

	// Display teams
	fmt.Printf("\nFound %d team(s):\n\n", len(teams))
	for _, team := range teams {
		fmt.Printf("  ID: %s\n  Name: %s\n\n", team.ID, team.Name)
	}

	// Show example config snippet
	fmt.Println("To add teams to your config, edit ~/.config/srepd/srepd.yaml and update the teams section:")
	fmt.Println("\nteams:")
	for i, team := range teams {
		if i < 3 { // Show first 3 as examples
			fmt.Printf("  - %s  # %s\n", team.ID, team.Name)
		}
	}
	if len(teams) > 3 {
		fmt.Println("  # ...")
	}

	return nil
}

// fetchPagerDutyTeams retrieves the current user's teams from PagerDuty API
func fetchPagerDutyTeams(token string) ([]pagerduty.Team, error) {
	// Create PagerDuty client
	client := pagerduty.NewClient(token)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current user with their teams
	opts := pagerduty.GetCurrentUserOptions{
		Includes: []string{"teams"},
	}

	user, err := client.GetCurrentUserWithContext(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current user from PagerDuty: %w", err)
	}

	if len(user.Teams) == 0 {
		return []pagerduty.Team{}, nil
	}

	return user.Teams, nil
}

// hasPlaceholderTeams checks if the teams list contains only placeholder values
func hasPlaceholderTeams() bool {
	teams := viper.GetStringSlice("teams")
	if len(teams) == 0 {
		return true
	}

	// Check if all teams are placeholder values
	for _, team := range teams {
		trimmed := strings.TrimSpace(team)
		if trimmed != "" && !strings.HasPrefix(trimmed, "<PagerDuty Team ID") {
			return false
		}
	}

	return true
}

// promptInteractiveTeamSelection prompts the user to select teams interactively
func promptInteractiveTeamSelection() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nNo teams found in config. This will result in zero tickets loaded in srepd.")
	fmt.Print("Would you like to add your teams now? (y/n): ")

	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response != "y" && response != "yes" {
		fmt.Println("Skipping team configuration. You can add teams later by editing the config file or running 'srepd config --list-teams'")
		return nil
	}

	// Get token
	token := viper.GetString("token")
	if token == "" || token == "<PagerDuty API token>" {
		return fmt.Errorf("no valid PagerDuty API token found in config. Please add your token to the config file first")
	}

	fmt.Println("\nFetching teams from PagerDuty...")
	teams, err := fetchPagerDutyTeams(token)
	if err != nil {
		return err
	}

	if len(teams) == 0 {
		fmt.Println("No teams found in your PagerDuty account.")
		return nil
	}

	// Display teams with numbers
	fmt.Printf("\nFound %d team(s):\n\n", len(teams))
	for i, team := range teams {
		fmt.Printf("%2d. %s - %s\n", i+1, team.ID, team.Name)
	}

	fmt.Println("\nEnter the numbers of the teams you want to add (comma-separated, e.g., 1,3,5):")
	fmt.Print("Teams: ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read team selection: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		fmt.Println("No teams selected. Skipping team configuration.")
		return nil
	}

	// Parse selections
	selections := strings.Split(input, ",")
	var selectedTeams []string

	for _, sel := range selections {
		sel = strings.TrimSpace(sel)
		num, err := strconv.Atoi(sel)
		if err != nil || num < 1 || num > len(teams) {
			fmt.Printf("Warning: Invalid selection '%s', skipping...\n", sel)
			continue
		}
		selectedTeams = append(selectedTeams, teams[num-1].ID)
	}

	if len(selectedTeams) == 0 {
		fmt.Println("No valid teams selected.")
		return nil
	}

	// Update config file
	err = updateConfigTeams(selectedTeams)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	fmt.Printf("\nSuccessfully added %d team(s) to config!\n", len(selectedTeams))
	for i, teamID := range selectedTeams {
		// Find the team name
		for _, team := range teams {
			if team.ID == teamID {
				fmt.Printf("  %d. %s - %s\n", i+1, teamID, team.Name)
				break
			}
		}
	}

	return nil
}

// updateConfigTeams updates only the teams list in the config file, preserving all other content
func updateConfigTeams(teams []string) error {
	const (
		cfgFile     = "srepd.yaml"
		cfgFilePath = ".config/srepd"
	)

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configFile := filepath.Join(home, cfgFilePath, cfgFile)

	// Read existing config
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	teamsIndentLevel := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Detect start of teams section
		if strings.HasPrefix(strings.TrimSpace(line), "teams:") {
			// Calculate indent level (number of leading spaces before "teams:")
			teamsIndentLevel = len(line) - len(strings.TrimLeft(line, " \t"))
			newLines = append(newLines, line)

			// Add the new teams
			for _, team := range teams {
				newLines = append(newLines, fmt.Sprintf("%s  - %s", strings.Repeat(" ", teamsIndentLevel), team))
			}

			// Skip old team entries (lines that start with "  -" after teams:)
			for i+1 < len(lines) {
				nextLine := lines[i+1]
				trimmed := strings.TrimSpace(nextLine)
				// Check if it's a team entry (starts with -)
				if trimmed != "" && strings.HasPrefix(trimmed, "-") {
					// Check if indentation matches teams section (is a direct child)
					indent := len(nextLine) - len(strings.TrimLeft(nextLine, " \t"))
					if indent > teamsIndentLevel {
						i++ // Skip this line
						continue
					}
				}
				// Not a team entry, stop skipping
				break
			}
			continue
		}

		// Keep all other lines
		newLines = append(newLines, line)
	}

	// Write back
	updatedData := strings.Join(newLines, "\n")
	err = os.WriteFile(configFile, []byte(updatedData), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Reload viper config
	viper.Set("teams", teams)

	return nil
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

	// If no errors so far and we have placeholder teams, prompt for interactive selection
	if len(errs) == 0 && hasPlaceholderTeams() {
		// Check if we have a valid token before prompting
		token := viper.GetString("token")
		if token != "" && token != "<PagerDuty API token>" {
			err := promptInteractiveTeamSelection()
			if err != nil {
				log.Warn("Failed to configure teams interactively", "error", err)
				// Don't add to errs - this is optional, user can configure later
			}
		}
	}

	return errors.Join(errs...)
}
