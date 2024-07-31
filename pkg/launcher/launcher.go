package launcher

import (
	"fmt"
	"github.com/charmbracelet/log"
	"strings"
)

type ClusterLauncher struct {
	Enabled             bool
	terminal            []string
	clusterLoginCommand []string
	settings            launcherSettings
}

type launcherSettings struct {
	// Future Usage Possibly
}

func NewClusterLauncher(terminal string, clusterLoginCommand string) (ClusterLauncher, error) {

	launcher := ClusterLauncher{
		terminal:            strings.Split(terminal, " "),
		clusterLoginCommand: strings.Split(clusterLoginCommand, " "),
		settings:            launcherSettings{},
	}

	err := launcher.validate()
	if err != nil {
		return ClusterLauncher{}, err
	}

	return launcher, nil
}

func (l *ClusterLauncher) validate() error {
	errs := []error{}

	if l.terminal == nil || l.terminal[0] == "" {
		errs = append(errs, fmt.Errorf("terminal is not set"))
	}

	if l.clusterLoginCommand == nil || l.clusterLoginCommand[0] == "" {
		errs = append(errs, fmt.Errorf("clusterLoginCommand is not set"))
	}

	if len(l.terminal) > 0 && strings.Contains(l.terminal[0], "%%") {
		errs = append(errs, fmt.Errorf("first terminal argument cannot have a replaceable"))
	}

	if (!strings.Contains(strings.Join(l.clusterLoginCommand, " "), "%%CLUSTER_ID%%")) && (!strings.Contains(strings.Join(l.terminal, " "), "%%CLUSTER_ID%%")) {
		errs = append(errs, fmt.Errorf("clusterLoginCommand must contain %%CLUSTER_ID%%"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("login error: %v", errs)
	}

	l.Enabled = true
	return nil
}

func (l *ClusterLauncher) BuildLoginCommand(vars map[string]string) []string {
	///func (l *ClusterLauncher) BuildLoginCommand() []string {
	command := []string{}

	// Handle the Terminal command
	// The first arg should not be something replaceable, as checked in the
	// validate function
	log.Debug("launcher.ClusterLauncher():", "Building command from terminal", "terminal", l.terminal[0])
	command = append(command, l.terminal[0])

	// If there are more than one terminal arguments, replace the vars
	// If there's not more than one terminal argument, the "replacement"
	// nil []string{} ends up being appended as a whitespace, so don't append
	if len(l.terminal) > 1 {
		command = append(command, replaceVars(l.terminal[1:], vars)...)
	}
	command = append(command, replaceVars(l.clusterLoginCommand, vars)...)
	log.Debug("launcher.ClusterLauncher():", "Built command", command)
	for x, i := range command {
		log.Debug("launcher.ClusterLauncher():", fmt.Sprintf("Built command argument [%d]", x), i)
	}

	return command
}

func replaceVars(args []string, vars map[string]string) []string {
	if args == nil || vars == nil {
		return []string{}
	}

	str := strings.Join(args, " ")

	for k, v := range vars {
		log.Debug("ClusterLauncher():", "Replacing vars in string", str, k, v)
		str = strings.Replace(str, k, v, -1)
	}

	transformedArgs := strings.Split(str, " ")
	return transformedArgs
}
