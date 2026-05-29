package launcher

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
)

const clusterLoginCommandFlag = "cluster_login_command"
const terminalFlag = "terminal"

type ClusterLauncher struct {
	Enabled             bool
	terminal            []string
	clusterLoginCommand []string
	profile             TerminalProfile
	settings            launcherSettings
}

type launcherSettings struct {
	// Future Usage Possibly
}

func NewClusterLauncher(terminal string, clusterLoginCommand string) (ClusterLauncher, error) {

	// Feature 2: Warn on redundant "flatpak run" prefix when the user
	// could simplify to just the app ID.
	if appID, redundant := detectRedundantFlatpakPrefix(terminal); redundant {
		log.Warn("launcher: terminal config includes 'flatpak run' prefix; you can simplify to just the app ID", "terminal", terminal, "suggestion", appID)
	}

	// Feature 1: If the terminal string is a bare Flatpak app ID
	// (e.g., "org.kde.konsole"), prepend "flatpak run" so the command
	// actually launches the Flatpak application.
	terminalForExec := terminal
	parts := strings.Fields(terminal)
	if len(parts) > 0 && isFlatpakAppID(parts[0]) {
		terminalForExec = "flatpak run " + terminal
		log.Debug("launcher: detected Flatpak app ID, prepending 'flatpak run'", "original", terminal, "resolved", terminalForExec)
	}

	// Feature 3: Validate that the terminal command exists on the system.
	if warning := validateTerminalExists(terminal); warning != "" {
		log.Warn("launcher: terminal command not found in PATH; cluster login may fail", "terminal", terminal, "detail", warning)
	}

	launcher := ClusterLauncher{
		terminal:            strings.Split(terminalForExec, " "),
		clusterLoginCommand: strings.Split(clusterLoginCommand, " "),
		profile:             DetectTerminalProfile(terminal),
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

// Profile returns the detected TerminalProfile for this launcher.
func (l *ClusterLauncher) Profile() TerminalProfile {
	return l.profile
}

func (l *ClusterLauncher) BuildLoginCommand(vars map[string]string) []string {
	// Ensure a profile is set; default to GenericProfile if nil (e.g.,
	// when ClusterLauncher was constructed directly without NewClusterLauncher).
	profile := l.profile
	if profile == nil {
		profile = &GenericProfile{}
	}

	log.Debug("launcher.ClusterLauncher(): building command", "terminal", l.terminal[0], "profile", profile.Name())

	// Replace variables in terminal args (skip the first arg, which is
	// the executable and must not be a replaceable value).
	terminalArgs := []string{l.terminal[0]}
	if len(l.terminal) > 1 {
		terminalArgs = append(terminalArgs, replaceVars(l.terminal[1:], vars)...)
	}

	loginCmd := replaceVars(l.clusterLoginCommand, vars)

	// If the terminal args already contain the separator or flag that
	// the profile would insert, fall back to simple concatenation to
	// preserve backward compatibility for users who manually specified
	// the separator in their config.
	if alreadyHasSeparator(profile, terminalArgs) {
		command := append(terminalArgs, loginCmd...)
		l.logCommand(command)
		return command
	}

	// Use the profile to build the command with the correct separator.
	command, err := profile.BuildCommand(terminalArgs, loginCmd)
	if err != nil {
		// Fallback to simple concatenation on error.
		log.Debug("launcher.ClusterLauncher(): profile.BuildCommand failed, falling back", "error", err)
		command = append(terminalArgs, loginCmd...)
	}

	l.logCommand(command)
	return command
}

// alreadyHasSeparator checks whether the terminal args already include
// the separator or flag that the detected profile would inject. This
// preserves backward compatibility for configs where the user manually
// placed "--" or "-e" in the terminal string.
func alreadyHasSeparator(profile TerminalProfile, terminalArgs []string) bool {
	switch p := profile.(type) {
	case *SeparatorProfile:
		for _, arg := range terminalArgs[1:] {
			if arg == "--" {
				return true
			}
		}
	case *FlagProfile:
		for _, arg := range terminalArgs[1:] {
			if arg == p.flag {
				return true
			}
		}
	}
	return false
}

func (l *ClusterLauncher) logCommand(command []string) {
	log.Debug("launcher.ClusterLauncher(): built command", "command", command)
	for x, i := range command {
		log.Debug("launcher.ClusterLauncher(): build command argument", fmt.Sprintf("[%d]", x), i)
	}
}

func replaceVars(args []string, vars map[string]string) []string {
	if args == nil || vars == nil {
		return []string{}
	}

	str := strings.Join(args, " ")

	for k, v := range vars {
		log.Debug("ClusterLauncher():", "Replacing vars in string", str, k, v)
		str = strings.ReplaceAll(str, k, v)
	}

	log.Debug("launcher.replaceVars(): Replaced vars in string", "string", str)

	transformedArgs := strings.Split(str, " ")
	return transformedArgs
}
