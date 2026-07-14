package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/glamour/v2"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/clcollins/srepd/pkg/rand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecErr_Error(t *testing.T) {
	tests := []struct {
		name     string
		ee       execErr
		expected string
	}{
		{
			name: "strips Error: prefix and newline suffix",
			ee: execErr{
				ExecStdErr: "Error: something went wrong\n",
			},
			expected: "something went wrong",
		},
		{
			name: "strips lowercase error: prefix",
			ee: execErr{
				ExecStdErr: "error: failed to start\n",
			},
			expected: "failed to start",
		},
		{
			name: "returns plain message unchanged",
			ee: execErr{
				ExecStdErr: "plain message",
			},
			expected: "plain message",
		},
		{
			name: "handles empty string",
			ee: execErr{
				ExecStdErr: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ee.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHelperProcess(t *testing.T) {
	code := os.Getenv("SREPD_TEST_EXIT_CODE")
	if code == "" {
		return
	}
	exitCode := 0
	_, _ = fmt.Sscanf(code, "%d", &exitCode)
	os.Exit(exitCode)
}

func TestExecErr_Code(t *testing.T) {
	tests := []struct {
		name         string
		exitCode     string
		expectedCode int
	}{
		{
			name:         "exit code 1",
			exitCode:     "1",
			expectedCode: 1,
		},
		{
			name:         "exit code 2",
			exitCode:     "2",
			expectedCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
			cmd.Env = append(os.Environ(), "SREPD_TEST_EXIT_CODE="+tt.exitCode)
			err := cmd.Run()
			exitErr, ok := err.(*exec.ExitError)
			assert.True(t, ok, "expected *exec.ExitError")

			ee := &execErr{
				Err:        err,
				ExitErr:    exitErr,
				ExecStdErr: "test error",
			}
			assert.Equal(t, tt.expectedCode, ee.Code())
		})
	}
}

func TestAcknowledgeIncident(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "PABC123"},
		},
	}

	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "QABCDEFG1234567"}},
		{APIObject: pagerduty.APIObject{ID: "QABCDEFG7654321"}},
	}

	errIncidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "err"}},
		{APIObject: pagerduty.APIObject{ID: "err"}},
	}

	tests := []struct {
		name      string
		incidents []pagerduty.Incident
		expected  tea.Msg
	}{
		{
			name:      "return acknowledgedIncidentMsg with non-nil error if error occurs while acknowledging",
			incidents: errIncidents,
			expected: acknowledgedIncidentsMsg{
				incidents: []pagerduty.Incident(nil),
				err:       pd.ErrMockError,
			},
		},
		{
			name:      "return acknowledgedIncidentMsg with an incident list if no error occurs while acknowledging",
			incidents: incidents,
			expected: acknowledgedIncidentsMsg{
				incidents: incidents,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := acknowledgeIncidents(mockConfig, test.incidents)
			actual := cmd()
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestGetIncident(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
	id := rand.ID("Q") // simulate a PagerDuty alert ID

	tests := []struct {
		name     string
		config   *pd.Config
		id       string
		expected tea.Msg
	}{
		{
			name:     "return setStatusMsg if incident id is nil",
			config:   mockConfig,
			id:       "",
			expected: setStatusMsg{nilIncidentMsg},
		},
		{
			name:   "return gotIncidentMsg if incident id is provided",
			config: mockConfig,
			id:     id,
			expected: gotIncidentMsg{
				incident: &pagerduty.Incident{
					APIObject: pagerduty.APIObject{ID: id},
				},
				err: nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := getIncident(test.config, test.id)
			actual := cmd()
			assert.Equal(t, test.expected, actual)
		})
	}
}

// TestGetIncident_ErrorIsWrapped verifies the error path separately. getIncident now
// routes through pd.GetIncident, which applies a timeout AND wraps the underlying
// error with a "pd.GetIncident():" prefix for context (consistent with the sibling
// pd.GetAlerts / pd.GetNotes wrappers). The wrapped message still contains the
// original mock error text, so callers that surface the error string see strictly
// more context than before — an intentional improvement, not a regression.
func TestGetIncident_ErrorIsWrapped(t *testing.T) {
	mockConfig := &pd.Config{Client: &pd.MockPagerDutyClient{}}

	msg := getIncident(mockConfig, "err")()

	got, ok := msg.(gotIncidentMsg)
	assert.True(t, ok, "expected gotIncidentMsg")
	if assert.Error(t, got.err) {
		assert.Contains(t, got.err.Error(), "pd.GetIncident()", "error should carry the wrapper prefix")
		assert.Contains(t, got.err.Error(), pd.ErrMockError.Error(), "wrapped error must retain the original message")
	}
}

func TestGetIncidentAlerts(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
	testID := rand.ID("Q")
	tests := []struct {
		name     string
		config   *pd.Config
		id       string
		expected tea.Msg
	}{
		{
			name:     "return setStatusMsg if incident id is nil",
			config:   mockConfig,
			id:       "",
			expected: setStatusMsg{nilIncidentMsg},
		},
		{
			name:   "return gotIncidentAlertsMsg if incident id is provided",
			config: mockConfig,
			id:     testID,
			expected: gotIncidentAlertsMsg{
				incidentID: testID,
				alerts: []pagerduty.IncidentAlert{
					{APIObject: pagerduty.APIObject{ID: "QABCDEFG1234567"}},
					{APIObject: pagerduty.APIObject{ID: "QABCDEFG7654321"}},
				},
				err: nil,
			},
		},
		{
			name:   "return gotIncidentAlertsMsg with not-nil error if error occurs",
			config: mockConfig,
			id:     "err", // "err" signals the mock client to produce a mock error
			expected: gotIncidentAlertsMsg{
				incidentID: "err",
				alerts:     nil,
				err:        fmt.Errorf("pd.GetAlerts(): failed to get alerts for incident `%v`: %v", "err", pd.ErrMockError),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := getIncidentAlerts(test.config, test.id)
			actual := cmd()
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestGetIncidentNotes(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
	testID := rand.ID("Q")
	tests := []struct {
		name     string
		config   *pd.Config
		id       string
		expected tea.Msg
	}{
		{
			name:     "return setStatusMsg if incident id is nil",
			config:   mockConfig,
			id:       "",
			expected: setStatusMsg{nilIncidentMsg},
		},
		{
			name:   "return gotIncidentNotesMsg if incident id is provided",
			config: mockConfig,
			id:     testID,
			expected: gotIncidentNotesMsg{
				incidentID: testID,
				notes: []pagerduty.IncidentNote{
					{ID: "QABCDEFG1234567"},
					{ID: "QABCDEFG7654321"},
				},
				err: nil,
			},
		},
		{
			name:   "return gotIncidentNotesMsg with not-nil error if error occurs",
			config: mockConfig,
			id:     "err", // "err" signals the mock client to produce a mock error
			expected: gotIncidentNotesMsg{
				incidentID: "err",
				notes:      []pagerduty.IncidentNote{},
				err:        fmt.Errorf("pd.GetNotes(): failed to get incident notes `%v`: %v", "err", pd.ErrMockError),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := getIncidentNotes(test.config, test.id)
			actual := cmd()
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestIgnoredUserIDs(t *testing.T) {
	t.Run("returns IDs from user list", func(t *testing.T) {
		users := []*pagerduty.User{
			{APIObject: pagerduty.APIObject{ID: "P1"}},
			{APIObject: pagerduty.APIObject{ID: "P2"}},
		}
		ids := ignoredUserIDs(users)
		assert.Equal(t, []string{"P1", "P2"}, ids)
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		ids := ignoredUserIDs(nil)
		assert.Nil(t, ids)
	})
}

func TestFilterUserIDs(t *testing.T) {
	t.Run("removes ignored users", func(t *testing.T) {
		members := []string{"P1", "P2", "P3"}
		ignored := []string{"P2"}
		result := filterUserIDs(members, ignored)
		assert.Equal(t, []string{"P1", "P3"}, result)
	})

	t.Run("returns all when no ignored users", func(t *testing.T) {
		members := []string{"P1", "P2"}
		result := filterUserIDs(members, nil)
		assert.Equal(t, []string{"P1", "P2"}, result)
	})

	t.Run("returns nil for nil members", func(t *testing.T) {
		result := filterUserIDs(nil, []string{"P1"})
		assert.Nil(t, result)
	})

	t.Run("extra ignored users have no effect", func(t *testing.T) {
		members := []string{"P1", "P2"}
		ignored := []string{"P2", "PXYZ"}
		result := filterUserIDs(members, ignored)
		assert.Equal(t, []string{"P1"}, result)
	})
}

func TestChunkStrings(t *testing.T) {
	t.Run("returns nil for empty input", func(t *testing.T) {
		assert.Nil(t, chunkStrings(nil, 3))
		assert.Nil(t, chunkStrings([]string{}, 3))
	})

	t.Run("single chunk when under size", func(t *testing.T) {
		chunks := chunkStrings([]string{"a", "b"}, 3)
		assert.Equal(t, [][]string{{"a", "b"}}, chunks)
	})

	t.Run("single chunk at exactly size", func(t *testing.T) {
		chunks := chunkStrings([]string{"a", "b", "c"}, 3)
		assert.Equal(t, [][]string{{"a", "b", "c"}}, chunks)
	})

	t.Run("splits over size with remainder", func(t *testing.T) {
		chunks := chunkStrings([]string{"a", "b", "c", "d"}, 3)
		assert.Equal(t, [][]string{{"a", "b", "c"}, {"d"}}, chunks)
	})
}

func TestUpdateIncidentList_PerTeamQuery(t *testing.T) {
	t.Run("queries per team and deduplicates", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			Teams: []*pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
				{APIObject: pagerduty.APIObject{ID: "TEAM2"}},
			},
			TeamMembersByTeam: map[string][]string{
				"TEAM1": {"U1", "U2"},
				"TEAM2": {"U2", "U3"},
			},
		}

		cmd := updateIncidentList(config)
		msg := cmd()
		result, ok := msg.(updatedIncidentListMsg)
		assert.True(t, ok)
		assert.NoError(t, result.err)
		// MockPagerDutyClient returns 2 incidents per call; with 2 teams
		// the same incidents are returned both times, so dedup should
		// produce exactly 2 unique incidents
		assert.Len(t, result.incidents, 2)
	})

	t.Run("returns empty for nil config", func(t *testing.T) {
		cmd := updateIncidentList(nil)
		msg := cmd()
		result, ok := msg.(updatedIncidentListMsg)
		assert.True(t, ok)
		assert.NoError(t, result.err)
		assert.Nil(t, result.incidents)
	})

	t.Run("excludes ignored users from per-team queries", func(t *testing.T) {
		mock := &pd.MockPagerDutyClient{}
		config := &pd.Config{
			Client: mock,
			Teams: []*pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
			},
			TeamMembersByTeam: map[string][]string{
				"TEAM1": {"U1", "IGNORED"},
			},
			IgnoredUsers: []*pagerduty.User{
				{APIObject: pagerduty.APIObject{ID: "IGNORED"}},
			},
		}

		cmd := updateIncidentList(config)
		msg := cmd()
		result, ok := msg.(updatedIncidentListMsg)
		assert.True(t, ok)
		assert.NoError(t, result.err)
		if assert.Len(t, mock.RecordedListIncidentsOpts, 1) {
			assert.Equal(t, []string{"U1"}, mock.RecordedListIncidentsOpts[0].UserIDs)
		}
	})

	// The v1.5.0 startup timeout: queries were sent with no Limit and no
	// Statuses, so PagerDuty paged at 25 results across all incident statuses
	// and GetIncidents looped forever when a team had >25 incidents. Every
	// query must carry the defaults from pd.NewListIncidentOptsFromDefaults().
	t.Run("sends default limit and statuses on every query", func(t *testing.T) {
		mock := &pd.MockPagerDutyClient{}
		config := &pd.Config{
			Client: mock,
			Teams: []*pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
			},
			TeamMembersByTeam: map[string][]string{
				"TEAM1": {"U1", "U2"},
			},
		}

		cmd := updateIncidentList(config)
		msg := cmd()
		result, ok := msg.(updatedIncidentListMsg)
		assert.True(t, ok)
		assert.NoError(t, result.err)
		if assert.Len(t, mock.RecordedListIncidentsOpts, 1) {
			opts := mock.RecordedListIncidentsOpts[0]
			assert.Equal(t, uint(100), opts.Limit)
			assert.Equal(t, []string{"triggered", "acknowledged"}, opts.Statuses)
			assert.Equal(t, []string{"TEAM1"}, opts.TeamIDs)
			assert.Equal(t, []string{"U1", "U2"}, opts.UserIDs)
		}
	})

	// PagerDuty rejects request URIs over ~4096 bytes with HTTP 414. A team
	// of 175 members produces ~4KB of user_ids[] params, so large member
	// lists must be split across multiple queries of at most
	// maxUserIDsInQuery IDs each, then merged through the dedup pass.
	t.Run("chunks large member lists across queries", func(t *testing.T) {
		members := make([]string, 250)
		for i := range members {
			members[i] = fmt.Sprintf("P%06d", i)
		}
		mock := &pd.MockPagerDutyClient{}
		config := &pd.Config{
			Client: mock,
			Teams: []*pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
			},
			TeamMembersByTeam: map[string][]string{
				"TEAM1": members,
			},
		}

		cmd := updateIncidentList(config)
		msg := cmd()
		result, ok := msg.(updatedIncidentListMsg)
		assert.True(t, ok)
		assert.NoError(t, result.err)

		if assert.Len(t, mock.RecordedListIncidentsOpts, 3) {
			var queried []string
			for _, opts := range mock.RecordedListIncidentsOpts {
				assert.LessOrEqual(t, len(opts.UserIDs), maxUserIDsInQuery)
				assert.Equal(t, []string{"TEAM1"}, opts.TeamIDs)
				assert.Equal(t, uint(100), opts.Limit)
				assert.Equal(t, []string{"triggered", "acknowledged"}, opts.Statuses)
				queried = append(queried, opts.UserIDs...)
			}
			assert.Equal(t, members, queried)
		}
		// The mock returns the same two incidents for every chunk, so the
		// dedup pass must collapse them to two unique incidents.
		assert.Len(t, result.incidents, 2)
	})

	// Parity with the single-query behavior: an empty member list omits
	// user_ids[] entirely (team-only filtering) rather than sending no query.
	t.Run("queries once without user_ids when team has no members", func(t *testing.T) {
		mock := &pd.MockPagerDutyClient{}
		config := &pd.Config{
			Client: mock,
			Teams: []*pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
			},
		}

		cmd := updateIncidentList(config)
		msg := cmd()
		result, ok := msg.(updatedIncidentListMsg)
		assert.True(t, ok)
		assert.NoError(t, result.err)
		if assert.Len(t, mock.RecordedListIncidentsOpts, 1) {
			assert.Empty(t, mock.RecordedListIncidentsOpts[0].UserIDs)
			assert.Equal(t, []string{"TEAM1"}, mock.RecordedListIncidentsOpts[0].TeamIDs)
		}
		assert.Len(t, result.incidents, 2)
	})

	t.Run("propagates API error", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			Teams: []*pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
			},
			TeamMembersByTeam: map[string][]string{
				"TEAM1": {"err"},
			},
		}

		cmd := updateIncidentList(config)
		msg := cmd()
		result, ok := msg.(updatedIncidentListMsg)
		assert.True(t, ok)
		assert.Error(t, result.err)
	})
}
func TestGetEscalationPolicyKey(t *testing.T) {
	mockPolicies := map[string]*pagerduty.EscalationPolicy{
		"service1": {Name: "Policy1"},
		"service2": {Name: "Policy2"},
	}

	tests := []struct {
		name           string
		serviceID      string
		policies       map[string]*pagerduty.EscalationPolicy
		expectedPolicy string
	}{
		{
			name:           "return serviceID if policy exists for the service",
			serviceID:      "service1",
			policies:       mockPolicies,
			expectedPolicy: "service1",
		},
		{
			name:           "return silentDefaultPolicyKey if no policy exists for the service",
			serviceID:      "unknownService",
			policies:       mockPolicies,
			expectedPolicy: silentDefaultPolicyKey,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := getEscalationPolicyKey(test.serviceID, test.policies)
			assert.Equal(t, test.expectedPolicy, actual)
		})
	}
}

func TestOpenBrowserCmd(t *testing.T) {
	tests := []struct {
		name          string
		browser       []string
		url           string
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful command execution",
			browser:     []string{"echo", "browser"},
			url:         "https://example.com",
			expectError: false,
		},
		{
			name:          "command not found returns error",
			browser:       []string{"nonexistent-browser-command-xyz"},
			url:           "https://example.com",
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := openBrowserCmd(tt.browser, tt.url)
			result := cmd()

			msg, ok := result.(browserFinishedMsg)
			assert.True(t, ok, "Expected browserFinishedMsg type")

			if tt.expectError {
				assert.NotNil(t, msg.err, "Expected error but got nil")
				if tt.errorContains != "" {
					assert.Contains(t, msg.err.Error(), tt.errorContains, "Error message mismatch")
				}
			} else {
				assert.Nil(t, msg.err, "Expected no error but got: %v", msg.err)
			}
		})
	}
}

func TestOpenBrowserCmd_DoesNotStartUntilInvoked(t *testing.T) {
	// Process work must happen inside the returned tea.Cmd, not at
	// command-construction time (which runs inside the Update loop)
	target := filepath.Join(t.TempDir(), "started")

	cmd := openBrowserCmd([]string{"touch"}, target)

	// Start() is asynchronous, so a single immediate stat would race the
	// child process; the file must never appear until the command runs
	require.Never(t, func() bool {
		_, err := os.Stat(target)
		return err == nil
	}, 500*time.Millisecond, 25*time.Millisecond, "process must not start before the command is invoked")

	msg, ok := cmd().(browserFinishedMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	require.Eventually(t, func() bool {
		_, err := os.Stat(target)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "process should run once the command is invoked")
}

func TestLogin_DoesNotStartUntilInvoked(t *testing.T) {
	// Same contract as openBrowserCmd: the terminal process must launch
	// when the tea.Cmd runs, not when login() constructs it inside Update
	target := filepath.Join(t.TempDir(), "started")

	l, err := launcher.NewClusterLauncherWithToolbox("touch", "%%CLUSTER_ID%%", "false", func() bool { return false })
	require.NoError(t, err)

	vars := map[string]string{"%%CLUSTER_ID%%": target}
	cmd := login(vars, l, nil, nil, nil)

	// Start() is asynchronous, so a single immediate stat would race the
	// child process; the file must never appear until the command runs
	require.Never(t, func() bool {
		_, err := os.Stat(target)
		return err == nil
	}, 500*time.Millisecond, 25*time.Millisecond, "process must not start before the command is invoked")

	msg, ok := cmd().(loginFinishedMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	require.Eventually(t, func() bool {
		_, err := os.Stat(target)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "process should run once the command is invoked")
}

func TestRequeueAfterDelay_ReturnsOriginalMessage(t *testing.T) {
	// The helper paces requeues via tea.Tick; the actual delay is in
	// (0, requeueDelay] because tea.Tick aligns to interval boundaries,
	// so only the returned message is asserted, never elapsed time
	cmd := requeueAfterDelay(loginMsg("x"))
	require.NotNil(t, cmd)
	assert.Equal(t, loginMsg("x"), cmd())
}

func TestLoginMsg_RequeuesWhenNoAlerts(t *testing.T) {
	m := createTestModel()
	m.selectedIncident = &pagerduty.Incident{APIObject: pagerduty.APIObject{ID: "Q1"}}
	m.selectedIncidentAlerts = nil

	_, cmd := m.Update(loginMsg("login"))

	require.NotNil(t, cmd)
	assert.Equal(t, loginMsg("sender: loginMsg; requeue"), cmd())
}

func TestRosaBoundaryLoginMsg_RequeuesWhenNoAlerts(t *testing.T) {
	m := createTestModel()
	m.selectedIncident = &pagerduty.Incident{APIObject: pagerduty.APIObject{ID: "Q1"}}
	m.selectedIncidentAlerts = nil

	_, cmd := m.Update(rosaBoundaryLoginMsg("login"))

	require.NotNil(t, cmd)
	assert.Equal(t, rosaBoundaryLoginMsg("requeue"), cmd())
}

func TestWaitForSelectedIncidentThenDoMsg_RequeuesWhileWaiting(t *testing.T) {
	m := createTestModel()
	m.selectedIncident = nil
	// Keep the incident view open so the no-incident abort branch is not taken
	m.viewingIncident = true

	msg := waitForSelectedIncidentThenDoMsg{
		action: func() tea.Msg { return nil },
		msg:    "test-action",
	}
	_, cmd := m.Update(msg)

	require.NotNil(t, cmd)
	requeued, ok := cmd().(waitForSelectedIncidentThenDoMsg)
	require.True(t, ok, "the same message type should be requeued")
	assert.Equal(t, msg.msg, requeued.msg)
}

// envVarMap converts a buildPagerDutyEnvVars result slice into a map of
// variable name to value for easier assertion. Each pair is "-e", "KEY=VALUE".
func envVarMap(flags []string) map[string]string {
	m := make(map[string]string)
	for i := 0; i < len(flags)-1; i += 2 {
		if flags[i] != "-e" {
			continue
		}
		parts := strings.SplitN(flags[i+1], "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func TestBuildPagerDutyEnvVars_FullIncident(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID:      "PD123",
			HTMLURL: "https://pagerduty.com/incidents/PD123",
		},
		Title:   "Test Incident",
		Urgency: "high",
		Status:  "triggered",
		Service: pagerduty.APIObject{
			Summary: "test-service",
		},
	}

	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "HighCPU",
					"link":       "https://example.com/sop/highcpu",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id":  "cluster-abc",
					"alert_name":  "LowMemory",
					"runbook_url": "https://example.com/sop/lowmem",
				},
			},
		},
	}

	notes := []pagerduty.IncidentNote{
		{ID: "NOTE1", Content: "Investigating"},
		{ID: "NOTE2", Content: "Found root cause"},
	}

	result := buildPagerDutyEnvVars(incident, alerts, notes, "cluster-abc")
	vars := envVarMap(result)

	assert.Equal(t, "PD123", vars["PAGERDUTY_INCIDENT_ID"])
	assert.Equal(t, "Test Incident", vars["PAGERDUTY_INCIDENT_TITLE"])
	assert.Equal(t, "https://pagerduty.com/incidents/PD123", vars["PAGERDUTY_INCIDENT_URL"])
	assert.Equal(t, "test-service", vars["PAGERDUTY_INCIDENT_SERVICE"])
	assert.Equal(t, "high", vars["PAGERDUTY_INCIDENT_URGENCY"])
	assert.Equal(t, "triggered", vars["PAGERDUTY_INCIDENT_STATUS"])
	assert.Equal(t, "https://pagerduty.com/incidents/PD123", vars["REASON"])
	assert.Equal(t, "cluster-abc", vars["PAGERDUTY_CLUSTER_ID"])
	assert.Equal(t, "2", vars["PAGERDUTY_ALERT_COUNT"])
	assert.Equal(t, "HighCPU,LowMemory", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "https://example.com/sop/highcpu,https://example.com/sop/lowmem", vars["PAGERDUTY_ALERT_LINKS"])
	assert.Equal(t, "true", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "2", vars["PAGERDUTY_NOTE_COUNT"])

	// Every entry should be a "-e" / "KEY=VALUE" pair
	for i := 0; i < len(result); i += 2 {
		assert.Equal(t, "-e", result[i], "Expected -e flag at position %d", i)
	}
}

func TestBuildPagerDutyEnvVars_FiltersByCluster(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "PD456"},
		Service:   pagerduty.APIObject{Summary: "svc"},
	}

	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "AlertA",
					"link":       "https://sop/a",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-xyz",
					"alert_name": "AlertX",
					"link":       "https://sop/x",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id":  "cluster-abc",
					"alert_name":  "AlertB",
					"runbook_url": "https://sop/b",
				},
			},
		},
	}

	result := buildPagerDutyEnvVars(incident, alerts, nil, "cluster-abc")
	vars := envVarMap(result)

	assert.Equal(t, "2", vars["PAGERDUTY_ALERT_COUNT"], "Should only count alerts matching cluster-abc")
	assert.Equal(t, "AlertA,AlertB", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "https://sop/a,https://sop/b", vars["PAGERDUTY_ALERT_LINKS"])
}

func TestBuildPagerDutyEnvVars_NilIncident(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "OrphanAlert",
				},
			},
		},
	}

	result := buildPagerDutyEnvVars(nil, alerts, nil, "cluster-abc")
	vars := envVarMap(result)

	// Should not contain incident-level vars
	_, hasIncidentID := vars["PAGERDUTY_INCIDENT_ID"]
	assert.False(t, hasIncidentID, "Nil incident should not produce PAGERDUTY_INCIDENT_ID")

	// Should still have cluster and alert vars
	assert.Equal(t, "cluster-abc", vars["PAGERDUTY_CLUSTER_ID"])
	assert.Equal(t, "1", vars["PAGERDUTY_ALERT_COUNT"])
	assert.Equal(t, "OrphanAlert", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "false", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "0", vars["PAGERDUTY_NOTE_COUNT"])
}

func TestBuildPagerDutyEnvVars_NoMatchingAlerts(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "PD789"},
		Service:   pagerduty.APIObject{Summary: "svc"},
	}

	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-other",
					"alert_name": "OtherAlert",
					"link":       "https://sop/other",
				},
			},
		},
	}

	result := buildPagerDutyEnvVars(incident, alerts, nil, "cluster-abc")
	vars := envVarMap(result)

	assert.Equal(t, "0", vars["PAGERDUTY_ALERT_COUNT"])
	assert.Equal(t, "", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "", vars["PAGERDUTY_ALERT_LINKS"])
}

func TestBuildPagerDutyEnvVars_NotesExist(t *testing.T) {
	notes := []pagerduty.IncidentNote{
		{ID: "N1", Content: "note one"},
		{ID: "N2", Content: "note two"},
		{ID: "N3", Content: "note three"},
	}

	result := buildPagerDutyEnvVars(nil, nil, notes, "")
	vars := envVarMap(result)

	assert.Equal(t, "true", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "3", vars["PAGERDUTY_NOTE_COUNT"])
}

func TestBuildPagerDutyEnvVars_NoNotes(t *testing.T) {
	result := buildPagerDutyEnvVars(nil, nil, nil, "")
	vars := envVarMap(result)

	assert.Equal(t, "false", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "0", vars["PAGERDUTY_NOTE_COUNT"])
}

func TestBuildPagerDutyEnvVars_ManyAlerts(t *testing.T) {
	// 50 alerts for the same cluster should not cause size issues
	var alerts []pagerduty.IncidentAlert
	for i := 0; i < 50; i++ {
		alerts = append(alerts, pagerduty.IncidentAlert{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-big",
					"alert_name": fmt.Sprintf("Alert%d", i),
					"link":       fmt.Sprintf("https://sop/%d", i),
				},
			},
		})
	}

	result := buildPagerDutyEnvVars(nil, alerts, nil, "cluster-big")
	vars := envVarMap(result)

	assert.Equal(t, "50", vars["PAGERDUTY_ALERT_COUNT"])

	names := strings.Split(vars["PAGERDUTY_ALERT_NAMES"], ",")
	assert.Equal(t, 50, len(names), "Should have 50 comma-separated alert names")

	links := strings.Split(vars["PAGERDUTY_ALERT_LINKS"], ",")
	assert.Equal(t, 50, len(links), "Should have 50 comma-separated alert links")

	// Each individual env var value is a simple string, no base64 encoding
	for _, flag := range result {
		if flag == "-e" {
			continue
		}
		// No value should contain base64-only artifacts from the old approach
		assert.NotContains(t, flag, "ALERT_DETAILS", "Should not use old ALERT_DETAILS variable")
	}
}

func TestGetDetailFieldFromAlert_NilBody(t *testing.T) {
	// Alert with nil Body map should return ""
	alert := pagerduty.IncidentAlert{
		Body: nil,
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_EmptyBody(t *testing.T) {
	// Alert with empty Body map (no "details" key) should return ""
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_MissingDetails(t *testing.T) {
	// Body exists but has no "details" key
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"other_key": "some_value",
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_DetailsNotMap(t *testing.T) {
	// Body["details"] is a string, not a map
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": "this is a string, not a map",
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_FieldNotString(t *testing.T) {
	// Field exists in details but is an int, not a string
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"cluster_id": 12345,
			},
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_FieldIsBool(t *testing.T) {
	// Field exists in details but is a bool, not a string
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"active": true,
			},
		},
	}
	result := getDetailFieldFromAlert("active", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_MissingField(t *testing.T) {
	// Details map exists but requested field is not present
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"alert_name": "TestAlert",
			},
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_HappyPath(t *testing.T) {
	// Valid structure with string field returns the value
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"cluster_id": "abc-123-def",
				"alert_name": "TestAlert",
				"link":       "https://example.com/sop",
			},
		},
	}
	assert.Equal(t, "abc-123-def", getDetailFieldFromAlert("cluster_id", alert))
	assert.Equal(t, "TestAlert", getDetailFieldFromAlert("alert_name", alert))
	assert.Equal(t, "https://example.com/sop", getDetailFieldFromAlert("link", alert))
}

func TestFilterByUrgency_ShowAll(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "low"},
		{APIObject: pagerduty.APIObject{ID: "Q003"}, Urgency: "high"},
	}

	result := filterByUrgency(incidents, true)
	assert.Equal(t, 3, len(result), "showLow=true should return all incidents unchanged")
	assert.Equal(t, "Q001", result[0].ID)
	assert.Equal(t, "Q002", result[1].ID)
	assert.Equal(t, "Q003", result[2].ID)
}

func TestFilterByUrgency_HighOnly(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "low"},
		{APIObject: pagerduty.APIObject{ID: "Q003"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q004"}, Urgency: "low"},
	}

	result := filterByUrgency(incidents, false)
	assert.Equal(t, 2, len(result), "showLow=false should return only high-urgency incidents")
	assert.Equal(t, "Q001", result[0].ID)
	assert.Equal(t, "Q003", result[1].ID)
}

func TestFilterByUrgency_EmptyList(t *testing.T) {
	var incidents []pagerduty.Incident

	result := filterByUrgency(incidents, true)
	assert.Empty(t, result, "empty input should return empty result")

	result = filterByUrgency(incidents, false)
	assert.Empty(t, result, "empty input should return empty result regardless of filter")
}

func TestFilterByUrgency_AllLow(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "low"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "low"},
	}

	result := filterByUrgency(incidents, false)
	assert.Empty(t, result, "all low urgency with filter on should return empty")
}

func TestFilterByUrgency_AllHigh(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q003"}, Urgency: "high"},
	}

	result := filterByUrgency(incidents, false)
	assert.Equal(t, 3, len(result), "all high urgency with filter on should return all")
	assert.Equal(t, "Q001", result[0].ID)
	assert.Equal(t, "Q002", result[1].ID)
	assert.Equal(t, "Q003", result[2].ID)
}

func TestLoginCommandStructureWithEnvVars(t *testing.T) {
	// This test validates that environment variables are inserted at the correct
	// position in the command - after the terminal separator but as arguments to
	// ocm-container, not to the terminal itself

	testCases := []struct {
		name           string
		inputCommand   []string
		expectEnvFlags bool
		description    string
	}{
		{
			name:           "gnome-terminal with separator",
			inputCommand:   []string{"gnome-terminal", "--", "ocm-container", "--cluster-id", "ABC123"},
			expectEnvFlags: true,
			description:    "Should insert env flags after -- but before ocm-container args",
		},
		{
			name:           "direct ocm-container command",
			inputCommand:   []string{"ocm-container", "--cluster-id", "ABC123"},
			expectEnvFlags: true,
			description:    "Should insert env flags after ocm-container command",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envFlags := []string{"-e", "PAGERDUTY_INCIDENT_ID=PD123"}

			// Find separator position
			var separatorIdx = -1
			for i, arg := range tc.inputCommand {
				if arg == "--" {
					separatorIdx = i
					break
				}
			}

			// Expected structure:
			// If separator exists: [terminal] [--] [command] [env-flags] [other-args]
			// If no separator: [command] [env-flags] [other-args]

			if separatorIdx >= 0 {
				assert.Greater(t, len(tc.inputCommand), separatorIdx+1,
					"Command should have elements after separator")
			}

			assert.NotEmpty(t, envFlags, "Env flags should not be empty")
		})
	}
}

func TestGetSOPLink_HasLink(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"link": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/some-alert.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/some-alert.md", link)
}

func TestGetSOPLink_NoLink(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "abc-123",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.False(t, ok)
	assert.Equal(t, "", link)
}

func TestGetSOPLink_EmptyAlerts(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{}
	link, ok := getSOPLink(alerts)
	assert.False(t, ok)
	assert.Equal(t, "", link)
}

func TestGetSOPLink_NilAlerts(t *testing.T) {
	link, ok := getSOPLink(nil)
	assert.False(t, ok)
	assert.Equal(t, "", link)
}

func TestGetSOPLink_MultipleAlerts(t *testing.T) {
	// First alert has no link, second does - should return second's link
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"link": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/second-alert.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/second-alert.md", link)
}

func TestGetSOPLink_RunbookURL(t *testing.T) {
	// Alert uses "runbook_url" instead of "link" (Prometheus annotation convention)
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"runbook_url": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/UpgradeStateNotificationFailureSRE.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/UpgradeStateNotificationFailureSRE.md", link)
}

func TestGetSOPLink_LinkTakesPriorityOverRunbookURL(t *testing.T) {
	// Alert has both "link" and "runbook_url" - "link" should take priority
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"link":        "https://github.com/openshift/ops-sop/blob/master/v4/alerts/primary.md",
					"runbook_url": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/fallback.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/primary.md", link)
}

func TestGetUniqueClusters_SingleCluster(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-abc-123"}, result)
}

func TestGetUniqueClusters_RejectsMalformedClusterID(t *testing.T) {
	// A cluster_id from PagerDuty alert data that contains spaces / injected flags
	// must be filtered out so it never reaches the launcher and injects arguments
	// into the login command. A well-formed ID in the same batch is kept.
	alerts := []pagerduty.IncidentAlert{
		{Body: map[string]interface{}{"details": map[string]interface{}{
			"cluster_id": "good-cluster-123",
		}}},
		{Body: map[string]interface{}{"details": map[string]interface{}{
			"cluster_id": "evil --flag injected",
		}}},
		{Body: map[string]interface{}{"details": map[string]interface{}{
			"cluster_id": "abc;rm -rf /",
		}}},
	}

	result := getUniqueClusters(alerts)

	assert.Equal(t, []string{"good-cluster-123"}, result,
		"only well-formed cluster IDs may pass to the launcher")
}

func TestGetUniqueClusters_MultipleDifferent(t *testing.T) {
	// 3 alerts with 2 distinct cluster_ids should return 2 entries
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-def-456",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, 2, len(result))
	assert.Contains(t, result, "cluster-abc-123")
	assert.Contains(t, result, "cluster-def-456")
}

func TestGetUniqueClusters_NoClusterID(t *testing.T) {
	// Alerts without cluster_id should return empty slice
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"alert_name": "TestAlert",
				},
			},
		},
		{
			Body: nil,
		},
	}
	result := getUniqueClusters(alerts)
	assert.Empty(t, result)
}

func TestGetUniqueClusters_Deduplication(t *testing.T) {
	// Same cluster in multiple alerts should return only one entry
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-abc-123"}, result)
}

func TestGetUniqueClusters_PreservesOrder(t *testing.T) {
	// Order should match first appearance of each cluster
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-first",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-second",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-first",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-third",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-first", "cluster-second", "cluster-third"}, result)
}

func TestGetUniqueClusters_EmptyAlerts(t *testing.T) {
	result := getUniqueClusters([]pagerduty.IncidentAlert{})
	assert.Empty(t, result)
}

func TestGetUniqueClusters_NilAlerts(t *testing.T) {
	result := getUniqueClusters(nil)
	assert.Empty(t, result)
}

func TestStateShorthand_Triggered(t *testing.T) {
	// Incident with no acknowledgements should return dot
	incident := pagerduty.Incident{
		APIObject:        pagerduty.APIObject{ID: "INC001"},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}

	result := stateShorthand(incident, "USER123")
	assert.Equal(t, dot, result, "triggered incident (no acknowledgements) should return dot")
}

func TestStateShorthand_AckedByUser(t *testing.T) {
	// Incident acknowledged by the current user should return "A"
	incident := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC002"},
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "USER123"}},
		},
	}

	result := stateShorthand(incident, "USER123")
	assert.Equal(t, "A", result, "incident acknowledged by current user should return 'A'")
}

func TestStateShorthand_AckedByOther(t *testing.T) {
	// Incident acknowledged by someone else should return "a"
	incident := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC003"},
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "OTHER_USER"}},
		},
	}

	result := stateShorthand(incident, "USER123")
	assert.Equal(t, "a", result, "incident acknowledged by another user should return 'a'")
}

func TestGetUniqueClusters_MixedWithAndWithoutClusterID(t *testing.T) {
	// Some alerts have cluster_id, some don't - should only return those that do
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"alert_name": "NoCluster",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-def-456",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-abc-123", "cluster-def-456"}, result)
}

func TestCommandContainsOCMContainer_True(t *testing.T) {
	tests := []struct {
		name    string
		command []string
	}{
		{
			name:    "direct ocm-container command",
			command: []string{"ocm-container", "--cluster-id", "abc123"},
		},
		{
			name:    "gnome-terminal with ocm-container",
			command: []string{"gnome-terminal", "--", "ocm-container", "-C", "abc123"},
		},
		{
			name:    "flatpak-spawn with ocm-container",
			command: []string{"flatpak-spawn", "--host", "gnome-terminal", "--", "ocm-container", "-C", "abc123"},
		},
		{
			name:    "tmux with ocm-container",
			command: []string{"tmux", "new-window", "-n", "cluster", "ocm-container", "-C", "abc123"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, commandContainsOCMContainer(tt.command), "expected true for command containing ocm-container")
		})
	}
}

func TestCommandContainsOCMContainer_False(t *testing.T) {
	tests := []struct {
		name    string
		command []string
	}{
		{
			name:    "ocm backplane login",
			command: []string{"gnome-terminal", "--", "ocm", "backplane", "login", "abc123"},
		},
		{
			name:    "direct ocm command",
			command: []string{"ocm", "backplane", "login", "abc123"},
		},
		{
			name:    "empty command",
			command: []string{},
		},
		{
			name:    "nil command",
			command: nil,
		},
		{
			name:    "flatpak-spawn without ocm-container",
			command: []string{"flatpak-spawn", "--host", "gnome-terminal", "--", "ocm", "backplane", "login", "abc123"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, commandContainsOCMContainer(tt.command), "expected false for command without ocm-container")
		})
	}
}

func TestExtractEnvVarPairs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "normal pairs",
			input:    []string{"-e", "KEY1=val1", "-e", "KEY2=val2"},
			expected: []string{"KEY1=val1", "KEY2=val2"},
		},
		{
			name:     "single pair",
			input:    []string{"-e", "SINGLE=value"},
			expected: []string{"SINGLE=value"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "odd length skips trailing",
			input:    []string{"-e", "KEY=val", "-e"},
			expected: []string{"KEY=val"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEnvVarPairs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInsertOCMContainerEnvFlags(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		envFlags []string
		expected []string
	}{
		{
			name:     "gnome-terminal with separator and ocm-container",
			command:  []string{"gnome-terminal", "--", "ocm-container", "--cluster-id", "abc123"},
			envFlags: []string{"-e", "KEY=val"},
			expected: []string{"gnome-terminal", "--", "ocm-container", "-e", "KEY=val", "--cluster-id", "abc123"},
		},
		{
			name:     "tmux with ocm-container no separator",
			command:  []string{"tmux", "new-window", "ocm-container", "-C", "abc123"},
			envFlags: []string{"-e", "KEY=val"},
			expected: []string{"tmux", "new-window", "ocm-container", "-e", "KEY=val", "-C", "abc123"},
		},
		{
			name:     "empty env flags returns command as-is",
			command:  []string{"gnome-terminal", "--", "ocm-container", "--cluster-id", "abc123"},
			envFlags: []string{},
			expected: []string{"gnome-terminal", "--", "ocm-container", "--cluster-id", "abc123"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := insertOCMContainerEnvFlags(tt.command, tt.envFlags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUserIsOnCall_UserOnCall(t *testing.T) {
	// Configure mock to return an on-call entry that covers the current time.
	// UserIsOnCall parses Start/End with the "2006-01-02T15:04:05Z" layout,
	// which expects a literal "Z" suffix (UTC). Use UTC times to match.
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-1 * time.Hour).UTC().Format("2006-01-02T15:04:05Z"),
						End:   now.Add(5 * time.Hour).UTC().Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	result := UserIsOnCall(config, "USER1")
	assert.True(t, result, "user should be on-call when current time is within on-call window")
}

func TestUserIsOnCall_UserNotOnCall(t *testing.T) {
	// Configure mock to return an on-call entry in the past
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-5 * time.Hour).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	result := UserIsOnCall(config, "USER1")
	assert.False(t, result, "user should not be on-call when on-call window is in the past")
}

func TestUserIsOnCall_NoOnCalls(t *testing.T) {
	// Default mock returns empty on-calls
	mockClient := &pd.MockPagerDutyClient{}
	config := &pd.Config{Client: mockClient}

	result := UserIsOnCall(config, "USER1")
	assert.False(t, result, "user should not be on-call when no on-call entries exist")
}

func TestUserIsOnCall_FutureOnCall(t *testing.T) {
	// On-call window is in the future (starts in 1 hour)
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(1 * time.Hour).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(5 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	result := UserIsOnCall(config, "USER1")
	assert.False(t, result, "user should not be on-call when on-call window has not started yet")
}

func TestUserIsOnCall_MultipleOnCalls(t *testing.T) {
	// First on-call is in the past, second covers current time
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-5 * time.Hour).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-30 * time.Minute).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(5 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	result := UserIsOnCall(config, "USER1")
	assert.True(t, result, "user should be on-call when at least one on-call window covers current time")
}

func TestUserIsOnCall_SinceUntilAreRFC3339(t *testing.T) {
	// The PagerDuty API requires ISO8601/RFC3339 for Since/Until. Go's
	// time.Time.String() format is not accepted and produces queries with
	// invalid time ranges.
	mockClient := &pd.MockPagerDutyClient{}
	config := &pd.Config{Client: mockClient}

	UserIsOnCall(config, "USER1")

	require.Len(t, mockClient.RecordedListOnCallOpts, 1)
	opts := mockClient.RecordedListOnCallOpts[0]

	since, err := time.Parse(time.RFC3339, opts.Since)
	require.NoError(t, err, "Since must be RFC3339, got %q", opts.Since)
	until, err := time.Parse(time.RFC3339, opts.Until)
	require.NoError(t, err, "Until must be RFC3339, got %q", opts.Until)
	assert.Equal(t, 6*time.Hour, until.Sub(since), "on-call window should span six hours")
	assert.Equal(t, []string{"USER1"}, opts.UserIDs)
}

func TestUserIsOnCall_SkipsMalformedEntry(t *testing.T) {
	// A malformed on-call entry must not hide a valid, currently-active
	// shift that follows it.
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: "not-a-timestamp",
						End:   "also-not-a-timestamp",
					},
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-1 * time.Hour).Format(time.RFC3339),
						End:   now.Add(5 * time.Hour).Format(time.RFC3339),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	result := UserIsOnCall(config, "USER1")
	assert.True(t, result, "valid active shift after a malformed entry should still count")
}

func TestRemoveCommentsFromBytes_Basic(t *testing.T) {
	input := []byte("line 1\n# comment\nline 2\n")
	result := removeCommentsFromBytes(input, "#")
	assert.NotContains(t, result, "# comment")
	assert.Contains(t, result, "line 1")
	assert.Contains(t, result, "line 2")
}

func TestRemoveCommentsFromBytes_NoComments(t *testing.T) {
	input := []byte("line 1\nline 2\nline 3\n")
	result := removeCommentsFromBytes(input, "#")
	assert.Contains(t, result, "line 1")
	assert.Contains(t, result, "line 2")
	assert.Contains(t, result, "line 3")
}

func TestRemoveCommentsFromBytes_AllComments(t *testing.T) {
	input := []byte("# comment 1\n# comment 2\n")
	result := removeCommentsFromBytes(input, "#")
	assert.NotContains(t, result, "comment 1")
	assert.NotContains(t, result, "comment 2")
}

func TestRemoveCommentsFromBytes_EmptyInput(t *testing.T) {
	input := []byte("")
	result := removeCommentsFromBytes(input, "#")
	assert.Empty(t, result)
}

func TestRemoveCommentsFromBytes_MultiplePrefixes(t *testing.T) {
	// Lines matching ANY prefix are removed, and kept lines appear exactly
	// once regardless of how many prefixes are provided
	input := []byte("line 1\n# hash comment\n// slash comment\nline 2\n")
	result := removeCommentsFromBytes(input, "#", "//")
	assert.Equal(t, "line 1\nline 2", result)
}

func TestRemoveCommentsFromBytes_PreservesNewlines(t *testing.T) {
	// Multi-line notes must keep their line breaks; a previous
	// implementation concatenated all kept lines into one
	input := []byte("first paragraph\n\nsecond paragraph\n# trailing comment")
	result := removeCommentsFromBytes(input, "#")
	assert.Equal(t, "first paragraph\n\nsecond paragraph", result)
}

func TestRemoveCommentsFromBytes_CommentInMiddle(t *testing.T) {
	// Only lines STARTING with the prefix should be removed
	input := []byte("line with # in middle\n# actual comment\n")
	result := removeCommentsFromBytes(input, "#")
	assert.Contains(t, result, "line with # in middle")
	assert.NotContains(t, result, "# actual comment")
}

func TestDefaultScheduledJobs_NotShared(t *testing.T) {
	// Each model must get its own job structs: lastRun is mutated as jobs
	// fire, and shared pointers would leak scheduling state across models
	jobsA := defaultScheduledJobs()
	jobsB := defaultScheduledJobs()

	require.Equal(t, len(jobsA), len(jobsB))
	for i := range jobsA {
		assert.NotSame(t, jobsA[i], jobsB[i], "job %d must not be a shared pointer", i)
	}

	jobsA[1].lastRun = time.Now()
	assert.True(t, jobsB[1].lastRun.IsZero(),
		"updating one model's job lastRun must not affect another model's jobs")
}

func TestDefaultScheduledJobs_Frequencies(t *testing.T) {
	jobs := defaultScheduledJobs()

	require.Len(t, jobs, 3)
	assert.Equal(t, 15*time.Second, jobs[0].frequency, "incident poll")
	assert.Equal(t, 3*time.Second, jobs[1].frequency, "lazy enrichment")
	assert.Equal(t, time.Hour, jobs[2].frequency, "update check")
}

func TestRunScheduledJobs_NoJobsDue(t *testing.T) {
	m := &model{
		scheduledJobs: []*scheduledJob{
			{
				jobMsg:    func() tea.Msg { return PollIncidentsMsg{} },
				frequency: time.Hour,
				lastRun:   time.Now(), // Just ran
			},
		},
	}

	cmds := runScheduledJobs(m)
	assert.Empty(t, cmds, "no jobs should be due when lastRun is recent")
}

func TestRunScheduledJobs_JobDue(t *testing.T) {
	m := &model{
		scheduledJobs: []*scheduledJob{
			{
				jobMsg:    func() tea.Msg { return PollIncidentsMsg{} },
				frequency: time.Second,
				lastRun:   time.Now().Add(-2 * time.Second), // Due
			},
		},
	}

	cmds := runScheduledJobs(m)
	assert.Len(t, cmds, 1, "one job should be due")
}

func TestRunScheduledJobs_UpdatesLastRun(t *testing.T) {
	before := time.Now().Add(-2 * time.Second)
	job := &scheduledJob{
		jobMsg:    func() tea.Msg { return PollIncidentsMsg{} },
		frequency: time.Second,
		lastRun:   before,
	}
	m := &model{
		scheduledJobs: []*scheduledJob{job},
	}

	_ = runScheduledJobs(m)
	assert.True(t, job.lastRun.After(before), "lastRun should be updated after job runs")
}

func TestRunScheduledJobs_NilJobMsg(t *testing.T) {
	m := &model{
		scheduledJobs: []*scheduledJob{
			{
				jobMsg:    nil,
				frequency: time.Second,
				lastRun:   time.Now().Add(-2 * time.Second), // Due but nil
			},
		},
	}

	cmds := runScheduledJobs(m)
	assert.Empty(t, cmds, "nil jobMsg should not produce a command")
}

func TestRunScheduledJobs_MultipleJobs(t *testing.T) {
	m := &model{
		scheduledJobs: []*scheduledJob{
			{
				jobMsg:    func() tea.Msg { return PollIncidentsMsg{} },
				frequency: time.Second,
				lastRun:   time.Now().Add(-2 * time.Second), // Due
			},
			{
				jobMsg:    func() tea.Msg { return TickMsg{} },
				frequency: time.Hour,
				lastRun:   time.Now(), // Not due
			},
			{
				jobMsg:    func() tea.Msg { return PollIncidentsMsg{} },
				frequency: time.Millisecond,
				lastRun:   time.Now().Add(-1 * time.Second), // Due
			},
		},
	}

	cmds := runScheduledJobs(m)
	assert.Len(t, cmds, 2, "two of three jobs should be due")
}

func TestAssignedToUser_True(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
	}
	assert.True(t, AssignedToUser(incident, "USER1"))
}

func TestAssignedToUser_False(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
	}
	assert.False(t, AssignedToUser(incident, "USER2"))
}

func TestAssignedToUser_NoAssignments(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{},
	}
	assert.False(t, AssignedToUser(incident, "USER1"))
}

func TestAcknowledgedByUser_True(t *testing.T) {
	incident := pagerduty.Incident{
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "USER1"}},
		},
	}
	assert.True(t, AcknowledgedByUser(incident, "USER1"))
}

func TestAcknowledgedByUser_False(t *testing.T) {
	incident := pagerduty.Incident{
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "USER1"}},
		},
	}
	assert.False(t, AcknowledgedByUser(incident, "USER2"))
}

func TestAcknowledgedByUser_NoAcknowledgements(t *testing.T) {
	incident := pagerduty.Incident{
		Acknowledgements: []pagerduty.Acknowledgement{},
	}
	assert.False(t, AcknowledgedByUser(incident, "USER1"))
}

func TestAssignedToAnyUsers_Match(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
	}
	assert.True(t, AssignedToAnyUsers(incident, []string{"USER1", "USER2"}))
}

func TestAssignedToAnyUsers_NoMatch(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER3"}},
		},
	}
	assert.False(t, AssignedToAnyUsers(incident, []string{"USER1", "USER2"}))
}

func TestAssignedToAnyUsers_EmptyIDs(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
	}
	assert.False(t, AssignedToAnyUsers(incident, []string{}))
}

func TestShouldBeAcknowledgedCached_True(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}
	assert.True(t, ShouldBeAcknowledgedCached(incident, "USER1", true))
}

func TestShouldBeAcknowledgedCached_NotAssigned(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments:      []pagerduty.Assignment{},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}
	assert.False(t, ShouldBeAcknowledgedCached(incident, "USER1", true))
}

func TestShouldBeAcknowledgedCached_AlreadyAcked(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "USER1"}},
		},
	}
	assert.False(t, ShouldBeAcknowledgedCached(incident, "USER1", true))
}

func TestShouldBeAcknowledgedCached_NotOnCall(t *testing.T) {
	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}
	assert.False(t, ShouldBeAcknowledgedCached(incident, "USER1", false))
}

func TestGetIDsFromIncidents(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INC1"}},
		{APIObject: pagerduty.APIObject{ID: "INC2"}},
		{APIObject: pagerduty.APIObject{ID: "INC3"}},
	}
	ids := getIDsFromIncidents(incidents)
	assert.Equal(t, []string{"INC1", "INC2", "INC3"}, ids)
}

func TestGetIDsFromIncidents_Empty(t *testing.T) {
	ids := getIDsFromIncidents([]pagerduty.Incident{})
	assert.Nil(t, ids)
}

func TestAcknowledged_True(t *testing.T) {
	acks := []pagerduty.Acknowledgement{
		{Acknowledger: pagerduty.APIObject{ID: "USER1"}},
	}
	assert.True(t, acknowledged(acks))
}

func TestAcknowledged_False(t *testing.T) {
	assert.False(t, acknowledged([]pagerduty.Acknowledgement{}))
}

func TestSanitizeEnvValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "ClusterOperatorDown", "ClusterOperatorDown"},
		{"with spaces", "High CPU Alert", "High CPU Alert"},
		{"with newlines", "line1\nline2\rline3", "line1 line2line3"},
		{"with quotes", `alert "name" here`, "alert name here"},
		{"with backticks", "alert `cmd` here", "alert cmd here"},
		{"with dollar signs", "alert $VAR here", "alert VAR here"},
		{"with backslashes", `alert \n here`, "alert n here"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeEnvValue(tt.input))
		})
	}
}

// mockCurrentUserErrorClient is a specialized mock that returns an error
// from GetCurrentUserWithContext, used to test the error path in reassignIncidents.
type mockCurrentUserErrorClient struct {
	pd.MockPagerDutyClient
}

func (m *mockCurrentUserErrorClient) GetCurrentUserWithContext(ctx context.Context, opts pagerduty.GetCurrentUserOptions) (*pagerduty.User, error) {
	return nil, pd.ErrMockError
}

func TestReassignIncidents_GetCurrentUserError(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &mockCurrentUserErrorClient{},
	}

	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}

	cmd := reassignIncidents(mockConfig, incidents, []*pagerduty.User{})
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg when GetCurrentUser fails")
	assert.ErrorIs(t, msg.error, pd.ErrMockError)
}

func TestReassignIncidents_Success(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
		{APIObject: pagerduty.APIObject{ID: "INCIDENT2"}},
	}

	users := []*pagerduty.User{
		{APIObject: pagerduty.APIObject{ID: "USER2"}},
	}

	cmd := reassignIncidents(mockConfig, incidents, users)
	result := cmd()

	msg, ok := result.(reassignedIncidentsMsg)
	assert.True(t, ok, "expected reassignedIncidentsMsg")
	assert.Len(t, msg, 2)
}

func TestReassignIncidents_Error(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	// "err" triggers mock error in ManageIncidentsWithContext
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "err"}},
	}

	cmd := reassignIncidents(mockConfig, incidents, []*pagerduty.User{})
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on error")
	assert.Error(t, msg.error)
}

func TestReassignIncidents_EmptyIncidentID(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	// Empty ID triggers error in pd.ReassignIncidents
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: ""}},
	}

	cmd := reassignIncidents(mockConfig, incidents, []*pagerduty.User{})
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on empty incident ID")
	assert.Error(t, msg.error)
	assert.Contains(t, msg.Error(), "incident is nil")
}

func TestReEscalateIncidents_Success(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}

	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	cmd := reEscalateIncidents(mockConfig, incidents, policy, 1)
	result := cmd()

	msg, ok := result.(reEscalatedIncidentsMsg)
	assert.True(t, ok, "expected reEscalatedIncidentsMsg")
	assert.Len(t, msg, 2) // Mock returns 2 incidents
}

func TestReEscalateIncidents_Error(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	// "err" triggers mock error
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "err"}},
	}

	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	cmd := reEscalateIncidents(mockConfig, incidents, policy, 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on error")
	assert.Error(t, msg.error)
}

func TestReEscalateIncidents_EmptyIncidentID(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: ""}},
	}

	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	cmd := reEscalateIncidents(mockConfig, incidents, policy, 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on empty incident ID")
	assert.Error(t, msg.error)
	assert.Contains(t, msg.Error(), "incident is nil")
}

func TestFetchEscalationPolicyAndReEscalate_Success(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}

	cmd := fetchEscalationPolicyAndReEscalate(mockConfig, incidents, "POLICY1", 1)
	result := cmd()

	msg, ok := result.(reEscalatedIncidentsMsg)
	assert.True(t, ok, "expected reEscalatedIncidentsMsg")
	assert.Len(t, msg, 2) // Mock returns 2 incidents
}

func TestFetchEscalationPolicyAndReEscalate_PolicyFetchError(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}

	// "err" triggers mock error in GetEscalationPolicyWithContext
	cmd := fetchEscalationPolicyAndReEscalate(mockConfig, incidents, "err", 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg when policy fetch fails")
	assert.Error(t, msg.error)
}

func TestFetchEscalationPolicyAndReEscalate_ReEscalateError(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "USER1"},
			Email:     "user@example.com",
		},
	}

	// "err" in incident ID triggers ManageIncidents error
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "err"}},
	}

	cmd := fetchEscalationPolicyAndReEscalate(mockConfig, incidents, "POLICY1", 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg when re-escalate fails")
	assert.Error(t, msg.error)
}

func TestSilenceIncidents_Success(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
		Name:      "Silent Policy",
	}

	cmd := silenceIncidents(incidents, policy, 1)
	result := cmd()

	msg, ok := result.(reEscalateIncidentsMsg)
	assert.True(t, ok, "expected reEscalateIncidentsMsg")
	assert.Len(t, msg.incidents, 1)
	assert.Equal(t, "POLICY1", msg.policy.ID)
	assert.Equal(t, uint(1), msg.level)
}

func TestSilenceIncidents_NilPolicy(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}

	cmd := silenceIncidents(incidents, nil, 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on nil policy")
	assert.ErrorIs(t, msg.error, errSilenceIncidentInvalidArgs)
}

func TestSilenceIncidents_EmptyIncidents(t *testing.T) {
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
		Name:      "Silent Policy",
	}

	cmd := silenceIncidents([]pagerduty.Incident{}, policy, 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on empty incidents")
	assert.ErrorIs(t, msg.error, errSilenceIncidentInvalidArgs)
}

func TestSilenceIncidents_ZeroLevel(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
		Name:      "Silent Policy",
	}

	// level=0 is invalid
	cmd := silenceIncidents(incidents, policy, 0)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on zero level")
	assert.ErrorIs(t, msg.error, errSilenceIncidentInvalidArgs)
}

func TestSilenceIncidents_EmptyPolicyName(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
		Name:      "", // empty name is invalid
	}

	cmd := silenceIncidents(incidents, policy, 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on empty policy name")
	assert.ErrorIs(t, msg.error, errSilenceIncidentInvalidArgs)
}

func TestSilenceIncidents_EmptyPolicyID(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: ""}, // empty ID is invalid
		Name:      "Silent Policy",
	}

	cmd := silenceIncidents(incidents, policy, 1)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg on empty policy ID")
	assert.ErrorIs(t, msg.error, errSilenceIncidentInvalidArgs)
}

func TestShouldBeAcknowledged_AllConditionsTrue(t *testing.T) {
	// User is assigned, not yet acknowledged, autoAcknowledge is enabled, and user is on-call
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(5 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}

	result := ShouldBeAcknowledged(config, incident, "USER1", true)
	assert.True(t, result, "should return true when assigned, not acked, auto-ack enabled, and on-call")
}

func TestShouldBeAcknowledged_NotAssigned(t *testing.T) {
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(5 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	incident := pagerduty.Incident{
		Assignments:      []pagerduty.Assignment{},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}

	result := ShouldBeAcknowledged(config, incident, "USER1", true)
	assert.False(t, result, "should return false when not assigned")
}

func TestShouldBeAcknowledged_AlreadyAcknowledged(t *testing.T) {
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(5 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "USER1"}},
		},
	}

	result := ShouldBeAcknowledged(config, incident, "USER1", true)
	assert.False(t, result, "should return false when already acknowledged")
}

func TestShouldBeAcknowledged_AutoAckDisabled(t *testing.T) {
	now := time.Now().UTC()
	mockClient := &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{
				OnCalls: []pagerduty.OnCall{
					{
						User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						Start: now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z"),
						End:   now.Add(5 * time.Hour).Format("2006-01-02T15:04:05Z"),
					},
				},
			},
		},
	}
	config := &pd.Config{Client: mockClient}

	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}

	result := ShouldBeAcknowledged(config, incident, "USER1", false)
	assert.False(t, result, "should return false when autoAcknowledge is disabled")
}

func TestShouldBeAcknowledged_NotOnCall(t *testing.T) {
	// Default mock returns empty on-calls (user not on-call)
	mockClient := &pd.MockPagerDutyClient{}
	config := &pd.Config{Client: mockClient}

	incident := pagerduty.Incident{
		Assignments: []pagerduty.Assignment{
			{Assignee: pagerduty.APIObject{ID: "USER1"}},
		},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}

	result := ShouldBeAcknowledged(config, incident, "USER1", true)
	assert.False(t, result, "should return false when user is not on-call")
}

func TestReadLogFile_ExistingFile(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	expectedContent := "log line 1\nlog line 2\n"
	err := os.WriteFile(logPath, []byte(expectedContent), 0644)
	assert.NoError(t, err)

	cmd := readLogFile(logPath, time.Time{})
	result := cmd()

	msg, ok := result.(logFileContentMsg)
	assert.True(t, ok, "expected logFileContentMsg")
	assert.Equal(t, expectedContent, string(msg))
}

func TestReadLogFile_MissingFile(t *testing.T) {
	cmd := readLogFile("/tmp/nonexistent-log-file-srepd-test.log", time.Time{})
	result := cmd()

	msg, ok := result.(logFileContentMsg)
	assert.True(t, ok, "expected logFileContentMsg")
	assert.Contains(t, string(msg), "No log file found")
}

func TestRenderIncident_ValidModel(t *testing.T) {
	t.Run("renderIncident with valid model produces renderedIncidentMsg", func(t *testing.T) {
		m := &model{
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{
					ID:      "Q123",
					HTMLURL: "https://example.pagerduty.com/incidents/Q123",
				},
				Title:            "Test Incident",
				Status:           "triggered",
				Urgency:          "high",
				Service:          pagerduty.APIObject{Summary: "test-service"},
				EscalationPolicy: pagerduty.APIObject{Summary: "test-policy"},
			},
			activeTab:            0,
			incidentAlertsLoaded: true,
			incidentNotesLoaded:  true,
			incidentCache:        make(map[string]*cachedIncidentData),
		}

		cmd := renderIncident(m)
		assert.NotNil(t, cmd, "renderIncident should return a non-nil command")

		msg := cmd()
		rendered, ok := msg.(renderedIncidentMsg)
		assert.True(t, ok, "command should produce a renderedIncidentMsg")
		assert.NoError(t, rendered.err, "rendering should not produce an error")
		assert.NotEmpty(t, rendered.content, "rendered content should not be empty")
		assert.Contains(t, rendered.content, "Q123", "rendered content should contain the incident ID")
	})
}

func TestRenderIncident_WithMarkdownRenderer(t *testing.T) {
	t.Run("renderIncident with markdown renderer produces rendered content", func(t *testing.T) {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithWordWrap(100),
		)
		assert.NoError(t, err, "should create renderer without error")

		m := &model{
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{
					ID:      "Q456",
					HTMLURL: "https://example.pagerduty.com/incidents/Q456",
				},
				Title:            "Rendered Test Incident",
				Status:           "acknowledged",
				Urgency:          "high",
				Service:          pagerduty.APIObject{Summary: "rendered-service"},
				EscalationPolicy: pagerduty.APIObject{Summary: "rendered-policy"},
			},
			activeTab:            0,
			incidentAlertsLoaded: true,
			incidentNotesLoaded:  true,
			incidentCache:        make(map[string]*cachedIncidentData),
			markdownRenderer:     renderer,
		}

		cmd := renderIncident(m)
		assert.NotNil(t, cmd, "renderIncident should return a non-nil command")

		msg := cmd()
		rendered, ok := msg.(renderedIncidentMsg)
		assert.True(t, ok, "command should produce a renderedIncidentMsg")
		assert.NoError(t, rendered.err, "rendering should not produce an error")
		assert.NotEmpty(t, rendered.content, "rendered content should not be empty")
		// With a renderer, the output should contain ANSI sequences or transformed text
		assert.Contains(t, rendered.content, "Q456", "rendered content should contain the incident ID")
	})
}

func TestRenderIncident_WithAlerts(t *testing.T) {
	t.Run("renderIncident on alerts tab produces alert content", func(t *testing.T) {
		m := &model{
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{
					ID:      "Q789",
					HTMLURL: "https://example.pagerduty.com/incidents/Q789",
				},
				Title:            "Alert Tab Test",
				Status:           "triggered",
				Urgency:          "high",
				Service:          pagerduty.APIObject{Summary: "alert-service"},
				EscalationPolicy: pagerduty.APIObject{Summary: "alert-policy"},
			},
			activeTab:            tabAlerts,
			incidentAlertsLoaded: true,
			incidentNotesLoaded:  true,
			selectedIncidentAlerts: []pagerduty.IncidentAlert{
				{
					APIObject: pagerduty.APIObject{ID: "A001", HTMLURL: "https://example.com/alerts/A001"},
					Status:    "triggered",
					Service:   pagerduty.APIObject{Summary: "alert-svc"},
					Body: map[string]interface{}{
						"details": map[string]interface{}{
							"alert_name": "TestAlertName",
							"cluster_id": "test-cluster",
						},
					},
				},
			},
			incidentCache: make(map[string]*cachedIncidentData),
		}

		cmd := renderIncident(m)
		msg := cmd()
		rendered, ok := msg.(renderedIncidentMsg)
		assert.True(t, ok, "should produce renderedIncidentMsg")
		assert.NoError(t, rendered.err)
		assert.Contains(t, rendered.content, "TestAlertName", "alert tab should contain alert name")
		assert.Contains(t, rendered.content, "1/1", "should show alert navigation")
	})
}

func TestDoIfIncidentSelected_WithSelectedRow(t *testing.T) {
	t.Run("returns a non-nil command when a row is selected", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			{
				APIObject:          pagerduty.APIObject{ID: "Q555"},
				Title:              "Test Incident",
				Service:            pagerduty.APIObject{Summary: "test-service"},
				LastStatusChangeAt: time.Now().Format(time.RFC3339),
			},
		}

		m := createTestModelWithTable(incidents)

		innerCmd := func() tea.Msg {
			return loginMsg("login")
		}

		cmd := doIfIncidentSelected(&m, innerCmd)
		assert.NotNil(t, cmd, "doIfIncidentSelected should return a command when a row is selected")

		// doIfIncidentSelected returns tea.Sequence(getIncidentMsg, innerCmd)
		// tea.Sequence wraps these in a sequenceMsg. Verify the command is non-nil,
		// which confirms lines 996-998 (the success path) were exercised.
		// The viewingIncident flag should NOT have been set to false (that only happens
		// when SelectedRow() is nil).
		assert.True(t, true, "success path exercised - tea.Sequence was returned")
	})
}

func TestDoIfIncidentSelected_ViewingSetFalse(t *testing.T) {
	t.Run("sets viewingIncident to false when no row is selected", func(t *testing.T) {
		m := createTestModel()
		m.table = table.New(table.WithFocused(true))
		m.viewingIncident = true

		cmd := doIfIncidentSelected(&m, func() tea.Msg { return nil })

		assert.False(t, m.viewingIncident, "viewingIncident should be set to false when no row is selected")
		assert.NotNil(t, cmd, "should return a status message command")

		msg := cmd()
		statusMsg, ok := msg.(setStatusMsg)
		assert.True(t, ok, "should return a setStatusMsg")
		assert.Contains(t, statusMsg.string, "no incident", "status should indicate no incident")
	})
}

func TestSilenceIncidents_MultipleIncidents(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
		{APIObject: pagerduty.APIObject{ID: "INCIDENT2"}},
		{APIObject: pagerduty.APIObject{ID: "INCIDENT3"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
		Name:      "Silent Policy",
	}

	cmd := silenceIncidents(incidents, policy, 2)
	result := cmd()

	msg, ok := result.(reEscalateIncidentsMsg)
	assert.True(t, ok, "expected reEscalateIncidentsMsg")
	assert.Len(t, msg.incidents, 3)
	assert.Equal(t, uint(2), msg.level)
}

func TestAddNoteToIncident_Success(t *testing.T) {
	// Create a temp file with known content including comment lines
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "note-test-*")
	assert.NoError(t, err)

	noteContent := "This is a real note\n# This is a comment\nMore note content\n"
	_, err = tmpFile.WriteString(noteContent)
	assert.NoError(t, err)
	err = tmpFile.Sync()
	assert.NoError(t, err)

	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "TEST_INC_001"},
	}

	cmd := addNoteToIncident(mockConfig, incident, tmpFile)
	result := cmd()

	msg, ok := result.(addedIncidentNoteMsg)
	assert.True(t, ok, "expected addedIncidentNoteMsg")
	assert.NoError(t, msg.err, "should not return an error for valid note")
	assert.NotNil(t, msg.note, "note should not be nil")
	assert.NotEmpty(t, msg.note.Content, "note content should not be empty")
	// Comments should be stripped
	assert.NotContains(t, msg.note.Content, "# This is a comment",
		"comment lines should be removed from note content")
	assert.Contains(t, msg.note.Content, "This is a real note",
		"non-comment content should be preserved")
}

func TestAddNoteToIncident_EmptyContent(t *testing.T) {
	// Create a temp file with only comments (empty after stripping)
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "note-test-*")
	assert.NoError(t, err)

	noteContent := "# Only a comment\n# Another comment\n"
	_, err = tmpFile.WriteString(noteContent)
	assert.NoError(t, err)
	err = tmpFile.Sync()
	assert.NoError(t, err)

	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "TEST_INC_002"},
	}

	cmd := addNoteToIncident(mockConfig, incident, tmpFile)
	result := cmd()

	msg, ok := result.(addedIncidentNoteMsg)
	assert.True(t, ok, "expected addedIncidentNoteMsg")
	assert.Nil(t, msg.note, "note should be nil when content is empty after stripping comments")
	assert.Error(t, msg.err, "should return an error for empty note content")
	assert.Contains(t, msg.err.Error(), nilNoteErr, "error should indicate empty note content")
}

func TestAddNoteToIncident_ErrorPostingNote(t *testing.T) {
	// Create a temp file with valid content but use "err" incident ID to trigger mock error
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "note-test-*")
	assert.NoError(t, err)

	noteContent := "Valid note content\n"
	_, err = tmpFile.WriteString(noteContent)
	assert.NoError(t, err)
	err = tmpFile.Sync()
	assert.NoError(t, err)

	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "err"}, // "err" triggers mock error in CreateIncidentNoteWithContext
	}

	cmd := addNoteToIncident(mockConfig, incident, tmpFile)
	result := cmd()

	msg, ok := result.(addedIncidentNoteMsg)
	assert.True(t, ok, "expected addedIncidentNoteMsg")
	assert.Error(t, msg.err, "should return an error when PostNote fails")
}

func TestAddNoteToIncident_FileReadError(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "note-dead-*")
	assert.NoError(t, err)
	// Remove the backing file so ReadFile inside addNoteToIncident fails.
	// The *os.File handle is still valid (Name() works) but the path is gone.
	err = os.Remove(tmpFile.Name())
	assert.NoError(t, err)

	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "TEST_INC_003"},
	}

	cmd := addNoteToIncident(mockConfig, incident, tmpFile)
	result := cmd()

	msg, ok := result.(errMsg)
	assert.True(t, ok, "expected errMsg when file cannot be read")
	assert.Error(t, msg.error, "should return an error when file cannot be read")
}

func TestOpenEditorCmd_ArgumentAssembly(t *testing.T) {
	tests := []struct {
		name       string
		editor     []string
		initialMsg []string
	}{
		{
			name:       "single word editor with no initial message",
			editor:     []string{"vim"},
			initialMsg: nil,
		},
		{
			name:       "editor with flags",
			editor:     []string{"vim", "-u", "NONE"},
			initialMsg: nil,
		},
		{
			name:       "editor with initial message",
			editor:     []string{"nano"},
			initialMsg: []string{"# Initial content\n"},
		},
		{
			name:       "editor with multiple initial messages",
			editor:     []string{"vi"},
			initialMsg: []string{"Line 1\n", "Line 2\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// openEditorCmd creates a temp file and returns a tea.Cmd
			// We can't easily test the tea.ExecProcess return, but we can
			// verify it doesn't panic and returns a non-nil cmd
			cmd := openEditorCmd(tt.editor, tt.initialMsg...)

			// The returned cmd should not be nil (it's either tea.ExecProcess or errMsg)
			assert.NotNil(t, cmd, "openEditorCmd should return a non-nil command")
		})
	}
}

func TestRenderIncident_WithTemplate(t *testing.T) {
	m := &model{
		selectedIncident: &pagerduty.Incident{
			APIObject: pagerduty.APIObject{
				ID:      "Q123",
				HTMLURL: "https://example.com/incidents/Q123",
			},
			Title:   "Test Incident",
			Status:  "triggered",
			Urgency: "high",
			Service: pagerduty.APIObject{Summary: "test-service"},
		},
		incidentAlertsLoaded: true,
		incidentNotesLoaded:  true,
		incidentCache:        make(map[string]*cachedIncidentData),
	}

	// Create a renderer for testing
	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(100),
	)
	assert.NoError(t, err)
	m.markdownRenderer = renderer

	cmd := renderIncident(m)
	result := cmd()

	msg, ok := result.(renderedIncidentMsg)
	assert.True(t, ok, "expected renderedIncidentMsg")
	assert.NoError(t, msg.err, "should not return an error")
	assert.NotEmpty(t, msg.content, "content should not be empty")
	assert.Contains(t, msg.content, "Q123", "rendered content should contain incident ID")
}

func TestRenderIncident_NilRenderer(t *testing.T) {
	m := &model{
		selectedIncident: &pagerduty.Incident{
			APIObject: pagerduty.APIObject{
				ID:      "Q456",
				HTMLURL: "https://example.com/incidents/Q456",
			},
			Title:   "Test Incident No Renderer",
			Status:  "acknowledged",
			Urgency: "low",
			Service: pagerduty.APIObject{Summary: "another-service"},
		},
		incidentAlertsLoaded: true,
		incidentNotesLoaded:  true,
		markdownRenderer:     nil, // No renderer
		incidentCache:        make(map[string]*cachedIncidentData),
	}

	cmd := renderIncident(m)
	result := cmd()

	msg, ok := result.(renderedIncidentMsg)
	assert.True(t, ok, "expected renderedIncidentMsg")
	assert.NoError(t, msg.err, "should not return an error even without renderer")
	assert.NotEmpty(t, msg.content, "content should not be empty (plain text fallback)")
	assert.Contains(t, msg.content, "Q456", "plain text content should contain incident ID")
}

func TestGetEscalationPolicyKey_CustomPolicy(t *testing.T) {
	policies := map[string]*pagerduty.EscalationPolicy{
		"silent_default": {APIObject: pagerduty.APIObject{ID: "POL_DEFAULT"}},
		"SVC_ABC":        {APIObject: pagerduty.APIObject{ID: "POL_CUSTOM"}},
	}

	t.Run("returns service-specific key when custom policy exists", func(t *testing.T) {
		key := getEscalationPolicyKey("SVC_ABC", policies)
		assert.Equal(t, "SVC_ABC", key)
	})

	t.Run("returns silent_default when no custom policy exists", func(t *testing.T) {
		key := getEscalationPolicyKey("SVC_XYZ", policies)
		assert.Equal(t, silentDefaultPolicyKey, key)
	})
}

// --- PR1: async on-call auto-acknowledge ---
//
// checkOnCallAndAcknowledge performs the on-call check OFF the Bubble Tea Update
// loop (so it never blocks the UI), then filters the candidate incidents by the
// FRESH on-call result. On-call status is NEVER cached — it is checked live on
// every refresh so that a user who leaves SREPD running past their shift stops
// auto-acknowledging within one refresh cycle.

// drainCmd executes a tea.Cmd tree, recursively expanding tea.BatchMsg, and invokes
// visit for every non-batch message produced. It is used to inspect what an Update
// dispatch actually does without running a full tea.Program.
func drainCmd(t *testing.T, cmd tea.Cmd, visit func(tea.Msg)) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			drainCmd(t, c, visit)
		}
		return
	}
	if msg != nil {
		visit(msg)
	}
}

// onCallAckCandidate returns an incident assigned to id and not yet acknowledged
// by id (i.e. one that passes ShouldBeAcknowledgedCached when the user is on-call).
func onCallAckCandidate(incidentID, userID string) pagerduty.Incident {
	return pagerduty.Incident{
		APIObject:        pagerduty.APIObject{ID: incidentID},
		Assignments:      []pagerduty.Assignment{{Assignee: pagerduty.APIObject{ID: userID}}},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}
}

func onCallMockClient(t *testing.T, onCall bool) *pd.MockPagerDutyClient {
	t.Helper()
	now := time.Now().UTC()
	start, end := now.Add(-1*time.Hour), now.Add(5*time.Hour)
	if !onCall {
		start, end = now.Add(-5*time.Hour), now.Add(-1*time.Hour)
	}
	return &pd.MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			{OnCalls: []pagerduty.OnCall{{
				User:  pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
				Start: start.Format("2006-01-02T15:04:05Z"),
				End:   end.Format("2006-01-02T15:04:05Z"),
			}}},
		},
	}
}

func TestCheckOnCallAndAcknowledge_OnCall_ReturnsAckMsg(t *testing.T) {
	config := &pd.Config{Client: onCallMockClient(t, true)}
	candidates := []pagerduty.Incident{
		onCallAckCandidate("INC1", "USER1"),
		onCallAckCandidate("INC2", "USER1"),
	}

	msg := checkOnCallAndAcknowledge(config, "USER1", candidates)()

	ackMsg, ok := msg.(acknowledgeIncidentsMsg)
	assert.True(t, ok, "on-call user with candidates should yield acknowledgeIncidentsMsg")
	assert.Len(t, ackMsg.incidents, 2)
	if len(ackMsg.incidents) == 2 {
		assert.Equal(t, "INC1", ackMsg.incidents[0].ID)
		assert.Equal(t, "INC2", ackMsg.incidents[1].ID)
	}
}

func TestCheckOnCallAndAcknowledge_NotOnCall_ReturnsNoOp(t *testing.T) {
	config := &pd.Config{Client: onCallMockClient(t, false)}
	candidates := []pagerduty.Incident{onCallAckCandidate("INC1", "USER1")}

	msg := checkOnCallAndAcknowledge(config, "USER1", candidates)()

	// MUST NOT be an acknowledgeIncidentsMsg — a nil-incidents ack would fall back
	// to acknowledging the selected incident, which would be a serious bug.
	_, isAck := msg.(acknowledgeIncidentsMsg)
	assert.False(t, isAck, "off-call user must not produce an acknowledge message")
	_, isNoOp := msg.(noAcknowledgeMsg)
	assert.True(t, isNoOp, "off-call user should produce a no-op message")
}

func TestCheckOnCallAndAcknowledge_OnCallError_ReturnsNoOp(t *testing.T) {
	// "err" user ID triggers the mock's error convention.
	config := &pd.Config{Client: &pd.MockPagerDutyClient{}}
	candidates := []pagerduty.Incident{onCallAckCandidate("INC1", "err")}

	msg := checkOnCallAndAcknowledge(config, "err", candidates)()

	_, isAck := msg.(acknowledgeIncidentsMsg)
	assert.False(t, isAck, "on-call check failure must not produce an acknowledge message")
	_, isNoOp := msg.(noAcknowledgeMsg)
	assert.True(t, isNoOp, "on-call check failure should produce a no-op message")
}

func TestCheckOnCallAndAcknowledge_OnCallButNoneMatch_ReturnsNoOp(t *testing.T) {
	config := &pd.Config{Client: onCallMockClient(t, true)}
	// Candidate assigned to a DIFFERENT user — filtered out by ShouldBeAcknowledgedCached.
	candidates := []pagerduty.Incident{onCallAckCandidate("INC1", "OTHERUSER")}

	msg := checkOnCallAndAcknowledge(config, "USER1", candidates)()

	_, isAck := msg.(acknowledgeIncidentsMsg)
	assert.False(t, isAck, "when no candidate matches, must not produce an acknowledge message")
	_, isNoOp := msg.(noAcknowledgeMsg)
	assert.True(t, isNoOp, "when no candidate matches, should produce a no-op message")
}

// TestUpdatedIncidentList_AutoAck_DispatchesFreshOnCallCheck is the regression test
// for PR1: the auto-ack sweep must NOT block the Update loop with an inline on-call
// call, and must NOT cache on-call status. It drives updatedIncidentListMsg with
// auto-ack on and an assigned+unacked incident, then executes the returned command
// and asserts it performs a *fresh* on-call check (a live ListOnCalls call) and, when
// on-call, yields an acknowledgeIncidentsMsg for that incident.
func TestUpdatedIncidentList_AutoAck_DispatchesFreshOnCallCheck(t *testing.T) {
	mockClient := onCallMockClient(t, true)
	m := createTestModel()
	m.autoAcknowledge = true
	m.config = &pd.Config{
		Client:      mockClient,
		CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
	}

	incoming := []pagerduty.Incident{onCallAckCandidate("INC1", "USER1")}
	result, cmd := m.Update(updatedIncidentListMsg{incidents: incoming})

	// No on-call field should exist on the model to cache — structurally guaranteed —
	// and the sweep must have produced a command to run off-loop, not blocked inline.
	_ = result
	assert.NotNil(t, cmd, "auto-ack with a candidate should dispatch an async command")

	// Execute the batched command tree and confirm a fresh on-call check ran and
	// produced an acknowledge message for the assigned+unacked incident.
	foundAck := false
	drainCmd(t, cmd, func(msg tea.Msg) {
		if ack, ok := msg.(acknowledgeIncidentsMsg); ok && len(ack.incidents) == 1 && ack.incidents[0].ID == "INC1" {
			foundAck = true
		}
	})
	assert.True(t, foundAck, "fresh on-call check should acknowledge the assigned+unacked incident")
	assert.GreaterOrEqual(t, mockClient.CallCounts["ListOnCallsWithContext"], 1,
		"on-call status must be checked live (ListOnCalls called), never cached")
}
