/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/deprecation"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const description = `The config command is used to create or validate the SREPD config file.
The config file is located at ~/.config/srepd/srepd.yaml and is used to store
the configuration options for the SREPD application.`

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

func ensureViperDefaults() {
	for k, v := range pkgconfig.DefaultOptionalKeys {
		if viper.GetString(k) == "" {
			viper.Set(k, v)
		}
	}
}

func runConfigWizard() error {
	if viper.GetBool("dev") {
		runDevMode()
		return nil
	}

	ensureViperDefaults()
	launchTUIWithConfig()
	return nil
}

// launchTUIWithConfig launches the TUI with configMode enabled, so it
// enters the inline config wizard instead of normal incident view.
func launchTUIWithConfig() {
	l, err := launcher.NewClusterLauncher(viper.GetString("terminal"), viper.GetString("cluster_login_command"), viper.GetString("toolbox_mode"))
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	ocmClient, ocmErr := ocm.NewClient(tui.Version)
	if ocmErr != nil {
		log.Warn("OCM connection failed", "error", ocmErr)
	} else if ocmClient != nil {
		defer ocmClient.Close()
		log.Info("OCM connected")
	} else {
		log.Warn("OCM not configured")
	}

	m, _ := tui.InitialModel(
		viper.GetString("token"),
		viper.GetStringSlice("teams"),
		viper.GetStringMapString("service_escalation_policies"),
		viper.GetStringSlice("ignoredusers"),
		viper.GetStringSlice("editor"),
		l,
		viper.GetBool("debug"),
		ocmClient,
		viper.GetStringMapString("colors"),
		viper.GetString("default_silent_escalation_policy"),
		viper.GetStringMapString("custom_service_escalation_policies"),
		true, // configMode
	)

	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		for {
			time.Sleep(tickInterval * time.Second)
			p.Send(tui.TickMsg{})
		}
	}()

	_, err = p.Run()
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}
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

	for k, v := range pkgconfig.RequiredKeys {
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

	for k := range pkgconfig.OptionalKeys {
		_, ok := settings[k]
		if !ok {
			log.Debug("cmd.validateConfig()", "msg", "missing optional key", "key", k, "default_value", pkgconfig.DefaultOptionalKeys[k])
			viper.Set(k, pkgconfig.DefaultOptionalKeys[k])
		}
	}

	return errors.Join(errs...)
}
