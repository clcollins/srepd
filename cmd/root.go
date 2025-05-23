/*
Copyright © 2023 Chris Collins 'collins.christopher@gmail.com'

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const tickInterval = 1

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "srepd",
	Short: "TUI for common SREP PagerDuty on-call tasks",
	Long: `'srepd' is a TUI application for common PagerDuty 
on-call tasks.  It is intended to be used by SREs to perform 
such tasks as acknowledging incidents, adding notes, 
reassigning to the next on-call, etc.  It is not intended
to be a full-featured PagerDuty client, or kitchen sink, 
but rather a simple tool to make on-call tasks easier.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		bindArgsToViper(cmd)

		log.SetLevel(func() log.Level {
			if viper.GetBool("debug") {
				return log.DebugLevel
			}
			return log.WarnLevel
		}())

		err := validateConfig()
		if err != nil {
			log.Fatal(err)
		}

	},
	Run: func(cmd *cobra.Command, args []string) {

		launcher, err := launcher.NewClusterLauncher(viper.GetString("terminal"), viper.GetString("cluster_login_command"))
		if err != nil {
			log.Warn(err)
		}

		m, _ := tui.InitialModel(
			viper.GetString("token"),
			viper.GetStringSlice("teams"),
			viper.GetStringMapString("service_escalation_policies"),
			viper.GetStringSlice("ignoredusers"), // TODO: replace this with escalationPolicy filter
			viper.GetStringSlice("editor"),
			launcher,
			viper.GetBool("debug"),
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
			log.Fatal(err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

// bindArgsToViper binds the command line arguments to viper
func bindArgsToViper(cmd *cobra.Command) {
	err := viper.BindPFlag("debug", cmd.Flags().Lookup("debug"))
	if err != nil {
		log.Fatal(err)
	}
}

type cliFlag struct {
	flagType  string
	name      string
	shorthand string
	value     string
	usage     string
}

func (f cliFlag) StringValue() string {
	return f.value
}

func (f cliFlag) BoolValue() bool {
	b, _ := strconv.ParseBool(f.value)
	return b
}

func init() {
	var flags = []cliFlag{
		{"bool", "debug", "d", "false", "enable debug logging (~/.config/srepd/debug.log)"},
		// TODO - For some reason the parsed cluster-login-command flag does not work (the "%%" is stripped out)
		// Commenting out the config options for now, as the config file is the preferred method
		// {"string", "token", "T", "", "PagerDuty API token"},
		// {"stringSlice", "teams", "t", []string{}, "teams to filter on"},
		// {"stringMapString", "service-escalation-policies", "s", map[string]string{}, "service to escalation policy mapping"},
		// {"stringSlice", "ignoredusers", "i", []string{}, "users to ignore"},
		// {"string", "editor", "e", defaultEditor, "editor to use for notes"},
		// {"string", "terminal", "t", defaultTerminal, "terminal to use for exec commands"},
		// {"stringSlice", "cluster-login-command", "c", defaultClusterLoginCmd, "cluster login command"},
	}

	cobra.OnInitialize(initConfig)

	for _, f := range flags {
		switch f.flagType {
		case "bool":
			rootCmd.Flags().BoolP(f.name, f.shorthand, f.BoolValue(), f.usage)
		case "string":
			rootCmd.Flags().StringP(f.name, f.shorthand, f.StringValue(), f.usage)
		case "stringSlice":
			rootCmd.Flags().StringSliceP(f.name, f.shorthand, []string{f.StringValue()}, f.usage)
		}
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	const (
		cfgFile     = "srepd.yaml"
		cfgFilePath = ".config/srepd/"
	)

	// Find home directory.
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	// Search config in home directory with name ".srepd" (without extension).
	viper.AddConfigPath(home + "/" + cfgFilePath)
	viper.SetConfigName(cfgFile)
	viper.SetConfigType("yaml")

	viper.SetEnvPrefix("srepd")
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintln(os.Stderr, "Config file not found: "+err.Error())
		} else {
			fmt.Fprintln(os.Stderr, "Config file error: "+err.Error())
		}
	}
}
