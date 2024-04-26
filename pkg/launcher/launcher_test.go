package launcher

import (
	"strings"
	"testing"
)

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
			name:            "Tests terminal is nill but login args are not",
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
			_, err := NewClusterLauncher(test.terminalArg, test.loginCommandArg)
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
			cmd := test.launcher.BuildLoginCommand("abcdefg")
			test.comparisonFN(t, cmd)
		})
	}
}
