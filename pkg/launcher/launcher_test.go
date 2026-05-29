package launcher

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClusterLauncher_ToolboxDetection(t *testing.T) {
	tests := []struct {
		name          string
		toolboxMode   string
		detectToolbox bool
		expectToolbox bool
	}{
		{
			name:          "auto mode with detection true",
			toolboxMode:   "auto",
			detectToolbox: true,
			expectToolbox: true,
		},
		{
			name:          "auto mode with detection false",
			toolboxMode:   "auto",
			detectToolbox: false,
			expectToolbox: false,
		},
		{
			name:          "forced true overrides detection",
			toolboxMode:   "true",
			detectToolbox: false,
			expectToolbox: true,
		},
		{
			name:          "forced false overrides detection",
			toolboxMode:   "false",
			detectToolbox: true,
			expectToolbox: false,
		},
		{
			name:          "empty string defaults to auto with detection true",
			toolboxMode:   "",
			detectToolbox: true,
			expectToolbox: true,
		},
		{
			name:          "empty string defaults to auto with detection false",
			toolboxMode:   "",
			detectToolbox: false,
			expectToolbox: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			launcher, err := NewClusterLauncherWithToolbox(
				"gnome-terminal --",
				"ocm backplane login %%CLUSTER_ID%%",
				test.toolboxMode,
				func() bool { return test.detectToolbox },
			)
			assert.NoError(t, err, "expected no error creating launcher")
			assert.Equal(t, test.expectToolbox, launcher.runInToolbox, "toolbox mode mismatch")
		})
	}
}

func TestBuildLoginCommand_ToolboxWrapping(t *testing.T) {
	launcher := ClusterLauncher{
		terminal:            []string{"gnome-terminal", "--"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        true,
	}

	vars := map[string]string{
		"%%CLUSTER_ID%%": "test-cluster-123",
	}

	cmd := launcher.BuildLoginCommand(vars)
	expected := []string{"flatpak-spawn", "--host", "gnome-terminal", "--", "ocm-container", "-C", "test-cluster-123"}

	assert.Equal(t, expected, cmd, "toolbox wrapping should prepend flatpak-spawn --host to entire command")
}

func TestBuildLoginCommand_NoWrappingNormal(t *testing.T) {
	launcher := ClusterLauncher{
		terminal:            []string{"gnome-terminal", "--"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        false,
	}

	vars := map[string]string{
		"%%CLUSTER_ID%%": "test-cluster-123",
	}

	cmd := launcher.BuildLoginCommand(vars)
	expected := []string{"gnome-terminal", "--", "ocm-container", "-C", "test-cluster-123"}

	assert.Equal(t, expected, cmd, "without toolbox mode, command should not be wrapped")
}

func TestBuildLoginCommand_ToolboxWrappingWithTmux(t *testing.T) {
	launcher := ClusterLauncher{
		terminal:            []string{"tmux", "new-window", "-n", "%%CLUSTER_ID%%"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        true,
	}

	vars := map[string]string{
		"%%CLUSTER_ID%%": "my-cluster",
	}

	cmd := launcher.BuildLoginCommand(vars)
	expected := []string{"flatpak-spawn", "--host", "tmux", "new-window", "-n", "my-cluster", "ocm-container", "-C", "my-cluster"}

	assert.Equal(t, expected, cmd, "toolbox wrapping should work with tmux terminal commands")
}

func TestClusterLauncherValidation(t *testing.T) {
	tests := []struct {
		name            string
		terminalArg     string
		loginCommandArg string
		expectErr       bool
	}{
		{
			name:            "Tests that the clusterLauncher can be created successfully",
			terminalArg:     "tmux new-window -n %%CLUSTER_NAME%%",
			loginCommandArg: "ocm-container -C %%CLUSTER_ID%%",
			expectErr:       false,
		},
		{
			name:            "Tests both terminal and login args are nil",
			terminalArg:     "",
			loginCommandArg: "",
			expectErr:       true,
		},
		{
			name:            "Tests terminal is nil but login args are not",
			terminalArg:     "",
			loginCommandArg: "ocm backplane session",
			expectErr:       true,
		},
		{
			name:            "Tests terminal is not nil but login args are nil",
			terminalArg:     "/bin/xterm",
			loginCommandArg: "",
			expectErr:       true,
		},
		{
			name:            "Tests that the first terminal argument cannot be a replaceable",
			terminalArg:     "%%CLUSTER_ID%% something",
			loginCommandArg: "ocm-container",
			expectErr:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClusterLauncher(test.terminalArg, test.loginCommandArg, "auto")
			if test.expectErr && err == nil {
				t.Fatalf("Expected error but none fired")
			}
			if !test.expectErr && err != nil {
				t.Fatalf("Expected no error but got %v", err)
			}
		})
	}
}

func TestLoginCommandBuild(t *testing.T) {
	tests := []struct {
		name         string
		launcher     ClusterLauncher
		expectErr    bool
		comparisonFN func(*testing.T, []string)
	}{
		{
			name: "Validate that the first argument won't be replaced",
			launcher: ClusterLauncher{
				terminal:            []string{"%%CLUSTER_ID%%", "%%CLUSTER_ID%%"},
				clusterLoginCommand: []string{"ocm", "backplane", "login", "%%CLUSTER_ID%%"},
			},
			expectErr: false,
			comparisonFN: func(t *testing.T, cmd []string) {
				expected := "%%CLUSTER_ID%% abcdefg ocm backplane login abcdefg"
				if strings.Join(cmd, " ") != expected {
					t.Fatalf("Expected command to be %s, got %s", expected, strings.Join(cmd, " "))
				}
			},
		},
		{
			name: "validate that cluster login command can be replaced",
			launcher: ClusterLauncher{
				terminal:            []string{"gnome-terminal"},
				clusterLoginCommand: []string{"ocm", "%%CLUSTER_ID%%", "backplane", "login", "-C", "%%CLUSTER_ID%%", "--manager"},
			},
			expectErr: false,
			comparisonFN: func(t *testing.T, cmd []string) {
				expected := "gnome-terminal ocm abcdefg backplane login -C abcdefg --manager"
				if strings.Join(cmd, " ") != expected {
					t.Fatalf("Expected command to be %s, got: %s", expected, strings.Join(cmd, " "))
				}
			},
		},
		{
			name: "validate multiple templated replacements",
			launcher: ClusterLauncher{
				terminal:            []string{"gnome-terminal", "--"},
				clusterLoginCommand: []string{"ocm-container", "--cluster-id", "%%CLUSTER_ID%%", "--launch-opts", "\"-e INCIDENT_ID=%%INCIDENT_ID%%\""},
			},
			expectErr: true,
			comparisonFN: func(t *testing.T, cmd []string) {
				expected := "gnome-terminal -- ocm-container --cluster-id abcdefg --launch-opts \"-e INCIDENT_ID=PD123456\""
				if strings.Join(cmd, " ") != expected {
					t.Fatalf("Expected command to be %s, got: %s", expected, strings.Join(cmd, " "))
				}
			},
		},
		{
			name: "validate that the cluster login command can be collapsed to a single string",
			launcher: ClusterLauncher{
				terminal:            []string{"tmux", "new-window", "-n", "%%CLUSTER_ID%%"},
				clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
			},
			expectErr: false,
			comparisonFN: func(t *testing.T, cmd []string) {
				expected := []string{"tmux", "new-window", "-n", "abcdefg", "ocm-container", "-C", "abcdefg"}
				if len(expected) != len(cmd) {
					t.Fatalf("Expected command slice %#v but got %#v", expected, cmd)
				}
				for k := range cmd {
					if cmd[k] != expected[k] {
						t.Fatalf("Expected command slice %#v but got %#v", expected, cmd)
					}
				}
			},
		},
		{
			name: "terminal or cluster login command must contain %%CLUSTER_ID%%",
			launcher: ClusterLauncher{
				terminal:            []string{"gnome-terminal", "--"},
				clusterLoginCommand: []string{"ocm", "backplane", "login"},
			},
			expectErr: true,
			comparisonFN: func(t *testing.T, cmd []string) {
				expected := []string{"gnome-terminal", "--", "ocm", "backplane", "login"} // Don't actually expect this - expect an error
				if len(expected) != len(cmd) {
					t.Fatalf("Expected command slice %#v but got %#v", expected, cmd)
				}
				for k := range cmd {
					if cmd[k] != expected[k] {
						t.Fatalf("Expected command slice %#v but got %#v", expected, cmd)
					}
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vars := map[string]string{
				"%%CLUSTER_ID%%":    "abcdefg",
				"%%INCIDENT_ID%%":   "PD123456",
				"%%UNHANDLED_VAR%%": "I SHOULD NOT SHOW UP",
			}
			cmd := test.launcher.BuildLoginCommand(vars)
			test.comparisonFN(t, cmd)
		})
	}
}
