package launcher

import (
	"fmt"
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

func (l *ClusterLauncher) BuildLoginCommand(cluster string) []string {
	command := []string{}

	// Handle the Terminal command
	// The first arg should not be something replaceable, as checked in the
	// validate function
	command = append(command, l.terminal[0])
	command = append(command, replaceVars(l.terminal[1:], cluster)...)
	command = append(command, replaceVars(l.clusterLoginCommand, cluster)...)

	return command
}

func replaceVars(args []string, cluster string) []string {
	transformedArgs := []string{}
	for _, str := range args {
		transformedArgs = append(transformedArgs, strings.Replace(str, "%%CLUSTER_ID%%", cluster, -1))
	}
	return transformedArgs
}
