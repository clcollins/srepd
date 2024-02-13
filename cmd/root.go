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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	cfgFile     = "srepd.yaml"
	cfgFilePath = ".config/srepd/"

	// Must not be aliases - must be real commands or links
	defaultEditor          = "/usr/bin/vim"
	defaultTerminal        = "/usr/bin/gnome-terminal"
	defaultShell           = "/bin/bash"
	defaultClusterLoginCmd = "/usr/local/bin/ocm backplane login"
)

var (
	debug                 bool
	editor                string
	terminal              string
	shell                 string
	cluster_login_command string
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

	Run: func(cmd *cobra.Command, args []string) {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()

		if debug {
			for k, v := range viper.GetViper().AllSettings() {
				if k == "token" {
					v = "*****"
				}
				log.Printf("Found key: `%v`, value: `%v`\n", k, v)
			}
		}

		token := viper.GetString("token")
		teams := viper.GetStringSlice("teams")
		silentuser := viper.GetString("silentuser")
		ignoredusers := viper.GetStringSlice("ignoredusers")

		// The environment variable will always override the config file if set
		if editor == "" {
			editor = viper.GetString("editor")
			if editor == "" {
				editor = defaultEditor
			}
		}

		if terminal == "" {
			terminal = viper.GetString("terminal")
			if terminal == "" {
				terminal = defaultTerminal
			}
		}

		if shell == "" {
			shell = viper.GetString("shell")
			if shell == "" {
				shell = defaultShell
			}
		}

		if cluster_login_command == "" {
			cluster_login_command = viper.GetString("cluster_login_command")
			if cluster_login_command == "" {
				cluster_login_command = defaultClusterLoginCmd
			}
		}

		if debug {
			log.Printf("Using editor: `%v`\n", editor)
			log.Printf("Using terminal: `%v`\n", terminal)
			log.Printf("Using shell: `%v`\n", shell)
			log.Printf("Using ClusterLoginCommand: `%v`\n", cluster_login_command)
		}

		m, _ := tui.InitialModel(
			token,
			teams,
			silentuser,
			ignoredusers,
			editor,
			tui.ClusterLauncher{
				Terminal:            terminal,
				Shell:               shell,
				ClusterLoginCommand: cluster_login_command,
			},
		)

		p := tea.NewProgram(m, tea.WithAltScreen())
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
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debugging output")
	rootCmd.PersistentFlags().StringVarP(&editor, "editor", "e", "", "Editor to use for notes; default: `$EDITOR`")
	rootCmd.PersistentFlags().StringVarP(&terminal, "terminal", "t", "", "Terminal to use for exec commands; default: `/usr/bin/gnome-terminal`")
	rootCmd.PersistentFlags().StringVarP(&shell, "shell", "s", "", "Shell to use for exec commands; default: `$SHELL`")
	rootCmd.PersistentFlags().StringVarP(&cluster_login_command, "cluster_login_cmd", "c", "", "Cluster login command; default: `/usr/local/bin/ocm backplane login`")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
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
