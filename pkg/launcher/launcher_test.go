package launcher

import (
	"strings"
	"testing"
)

func TestClusterLauncherValidation(t *testing.T) {
	tests := []struct {
		name            string
		terminalArg     []string
		loginCommandArg []string
		expectErr       bool
	}{
		{
			name:            "Tests that the clusterLauncher can be created successfully",
			terminalArg:     []string{"tmux", "new-window", "-n", "%%CLUSTER_NAME%%"},
			loginCommandArg: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
			expectErr:       false,
		},
		{
			name:            "Tests both terminal and login args are nil",
			terminalArg:     nil,
			loginCommandArg: nil,
			expectErr:       true,
		},
		{
			name:            "Tests terminal is nill but login args are not",
			terminalArg:     nil,
			loginCommandArg: []string{"ocm", "backplane", "session"},
			expectErr:       true,
		},
		{
			name:            "Tests terminal is not nil but login args are nil",
			terminalArg:     []string{"/bin/xterm"},
			loginCommandArg: nil,
			expectErr:       true,
		},
		{
			name:            "Tests that the first terminal argument cannot be a replaceable",
			terminalArg:     []string{"%%CLUSTER_ID%%", "something"},
			loginCommandArg: []string{"ocm-container"},
			expectErr:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			launcher, err := NewClusterLauncher(test.terminalArg, test.loginCommandArg)
			if err != nil {
				t.Fatalf("Unexpected Error initializing Launcher")
			}

			err = launcher.Validate()
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
				clusterLoginCommand: []string{"ocm", "backplane", "login"},
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
			name: "validate that the cluster login command can be collapsed to a single string",
			launcher: ClusterLauncher{
				terminal:            []string{"tmux", "new-window", "-n", "%%CLUSTER_ID%%"},
				clusterLoginCommand: []string{"ocm-container", "-C", "%%CLUSTER_ID%%"},
				settings:            launcherSettings{collapseLoginCommand: true},
			},
			expectErr: false,
			comparisonFN: func(t *testing.T, cmd []string) {
				expected := []string{"tmux", "new-window", "-n", "abcdefg", "ocm-container -C abcdefg"}
				if len(expected) != len(cmd) {
					t.Fatalf("Expected command slice %#v but got %#v", expected, cmd)
				}
				for k, _ := range cmd {
					if cmd[k] != expected[k] {
						t.Fatalf("Expected command slice %#v but got %#v", expected, cmd)
					}
				}
			},
		},
		{
			name: "cluster login command when it has no replaceable values should have the cluster ID appended to the end",
			launcher: ClusterLauncher{
				terminal:            []string{"gnome-terminal"},
				clusterLoginCommand: []string{"ocm-container"},
			},
			expectErr: false,
			comparisonFN: func(t *testing.T, cmd []string) {
				expected := "gnome-terminal ocm-container abcdefg"
				if expected != strings.Join(cmd, " ") {
					t.Fatalf("Expected cluster ID to be automaticall appended. Got: %s, expected: %s", cmd, expected)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd, err := test.launcher.BuildLoginCommand("abcdefg")
			if test.expectErr && err == nil {
				t.Fatal("Expected an error and got none")
			}
			if !test.expectErr && err != nil {
				t.Fatalf("Was not expecting an error but got: %v", err)
			}

			test.comparisonFN(t, cmd)
		})
	}
}
