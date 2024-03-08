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
	"log"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
	},
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		f, err := tea.LogToFile(home+"/.config/srepd/debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()

		if viper.GetBool("debug") {
			log.Printf("Debugging enabled\n")
			for k, v := range viper.GetViper().AllSettings() {
				if k == "token" {
					v = "*****"
				}
				log.Printf("Found key: `%v`, value: `%v`\n", k, v)
			}
		}

		m, _ := tui.InitialModel(
			viper.GetBool("debug"),
			viper.GetString("token"),
			viper.GetStringSlice("teams"),
			viper.GetString("silentuser"),
			viper.GetStringSlice("ignoredusers"),
			viper.GetStringSlice("editor"),
			tui.ClusterLauncher{
				Terminal:            viper.GetStringSlice("terminal"),
				Shell:               viper.GetStringSlice("shell"),
				ClusterLoginCommand: viper.GetStringSlice("cluster_login_command"),
			},
		)

		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// bindArgsToViper binds the command line arguments to viper
func bindArgsToViper(cmd *cobra.Command) {
	viper.BindPFlag("debug", cmd.Flags().Lookup("debug"))
	viper.BindPFlag("editor", cmd.Flags().Lookup("editor"))
	viper.BindPFlag("terminal", cmd.Flags().Lookup("terminal"))
	viper.BindPFlag("shell", cmd.Flags().Lookup("shell"))
	viper.BindPFlag("cluster_login_command", cmd.Flags().Lookup("clusterLoginCommand"))
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
	// Must not be aliases - must be real commands or links
	const (
		defaultEditor          = "/usr/bin/vim"
		defaultTerminal        = "/usr/bin/gnome-terminal"
		defaultShell           = "/bin/bash"
		defaultClusterLoginCmd = "/usr/local/bin/ocm backplane login"
	)

	var flags = []cliFlag{
		{"bool", "debug", "d", "false", "Enable debug logging (~/.config/srepd/debug.log)"},
		{"string", "editor", "e", defaultEditor, "Editor to use for notes; $EDITOR takes precedence"},
		{"string", "terminal", "t", defaultTerminal, "Terminal to use for exec commands"},
		{"string", "shell", "s", defaultShell, "Shell to use for exec commands; $SHELL takes precedence"},
		{"string", "clusterLoginCmd", "c", defaultClusterLoginCmd, "Cluster login command"},
	}

	cobra.OnInitialize(initConfig)

	for _, f := range flags {
		switch f.flagType {
		case "bool":
			rootCmd.Flags().BoolP(f.name, f.shorthand, f.BoolValue(), f.usage)
		case "string":
			rootCmd.Flags().StringP(f.name, f.shorthand, f.StringValue(), f.usage)
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
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintln(os.Stderr, "Config file not found: "+err.Error())
		} else {
			fmt.Fprintln(os.Stderr, "Config file error: "+err.Error())
		}
	}
}
