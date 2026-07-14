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

func TestIsToolbox(t *testing.T) {
	tests := []struct {
		name     string
		toolbox  bool
		expected bool
	}{
		{"toolbox mode on", true, true},
		{"toolbox mode off", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := ClusterLauncher{
				terminal:            []string{"gnome-terminal", "--"},
				clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
				runInToolbox:        tt.toolbox,
			}
			assert.Equal(t, tt.expected, l.IsToolbox())
		})
	}
}

func TestToolboxEnvFlags_AddsEnvFlags(t *testing.T) {
	l := ClusterLauncher{
		terminal:            []string{"gnome-terminal", "--"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        true,
	}
	envFlags := []string{"-e", "KEY1=val1", "-e", "KEY2=val2"}
	result := l.ToolboxEnvFlags(envFlags)
	expected := []string{"--env=KEY1=val1", "--env=KEY2=val2"}
	assert.Equal(t, expected, result)
}

func TestToolboxEnvFlags_NoFlagsWhenNotToolbox(t *testing.T) {
	l := ClusterLauncher{
		terminal:            []string{"gnome-terminal", "--"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        false,
	}
	envFlags := []string{"-e", "KEY1=val1", "-e", "KEY2=val2"}
	result := l.ToolboxEnvFlags(envFlags)
	assert.Nil(t, result)
}

func TestToolboxEnvFlags_EmptyEnvFlags(t *testing.T) {
	l := ClusterLauncher{
		terminal:            []string{"gnome-terminal", "--"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        true,
	}
	result := l.ToolboxEnvFlags([]string{})
	assert.Nil(t, result)
}

func TestToolboxEnvFlags_NilEnvFlags(t *testing.T) {
	l := ClusterLauncher{
		terminal:            []string{"gnome-terminal", "--"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        true,
	}
	result := l.ToolboxEnvFlags(nil)
	assert.Nil(t, result)
}

func TestToolboxEnvFlags_OddLengthEnvFlags(t *testing.T) {
	l := ClusterLauncher{
		terminal:            []string{"gnome-terminal", "--"},
		clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
		runInToolbox:        true,
	}
	// Odd-length slice: trailing "-e" without a value should be ignored
	envFlags := []string{"-e", "KEY1=val1", "-e"}
	result := l.ToolboxEnvFlags(envFlags)
	expected := []string{"--env=KEY1=val1"}
	assert.Equal(t, expected, result)
}

func TestInsertToolboxEnvFlags_PositionAfterFlatpakSpawn(t *testing.T) {
	command := []string{"flatpak-spawn", "--host", "gnome-terminal", "--", "ocm", "backplane", "login", "abc123"}
	toolboxFlags := []string{"--env=KEY1=val1", "--env=KEY2=val2"}
	result := InsertToolboxEnvFlags(command, toolboxFlags)
	expected := []string{"flatpak-spawn", "--host", "--env=KEY1=val1", "--env=KEY2=val2", "gnome-terminal", "--", "ocm", "backplane", "login", "abc123"}
	assert.Equal(t, expected, result)
}

func TestInsertToolboxEnvFlags_EmptyFlags(t *testing.T) {
	command := []string{"flatpak-spawn", "--host", "gnome-terminal", "--", "ocm", "backplane", "login"}
	result := InsertToolboxEnvFlags(command, []string{})
	assert.Equal(t, command, result)
}

func TestInsertToolboxEnvFlags_NoFlatpakSpawn(t *testing.T) {
	// No --host in command; should return command unchanged
	command := []string{"gnome-terminal", "--", "ocm-container", "-C", "abc123"}
	toolboxFlags := []string{"--env=KEY=val"}
	result := InsertToolboxEnvFlags(command, toolboxFlags)
	assert.Equal(t, command, result)
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

func TestProfile_ReturnsDetectedProfile(t *testing.T) {
	tests := []struct {
		name            string
		terminal        string
		loginCommand    string
		expectedProfile string
	}{
		{
			name:            "gnome-terminal detected as SeparatorProfile",
			terminal:        "gnome-terminal",
			loginCommand:    "ocm backplane login %%CLUSTER_ID%%",
			expectedProfile: "separator (gnome-terminal)",
		},
		{
			name:            "tmux detected as SeparatorProfile",
			terminal:        "tmux new-window -n %%CLUSTER_ID%%",
			loginCommand:    "ocm-container -C %%CLUSTER_ID%%",
			expectedProfile: "separator (tmux)",
		},
		{
			name:            "konsole detected as FlagProfile",
			terminal:        "konsole",
			loginCommand:    "ocm backplane login %%CLUSTER_ID%%",
			expectedProfile: "flag[-e] (konsole)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			launcher, err := NewClusterLauncherWithToolbox(
				tt.terminal,
				tt.loginCommand,
				"false",
				func() bool { return false },
			)
			assert.NoError(t, err)
			assert.NotNil(t, launcher.Profile(), "Profile() should not return nil")
			assert.Equal(t, tt.expectedProfile, launcher.Profile().Name())
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

// TestReplaceVars_PreservesArgBoundaries verifies that a substituted value
// containing spaces stays within its single argv element and does not get
// re-tokenized into extra arguments. This is the core of the argument-injection
// hardening: an attacker-controlled %%CLUSTER_ID%% like "x --evil-flag y" must
// not inject extra arguments into the launched command.
func TestReplaceVars_PreservesArgBoundaries(t *testing.T) {
	t.Run("value with spaces stays a single argv element", func(t *testing.T) {
		args := []string{"login", "-c", "%%CLUSTER_ID%%"}
		vars := map[string]string{"%%CLUSTER_ID%%": "evil --flag injected"}

		got := replaceVars(args, vars)

		assert.Equal(t, []string{"login", "-c", "evil --flag injected"}, got,
			"substituted value must occupy exactly one argv slot")
		assert.Len(t, got, 3, "must not re-tokenize the substituted value into extra args")
	})

	t.Run("simple substitution without spaces is unchanged", func(t *testing.T) {
		args := []string{"login", "-c", "%%CLUSTER_ID%%", "--incident", "%%INCIDENT_ID%%"}
		vars := map[string]string{"%%CLUSTER_ID%%": "abcdefg", "%%INCIDENT_ID%%": "PD123456"}

		got := replaceVars(args, vars)

		assert.Equal(t, []string{"login", "-c", "abcdefg", "--incident", "PD123456"}, got)
	})

	t.Run("nil args or vars returns empty slice", func(t *testing.T) {
		assert.Equal(t, []string{}, replaceVars(nil, map[string]string{"a": "b"}))
		assert.Equal(t, []string{}, replaceVars([]string{"a"}, nil))
	})

	t.Run("multiple vars in one arg all substitute in place", func(t *testing.T) {
		args := []string{"prefix-%%CLUSTER_ID%%-%%INCIDENT_ID%%-suffix"}
		vars := map[string]string{"%%CLUSTER_ID%%": "CID", "%%INCIDENT_ID%%": "IID"}

		got := replaceVars(args, vars)

		assert.Equal(t, []string{"prefix-CID-IID-suffix"}, got)
	})
}

// TestBuildRosaBoundaryCommand verifies that rosa-boundary commands are built
// without terminal wrapping, since rosa-boundary manages its own interactive
// session via session-manager-plugin.
func TestBuildRosaBoundaryCommand_DirectExecution(t *testing.T) {
	t.Run("basic rosa-boundary command without terminal wrapper", func(t *testing.T) {
		launcher := ClusterLauncher{
			terminal:            []string{"gnome-terminal", "--"},
			clusterLoginCommand: []string{"rosa-boundary", "start-task", "--cluster-id", "%%CLUSTER_ID%%", "--connect"},
			runInToolbox:        false,
		}

		vars := map[string]string{
			"%%CLUSTER_ID%%": "test-cluster-123",
		}

		cmd := launcher.BuildRosaBoundaryCommand(vars)
		expected := []string{"rosa-boundary", "start-task", "--cluster-id", "test-cluster-123", "--connect"}

		assert.Equal(t, expected, cmd, "rosa-boundary command should execute directly without terminal wrapper")
	})

	t.Run("rosa-boundary with toolbox mode adds flatpak-spawn like BuildLoginCommand", func(t *testing.T) {
		// rosa-boundary is a peer of cluster_login_command (and its eventual
		// replacement), so it follows the same toolbox convention: srepd in
		// a toolbox runs the command on the host via flatpak-spawn --host.
		launcher := ClusterLauncher{
			terminal:            []string{"gnome-terminal", "--"},
			clusterLoginCommand: []string{"rosa-boundary", "start-task", "--cluster-id", "%%CLUSTER_ID%%", "--connect"},
			runInToolbox:        true,
		}

		vars := map[string]string{
			"%%CLUSTER_ID%%": "test-cluster-456",
		}

		cmd := launcher.BuildRosaBoundaryCommand(vars)
		expected := []string{"flatpak-spawn", "--host", "rosa-boundary", "start-task", "--cluster-id", "test-cluster-456", "--connect"}

		assert.Equal(t, expected, cmd, "toolbox mode must wrap rosa-boundary with flatpak-spawn --host, same as the cluster login path")
	})

	t.Run("rosa-boundary with multiple variable replacements", func(t *testing.T) {
		launcher := ClusterLauncher{
			terminal:            []string{"gnome-terminal", "--"},
			clusterLoginCommand: []string{"rosa-boundary", "start-task", "--cluster-id", "%%CLUSTER_ID%%", "--incident", "%%INCIDENT_ID%%", "--connect"},
			runInToolbox:        false,
		}

		vars := map[string]string{
			"%%CLUSTER_ID%%":  "my-cluster",
			"%%INCIDENT_ID%%": "PD789",
		}

		cmd := launcher.BuildRosaBoundaryCommand(vars)
		expected := []string{"rosa-boundary", "start-task", "--cluster-id", "my-cluster", "--incident", "PD789", "--connect"}

		assert.Equal(t, expected, cmd, "rosa-boundary command should replace all variables correctly")
	})

	t.Run("comparison: BuildLoginCommand adds terminal wrapper", func(t *testing.T) {
		launcher := ClusterLauncher{
			terminal:            []string{"gnome-terminal", "--"},
			clusterLoginCommand: []string{"rosa-boundary", "start-task", "--cluster-id", "%%CLUSTER_ID%%", "--connect"},
			runInToolbox:        false,
			profile:             &GenericProfile{},
		}

		vars := map[string]string{
			"%%CLUSTER_ID%%": "test-cluster-789",
		}

		loginCmd := launcher.BuildLoginCommand(vars)
		rbCmd := launcher.BuildRosaBoundaryCommand(vars)

		// BuildLoginCommand wraps with terminal
		assert.Contains(t, loginCmd, "gnome-terminal", "BuildLoginCommand should include terminal wrapper")
		assert.Contains(t, loginCmd, "--", "BuildLoginCommand should include terminal separator")

		// BuildRosaBoundaryCommand does not
		assert.NotContains(t, rbCmd, "gnome-terminal", "BuildRosaBoundaryCommand should not include terminal wrapper")
		assert.Equal(t, "rosa-boundary", rbCmd[0], "BuildRosaBoundaryCommand should start with rosa-boundary command")
	})
}
