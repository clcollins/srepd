/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
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

	var ocmClient ocm.OCMClient
	var ocmAuthPending bool
	var asyncOCMClient *ocm.Client

	cfg, armed, checkErr := ocm.CheckTokens()
	if checkErr != nil {
		log.Warn("OCM config check failed", "error", checkErr)
	} else if armed {
		client, connErr := ocm.NewClientFromConfig(cfg, tui.Version)
		if connErr != nil {
			log.Warn("OCM connection failed", "error", connErr)
		} else {
			ocmClient = client
			asyncOCMClient = client
			log.Info("OCM connected")
		}
	} else {
		ocmAuthPending = true
		log.Info("OCM tokens not valid — will authenticate async")
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
		ocmAuthPending,
		nil, // aiProvider — not needed in config mode
		"",  // agentCLICommand — not needed in config mode
		nil, // backplaneClient — not needed in config mode
		nil, // backplaneConfig — not needed in config mode
	)

	p := tea.NewProgram(m, tea.WithAltScreen())

	if ocmAuthPending {
		go func() {
			fmt.Fprintln(os.Stderr, "OCM tokens expired — opening browser for authentication...")
			token, authErr := ocm.AuthenticateAsync(cfg)
			if authErr != nil {
				log.Debug("OCM browser auth failed", "error", authErr)
				p.Send(tui.OCMClientReadyMsg{Err: authErr})
				return
			}
			fmt.Fprintln(os.Stderr, "OCM authentication successful.")
			ocm.ApplyAuthToken(cfg, token)
			client, connErr := ocm.NewClientFromConfig(cfg, tui.Version)
			if connErr != nil {
				p.Send(tui.OCMClientReadyMsg{Err: connErr})
				return
			}
			asyncOCMClient = client
			p.Send(tui.OCMClientReadyMsg{Client: client})
		}()
	}

	go func() {
		for {
			time.Sleep(tickInterval * time.Second)
			p.Send(tui.TickMsg{})
		}
	}()

	_, err = p.Run()

	if asyncOCMClient != nil {
		asyncOCMClient.Close()
	}

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
