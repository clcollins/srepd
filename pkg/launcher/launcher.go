package launcher

import (
	"fmt"
	"strings"
)

type ClusterLauncher struct {
	terminal            []string
	clusterLoginCommand []string
	settings            launcherSettings
}

type launcherSettings struct {
	collapseLoginCommand bool
}

func NewClusterLauncher(terminal []string, clusterLoginCommand []string) (ClusterLauncher, error) {
	launcher := ClusterLauncher{
		terminal:            terminal,
		clusterLoginCommand: clusterLoginCommand,
		settings:            launcherSettings{},
	}

	return launcher, nil
}

func (l *ClusterLauncher) SetCollapseLoginCommand(setting bool) {
	l.settings.collapseLoginCommand = setting
}

func (l *ClusterLauncher) Validate() error {
	errs := []error{}

	if l.terminal == nil {
		errs = append(errs, fmt.Errorf("terminal is not set"))
	}

	if l.clusterLoginCommand == nil {
		errs = append(errs, fmt.Errorf("clusterLoginCommand is not set"))
	}

	if len(l.terminal) > 0 && strings.Contains(l.terminal[0], "%%") {
		errs = append(errs, fmt.Errorf("first terminal argument cannot have a replaceable"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("login error: %v", errs)
	}
	return nil
}

func (l *ClusterLauncher) BuildLoginCommand(cluster string) ([]string, error) {
	command := []string{}

	// Handle the Terminal command
	// The first arg should not be something replaceable, as checked in the
	// validate function
	command = append(command, l.terminal[0])
	command = append(command, replaceVars(l.terminal[1:], cluster)...)

	loginCmd := replaceVars(l.clusterLoginCommand, cluster)
	if l.settings.collapseLoginCommand {
		command = append(command, strings.Join(loginCmd, " "))
	} else {
		command = append(command, loginCmd...)
	}

	return command, nil
}

func replaceVars(args []string, cluster string) []string {
	transformedArgs := []string{}
	for _, str := range args {
		transformedArgs = append(transformedArgs, strings.Replace(str, "%%CLUSTER_ID%%", cluster, -1))
	}
	return transformedArgs
}
