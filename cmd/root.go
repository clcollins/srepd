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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/clcollins/srepd/pkg/tui"
	"github.com/coreos/go-systemd/journal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const tickInterval = 1

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "srepd",
	Short:   "TUI for common SREP PagerDuty on-call tasks",
	Version: tui.Version + " (" + tui.GitSHA + ")",
	Long: `'srepd' is a TUI application for common PagerDuty
on-call tasks.  It is intended to be used by SREs to perform
such tasks as acknowledging incidents, adding notes,
reassigning to the next on-call, etc.  It is not intended
to be a full-featured PagerDuty client, or kitchen sink,
but rather a simple tool to make on-call tasks easier.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		bindArgsToViper(cmd)

		log.SetLevel(func() log.Level {
			if viper.GetBool("debug") {
				return log.DebugLevel
			}
			return log.WarnLevel
		}())
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		if viper.GetBool("dev") {
			log.Info("Dev mode enabled: skipping config validation")
			return
		}

		home, _ := os.UserHomeDir()
		configFile := filepath.Join(home, pkgconfig.CfgFileDir, pkgconfig.CfgFileName)
		if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
			return
		}

		err := validateConfig()
		if err != nil {
			log.Fatal(err)
		}

		if _, err := autoCommentOldPolicies(configFile); err != nil {
			log.Warn("Failed to auto-migrate old config format", "error", err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {

		if viper.GetBool("dev") {
			runDevMode()
			return
		}

		home, _ := os.UserHomeDir()
		configFile := filepath.Join(home, pkgconfig.CfgFileDir, pkgconfig.CfgFileName)
		if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
			ensureViperDefaults()
			launchTUIWithConfig()
			return
		}

		launchTUI()
	},
}

func autoCommentOldPolicies(configFile string) (bool, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}

	content := string(data)
	hasOld := strings.Contains(content, "\nservice_escalation_policies:") || strings.HasPrefix(content, "service_escalation_policies:")
	hasNew := strings.Contains(content, "default_silent_escalation_policy:")

	if !hasOld || !hasNew {
		return false, nil
	}

	migrated := pkgconfig.CommentOutOldPolicies(data)
	if bytes.Equal(data, migrated) {
		return false, nil
	}

	if err := os.WriteFile(configFile, migrated, 0644); err != nil {
		return false, fmt.Errorf("failed to write migrated config: %w", err)
	}

	log.Info("Auto-commented deprecated service_escalation_policies block")
	return true, nil
}

func launchTUI() {
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
		false,
		ocmAuthPending,
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

// logWriter holds the active asyncWriter so it can be flushed on shutdown.
var logWriter *asyncWriter

// asyncWriterBufferSize is the number of log messages that can be buffered
// before the asyncWriter starts dropping messages.
const asyncWriterBufferSize = 5000

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	defer CleanupLogging()

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

// CleanupLogging flushes and closes the async log writer.
// It is safe to call multiple times.
func CleanupLogging() {
	if logWriter != nil {
		logWriter.Close() //nolint:errcheck
	}
}

// bindArgsToViper binds the command line arguments to viper
func bindArgsToViper(cmd *cobra.Command) {
	root := cmd.Root()
	err := viper.BindPFlag("debug", root.PersistentFlags().Lookup("debug"))
	if err != nil {
		log.Fatal(err)
	}

	err = viper.BindPFlag("dev", root.PersistentFlags().Lookup("dev"))
	if err != nil {
		log.Fatal(err)
	}
	err = viper.BindPFlag("fixtures_dir", root.PersistentFlags().Lookup("fixtures-dir"))
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
		{"bool", "debug", "d", "false", "enable debug logging"},
		{"bool", "dev", "D", "false", "enable dev mode with fixture data (no PagerDuty connection required)"},
		{"string", "fixtures-dir", "F", "testdata/fixtures", "path to fixture data directory for dev mode"},
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

	cobra.OnInitialize(initConfig, configureLogging)

	for _, f := range flags {
		switch f.flagType {
		case "bool":
			rootCmd.PersistentFlags().BoolP(f.name, f.shorthand, f.BoolValue(), f.usage)
		case "string":
			rootCmd.PersistentFlags().StringP(f.name, f.shorthand, f.StringValue(), f.usage)
		case "stringSlice":
			rootCmd.PersistentFlags().StringSliceP(f.name, f.shorthand, []string{f.StringValue()}, f.usage)
		}
	}
}

// defaultFixturesDir is the path to fixture data relative to the binary's working directory.
// It can be overridden by the SREPD_FIXTURES_DIR environment variable.
const defaultFixturesDir = "testdata/fixtures"

// runDevMode starts the TUI in development mode using fixture data instead of live PagerDuty.
func runDevMode() {
	fixturesDir := viper.GetString("fixtures_dir")
	if fixturesDir == "" {
		fixturesDir = defaultFixturesDir
	}

	log.Info("Dev mode: loading fixtures", "dir", fixturesDir)

	config, err := pd.NewDevConfig(fixturesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load dev fixtures: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nRun srepd --dev from the repository root, or set SREPD_FIXTURES_DIR:\n")
		fmt.Fprintf(os.Stderr, "  SREPD_FIXTURES_DIR=/path/to/testdata/fixtures srepd --dev\n\n")
		log.Fatal(err)
	}

	// Create a dev launcher that logs instead of executing
	devLauncher, err := launcher.NewClusterLauncherWithToolbox(
		"echo dev-mode",
		"echo %%CLUSTER_ID%%",
		"false",
		func() bool { return false },
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dev launcher: %v\n", err)
		log.Fatal(err)
	}

	// Load OCM mock client with fixture data for dev mode
	ocmMock, ocmErr := ocm.LoadMockClientFromFixtures(fixturesDir)
	if ocmErr != nil {
		log.Warn("Dev mode: OCM fixtures not loaded", "error", ocmErr)
	}

	m, _ := tui.InitialModelWithConfig(
		config,
		viper.GetStringSlice("editor"),
		devLauncher,
		viper.GetBool("debug"),
		ocmMock,
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

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Find home directory.
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	// Search config in home directory with name ".srepd" (without extension).
	viper.AddConfigPath(filepath.Join(home, pkgconfig.CfgFileDir))
	viper.SetConfigName(pkgconfig.CfgFileName)
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

// LogDestination represents where logs should be written
type LogDestination int

const (
	LogToJournal LogDestination = iota
	LogToFile
	LogToStderr
)

// determineLogDestination returns the appropriate log destination based on OS and config
func determineLogDestination(goos string, logToJournal bool, journalEnabled bool) (LogDestination, string) {
	switch goos {
	case "linux":
		if logToJournal {
			if journalEnabled {
				return LogToJournal, ""
			}
			return LogToFile, "/var/log/srepd.log"
		}
		// User explicitly wants file logging
		return LogToFile, "~/.config/srepd/debug.log"

	case "darwin":
		return LogToFile, "~/Library/Logs/srepd.log"

	default:
		return LogToStderr, ""
	}
}

func configureLogging() {
	log.SetPrefix("srepd")

	// Check if user wants to log to journal (default: true)
	viper.SetDefault("log_to_journal", true)
	logToJournal := viper.GetBool("log_to_journal")

	dest, logPath := determineLogDestination(runtime.GOOS, logToJournal, journal.Enabled())

	switch dest {
	case LogToJournal:
		logWriter = newAsyncWriter(journalWriter{}, asyncWriterBufferSize)
		log.SetOutput(logWriter)
		log.Info("Logging to systemd journal")

	case LogToFile:
		// Expand home directory if needed
		if strings.HasPrefix(logPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Fatal("Failed to get user home directory:", err)
			}
			logPath = home + logPath[1:]
		}
		setupFileLogging(logPath)
		log.Info("Logging to " + logPath)

	case LogToStderr:
		logWriter = newAsyncWriter(os.Stderr, asyncWriterBufferSize)
		log.SetOutput(logWriter)
		log.Warn("Unsupported OS: logging to stderr")
	}
}

// setupFileLogging configures logging to a file, wrapped in asyncWriter
// to prevent log I/O from blocking the TUI.
func setupFileLogging(filePath string) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}
	logWriter = newAsyncWriter(file, asyncWriterBufferSize)
	log.SetOutput(logWriter)
}

// syslogIdentifier is the identifier used for systemd journal entries,
// enabling log retrieval via "journalctl -t srepd".
const syslogIdentifier = "srepd"

// journalWriter implements io.Writer for systemd journal
type journalWriter struct{}

func (jw journalWriter) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))
	priority := journalPriority(message)
	vars := map[string]string{
		"SYSLOG_IDENTIFIER": syslogIdentifier,
	}
	err = journal.Send(message, priority, vars)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// journalPriority maps log message content to the appropriate systemd
// journal priority level. It inspects the message for common log level
// prefixes emitted by the charmbracelet/log package.
func journalPriority(message string) journal.Priority {
	upper := strings.ToUpper(message)
	switch {
	case strings.Contains(upper, "ERROR"), strings.Contains(upper, "ERRO"):
		return journal.PriErr
	case strings.Contains(upper, "WARN"):
		return journal.PriWarning
	case strings.Contains(upper, "DEBUG"):
		return journal.PriDebug
	default:
		return journal.PriInfo
	}
}
