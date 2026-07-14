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
	profile             TerminalProfile
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
		runInToolbox:        inToolbox,
		profile:             DetectTerminalProfile(terminal),
		settings:            launcherSettings{},
	}

	if inToolbox {
		log.Debug("Toolbox detected: terminal commands will be prefixed with flatpak-spawn --host")
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

// IsToolbox returns whether this launcher is configured to run inside a
// Fedora Toolbox container, meaning commands are prefixed with flatpak-spawn.
func (l *ClusterLauncher) IsToolbox() bool {
	return l.runInToolbox
}

// ToolboxEnvFlags converts a slice of "-e", "VAR=VALUE" pairs (the format
// produced by buildPagerDutyEnvVars) into "--env=VAR=VALUE" flags suitable
// for flatpak-spawn. Returns nil if the launcher is not in toolbox mode,
// or if envFlags is nil/empty.
func (l *ClusterLauncher) ToolboxEnvFlags(envFlags []string) []string {
	if !l.runInToolbox {
		return nil
	}
	if len(envFlags) == 0 {
		return nil
	}

	var toolboxFlags []string
	for i := 0; i < len(envFlags)-1; i += 2 {
		if envFlags[i] == "-e" {
			toolboxFlags = append(toolboxFlags, fmt.Sprintf("--env=%s", envFlags[i+1]))
		}
	}
	return toolboxFlags
}

// InsertToolboxEnvFlags inserts --env= flags into a command slice at the
// correct position for flatpak-spawn: after "flatpak-spawn" and "--host"
// but before the terminal command. Returns the original command if no
// flatpak-spawn is found or toolboxFlags is empty.
func InsertToolboxEnvFlags(command []string, toolboxFlags []string) []string {
	if len(toolboxFlags) == 0 {
		return command
	}

	// Find the insertion point: right after "--host" in the flatpak-spawn prefix
	insertIdx := -1
	for i, arg := range command {
		if arg == "--host" {
			insertIdx = i + 1
			break
		}
	}

	if insertIdx < 0 || insertIdx > len(command) {
		return command
	}

	result := make([]string, 0, len(command)+len(toolboxFlags))
	result = append(result, command[:insertIdx]...)
	result = append(result, toolboxFlags...)
	result = append(result, command[insertIdx:]...)
	return result
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

	var command []string

	// If the terminal args already contain the separator or flag that
	// the profile would insert, fall back to simple concatenation to
	// preserve backward compatibility for users who manually specified
	// the separator in their config.
	if alreadyHasSeparator(profile, terminalArgs) {
		command = append(terminalArgs, loginCmd...)
	} else {
		// Use the profile to build the command with the correct separator.
		var err error
		command, err = profile.BuildCommand(terminalArgs, loginCmd)
		if err != nil {
			// Fallback to simple concatenation on error.
			log.Debug("launcher.ClusterLauncher(): profile.BuildCommand failed, falling back", "error", err)
			command = append(terminalArgs, loginCmd...)
		}
	}

	// When running in a Fedora Toolbox container, prepend flatpak-spawn --host
	// so the terminal emulator command executes on the host system rather than
	// inside the container where it does not exist.
	if l.runInToolbox {
		log.Debug("launcher.ClusterLauncher(): prepending flatpak-spawn --host for toolbox")
		command = append([]string{"flatpak-spawn", "--host"}, command...)
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
		log.Debug("launcher.BuildLoginCommand()", "index", x, "arg", i)
	}
}

// replaceVars substitutes template variables (e.g. %%CLUSTER_ID%%) in each arg,
// operating per-arg so a substituted value never crosses argv boundaries. The
// previous implementation joined all args on spaces, substituted, then split back
// on spaces — which re-tokenized any substituted value containing a space into
// extra argv elements. Because these args are passed to exec.Command (no shell),
// that allowed attacker-controlled alert data (cluster/incident IDs) to inject
// additional command-line arguments. Per-arg substitution closes that.
func replaceVars(args []string, vars map[string]string) []string {
	if args == nil || vars == nil {
		return []string{}
	}

	out := make([]string, len(args))
	for i, arg := range args {
		for k, v := range vars {
			arg = strings.ReplaceAll(arg, k, v)
		}
		out[i] = arg
	}

	log.Debug("launcher.replaceVars()", "result", out)
	return out
}
