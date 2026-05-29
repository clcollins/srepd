package launcher

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/container"
)

const clusterLoginCommandFlag = "cluster_login_command"
const terminalFlag = "terminal"

type ClusterLauncher struct {
	Enabled             bool
	terminal            []string
	clusterLoginCommand []string
	runInToolbox        bool
	settings            launcherSettings
}

type launcherSettings struct {
	// Future Usage Possibly
}

// NewClusterLauncher creates a new ClusterLauncher with automatic toolbox
// detection using the default "auto" mode. When running inside a Fedora
// Toolbox container, terminal commands are automatically prefixed with
// "flatpak-spawn --host" so they execute on the host system.
func NewClusterLauncher(terminal string, clusterLoginCommand string, toolboxMode string) (ClusterLauncher, error) {
	return NewClusterLauncherWithToolbox(terminal, clusterLoginCommand, toolboxMode, container.IsRunningInToolbox)
}

// NewClusterLauncherWithToolbox creates a new ClusterLauncher with an
// injectable toolbox detection function, enabling unit testing without
// relying on actual environment state. The toolboxMode parameter controls
// behavior: "auto" (or "") uses detectFn, "true" forces toolbox mode on,
// "false" forces it off.
func NewClusterLauncherWithToolbox(terminal string, clusterLoginCommand string, toolboxMode string, detectFn func() bool) (ClusterLauncher, error) {
	inToolbox := resolveToolboxMode(toolboxMode, detectFn)

	launcher := ClusterLauncher{
		terminal:            strings.Split(terminal, " "),
		clusterLoginCommand: strings.Split(clusterLoginCommand, " "),
		runInToolbox:        inToolbox,
		settings:            launcherSettings{},
	}

	if inToolbox {
		log.Info("Toolbox detected: terminal commands will be prefixed with flatpak-spawn --host")
	}

	err := launcher.validate()
	if err != nil {
		return ClusterLauncher{}, err
	}

	return launcher, nil
}

// resolveToolboxMode determines whether toolbox wrapping should be enabled
// based on the configuration mode and the detection function result.
func resolveToolboxMode(mode string, detectFn func() bool) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "true":
		return true
	case "false":
		return false
	case "auto", "":
		return detectFn()
	default:
		log.Warn("Unknown toolbox_mode value, falling back to auto", "value", mode)
		return detectFn()
	}
}

func (l *ClusterLauncher) validate() error {
	errs := []error{}

	if l.terminal == nil || l.terminal[0] == "" {
		errs = append(errs, fmt.Errorf("%s is not set", terminalFlag))
	}

	if l.clusterLoginCommand == nil || l.clusterLoginCommand[0] == "" {
		errs = append(errs, fmt.Errorf("%s is not set", clusterLoginCommandFlag))
	}

	if len(l.terminal) > 0 && strings.Contains(l.terminal[0], "%%") {
		errs = append(errs, fmt.Errorf("first terminal argument cannot have a replaceable value"))
	}

	if (!strings.Contains(strings.Join(l.clusterLoginCommand, " "), "%%CLUSTER_ID%%")) && (!strings.Contains(strings.Join(l.terminal, " "), "%%CLUSTER_ID%%")) {
		errs = append(errs, fmt.Errorf("%s must contain %%CLUSTER_ID%%", clusterLoginCommandFlag))
	}

	if len(errs) > 0 {
		return fmt.Errorf("login error: %v", errs)
	}

	l.Enabled = true
	return nil
}

func (l *ClusterLauncher) BuildLoginCommand(vars map[string]string) []string {
	command := []string{}

	// When running in a Fedora Toolbox container, prepend flatpak-spawn --host
	// so the terminal emulator command executes on the host system rather than
	// inside the container where it does not exist.
	if l.runInToolbox {
		log.Debug("launcher.ClusterLauncher(): prepending flatpak-spawn --host for toolbox")
		command = append(command, "flatpak-spawn", "--host")
	}

	// Handle the Terminal command
	// The first arg should not be something replaceable, as checked in the
	// validate function
	log.Debug("launcher.BuildLoginCommand()", "terminal", l.terminal[0])
	command = append(command, l.terminal[0])

	// If there are more than one terminal arguments, replace the vars
	// If there's not more than one terminal argument, the "replacement"
	// nil []string{} ends up being appended as a whitespace, so don't append
	if len(l.terminal) > 1 {
		command = append(command, replaceVars(l.terminal[1:], vars)...)
	}
	command = append(command, replaceVars(l.clusterLoginCommand, vars)...)
	log.Debug("launcher.BuildLoginCommand()", "command", command)
	for x, i := range command {
		log.Debug("launcher.BuildLoginCommand()", "index", x, "arg", i)
	}

	return command
}

func replaceVars(args []string, vars map[string]string) []string {
	if args == nil || vars == nil {
		return []string{}
	}

	str := strings.Join(args, " ")

	for k, v := range vars {
		log.Debug("launcher.replaceVars()", "string", str, "key", k, "value", v)
		str = strings.ReplaceAll(str, k, v)
	}

	log.Debug("launcher.replaceVars()", "result", str)

	transformedArgs := strings.Split(str, " ")
	return transformedArgs
}
