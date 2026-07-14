package pd

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockManageIncidentsMoreClient is a specialized mock that returns More=true
// from ManageIncidentsWithContext, to test the unexpected pagination handling.
type MockManageIncidentsMoreClient struct {
	MockPagerDutyClient
}

func (m *MockManageIncidentsMoreClient) ManageIncidentsWithContext(ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	return &pagerduty.ListIncidentsResponse{
		APIListObject: pagerduty.APIListObject{
			More: true,
		},
		Incidents: []pagerduty.Incident{
			{
				APIObject: pagerduty.APIObject{
					ID: "QABCDEFG1234567",
				},
			},
		},
	}, nil
}

func TestContextWithTimeout_HasDeadline(t *testing.T) {
	ctx, cancel := contextWithTimeout()
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "context should have a deadline set")
	assert.WithinDuration(t, time.Now().Add(defaultAPITimeout), deadline, 2*time.Second,
		"deadline should be approximately defaultAPITimeout from now")
}

func TestContextWithTimeout_CancelWorks(t *testing.T) {
	ctx, cancel := contextWithTimeout()

	// Context should not be done before cancel
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done before cancel is called")
	default:
		// expected
	}

	cancel()

	// Context should be done after cancel
	select {
	case <-ctx.Done():
		// expected
	case <-time.After(1 * time.Second):
		t.Fatal("context should be done after cancel is called")
	}

	assert.ErrorIs(t, ctx.Err(), context.Canceled, "context error should be context.Canceled after cancel")
}

func TestGetTeams(t *testing.T) {
	t.Run("GetTeams", func(t *testing.T) {
		mockClient := new(MockPagerDutyClient)
		// Testing GetTeams MOCKS the GetTeamWithContext method, but appends the returned teams to a slice
		// so this is testing the local pd.GetTeams method, not the PagerDuty client.GetTeamWithContext method
		teams, err := GetTeams(mockClient, []string{"team1", "team2"})
		if !reflect.DeepEqual(teams, []*pagerduty.Team{{Name: "team1"}, {Name: "team2"}}) {
			t.Errorf("expected (%v), got (%v)", []string{"team1", "team2"}, teams)
		}
		assert.Len(t, teams, 2)
		assert.Equal(t, "team1", teams[0].Name)
		// HOW DOES THIS WORK
		// assert.Error(t, nil)

		if !errors.Is(err, nil) {
			t.Errorf("GetTeams() error = %v, wantErr %v", err, nil)
		}

		// mockClient := new(MockPagerDutyClient)
		// mockClient.On("GetTeamWithContext", mock.Anything, "team1").Return(&pagerduty.Team{Name: "team1"}, nil)
		// mockClient.On("GetTeamWithContext", mock.Anything, "team2").Return(nil, errors.New("error"))

		// teams, err := GetTeams(mockClient, []string{"team1", "team2"})
		// assert.Error(t, err)
		// assert.Len(t, teams, 1)
		// assert.Equal(t, "team1", teams[0].Name)
	})
}

func TestLoopManageIncidents_Success(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	ctx := context.Background()
	opts := []pagerduty.ManageIncidentsOptions{
		{ID: "INCIDENT1", Status: "acknowledged"},
	}

	incidents, err := loopManageIncidents(mockClient, ctx, "user@example.com", opts)

	assert.NoError(t, err)
	assert.Len(t, incidents, 2)
	assert.Equal(t, "QABCDEFG1234567", incidents[0].ID)
	assert.Equal(t, "QABCDEFG7654321", incidents[1].ID)
}

func TestLoopManageIncidents_APIError(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	ctx := context.Background()
	// The mock returns ErrMockError when any option has ID "err"
	opts := []pagerduty.ManageIncidentsOptions{
		{ID: "err"},
	}

	incidents, err := loopManageIncidents(mockClient, ctx, "user@example.com", opts)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMockError))
	assert.Empty(t, incidents)
}

func TestLoopManageIncidents_UnexpectedMore(t *testing.T) {
	mockClient := new(MockManageIncidentsMoreClient)
	ctx := context.Background()
	opts := []pagerduty.ManageIncidentsOptions{
		{ID: "INCIDENT1", Status: "acknowledged"},
	}

	// After the fix, this should return an error instead of panicking
	incidents, err := loopManageIncidents(mockClient, ctx, "user@example.com", opts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected pagination response")
	// Even though there's an error, the incidents from the response should still be nil
	assert.Nil(t, incidents)
}

func TestGetTeamMemberIDs_MultipleTeams(t *testing.T) {
	// Test that GetTeamMemberIDs collects members from multiple teams
	// without duplication or loss
	mockClient := &MockPagerDutyClient{
		ListMembersResponses: []ListMembersResponse{
			// Response for team A (single page)
			{Response: &pagerduty.ListTeamMembersResponse{
				APIListObject: pagerduty.APIListObject{More: false},
				Members: []pagerduty.Member{
					{User: pagerduty.APIObject{ID: "USER_A1"}},
					{User: pagerduty.APIObject{ID: "USER_A2"}},
				},
			}},
			// Response for team B (single page)
			{Response: &pagerduty.ListTeamMembersResponse{
				APIListObject: pagerduty.APIListObject{More: false},
				Members: []pagerduty.Member{
					{User: pagerduty.APIObject{ID: "USER_B1"}},
					{User: pagerduty.APIObject{ID: "USER_B2"}},
					{User: pagerduty.APIObject{ID: "USER_B3"}},
				},
			}},
		},
	}

	teams := []*pagerduty.Team{
		{APIObject: pagerduty.APIObject{ID: "TEAM_A"}},
		{APIObject: pagerduty.APIObject{ID: "TEAM_B"}},
	}
	opts := pagerduty.ListTeamMembersOptions{Limit: 100, Offset: 0}

	memberIDs, byTeam, err := GetTeamMemberIDs(mockClient, teams, opts)

	assert.NoError(t, err)
	assert.Len(t, memberIDs, 5)
	assert.Contains(t, memberIDs, "USER_A1")
	assert.Contains(t, memberIDs, "USER_A2")
	assert.Contains(t, memberIDs, "USER_B1")
	assert.Contains(t, memberIDs, "USER_B2")
	assert.Contains(t, memberIDs, "USER_B3")
	assert.Len(t, byTeam["TEAM_A"], 2)
	assert.Len(t, byTeam["TEAM_B"], 3)
}

func TestGetTeamMemberIDs_PaginatedTeam(t *testing.T) {
	// Test that pagination within a single team collects all pages,
	// and that pagination offset is reset between teams so the second
	// team also starts at offset 0.
	mockClient := &MockPagerDutyClient{
		ListMembersResponses: []ListMembersResponse{
			// Team A, page 1 (more pages follow)
			{Response: &pagerduty.ListTeamMembersResponse{
				APIListObject: pagerduty.APIListObject{More: true},
				Members: []pagerduty.Member{
					{User: pagerduty.APIObject{ID: "USER_A1"}},
				},
			}},
			// Team A, page 2 (last page)
			{Response: &pagerduty.ListTeamMembersResponse{
				APIListObject: pagerduty.APIListObject{More: false},
				Members: []pagerduty.Member{
					{User: pagerduty.APIObject{ID: "USER_A2"}},
				},
			}},
			// Team B, page 1 (only page) - this will only be reached
			// if the offset is properly reset for team B
			{Response: &pagerduty.ListTeamMembersResponse{
				APIListObject: pagerduty.APIListObject{More: false},
				Members: []pagerduty.Member{
					{User: pagerduty.APIObject{ID: "USER_B1"}},
				},
			}},
		},
	}

	teams := []*pagerduty.Team{
		{APIObject: pagerduty.APIObject{ID: "TEAM_A"}},
		{APIObject: pagerduty.APIObject{ID: "TEAM_B"}},
	}
	opts := pagerduty.ListTeamMembersOptions{Limit: 1, Offset: 0}

	memberIDs, byTeam, err := GetTeamMemberIDs(mockClient, teams, opts)

	assert.NoError(t, err)
	assert.Len(t, memberIDs, 3)
	assert.Contains(t, memberIDs, "USER_A1")
	assert.Contains(t, memberIDs, "USER_A2")
	assert.Contains(t, memberIDs, "USER_B1")
	assert.Equal(t, 3, mockClient.CallCounts["ListMembersWithContext"])
	assert.Len(t, byTeam["TEAM_A"], 2)
	assert.Len(t, byTeam["TEAM_B"], 1)
	// Verify offsets: team A page 1 at 0, team A page 2 at 1, team B page 1 at 0 (reset)
	assert.Equal(t, []uint{0, 1, 0}, mockClient.ListMembersOffsets,
		"offset should reset to 0 for each new team")
}

func TestPostNote(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1", Type: "user_reference"},
	}

	tests := []struct {
		name        string
		id          string
		content     string
		expectErr   bool
		errContains string
	}{
		{
			name:      "successfully creates a note",
			id:        "INCIDENT1",
			content:   "Test note content",
			expectErr: false,
		},
		{
			name:        "returns wrapped error on failure",
			id:          "err",
			content:     "Test note content",
			expectErr:   true,
			errContains: "pd.PostNote()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note, err := PostNote(mockClient, tt.id, user, tt.content)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains,
					"error should be wrapped with function context")
				assert.Contains(t, err.Error(), tt.id,
					"error should include the incident ID")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, note)
				assert.Equal(t, tt.content, note.Content)
			}
		})
	}
}

func TestGetAlerts_Success(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.ListIncidentAlertsOptions{}

	alerts, err := GetAlerts(mockClient, "INCIDENT1", opts)

	assert.NoError(t, err)
	assert.Len(t, alerts, 2)
	assert.Equal(t, "QABCDEFG1234567", alerts[0].ID)
	assert.Equal(t, "QABCDEFG7654321", alerts[1].ID)
}

func TestGetAlerts_Error(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.ListIncidentAlertsOptions{}

	alerts, err := GetAlerts(mockClient, "err", opts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetAlerts()")
	assert.Empty(t, alerts)
}

func TestGetEscalationPolicy_Success(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.GetEscalationPolicyOptions{}

	policy, err := GetEscalationPolicy(mockClient, "POLICY1", opts)

	assert.NoError(t, err)
	assert.NotNil(t, policy)
	assert.Equal(t, "POLICY1", policy.ID)
}

func TestGetEscalationPolicy_Error(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.GetEscalationPolicyOptions{}

	policy, err := GetEscalationPolicy(mockClient, "err", opts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetEscalationPolicy()")
	assert.Nil(t, policy)
}

func TestGetIncident_Success(t *testing.T) {
	mockClient := new(MockPagerDutyClient)

	incident, err := GetIncident(mockClient, "INCIDENT1")

	assert.NoError(t, err)
	assert.NotNil(t, incident)
	assert.Equal(t, "INCIDENT1", incident.ID)
}

func TestGetIncident_Error(t *testing.T) {
	mockClient := new(MockPagerDutyClient)

	_, err := GetIncident(mockClient, "err")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetIncident()")
}

func TestGetIncidents_Success(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.ListIncidentsOptions{}

	incidents, err := GetIncidents(mockClient, opts)

	assert.NoError(t, err)
	assert.Len(t, incidents, 2)
	assert.Equal(t, "QABCDEFG1234567", incidents[0].ID)
	assert.Equal(t, "QABCDEFG7654321", incidents[1].ID)
}

func TestGetIncidents_Error(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.ListIncidentsOptions{
		UserIDs: []string{"err"},
	}

	incidents, err := GetIncidents(mockClient, opts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetIncidents()")
	assert.Empty(t, incidents)
}

// TestGetIncidents_DefaultsLimitAndAdvancesOffset reproduces the v1.5.0
// startup timeout: with Limit omitted the API pages at 25 results, and
// `opts.Offset += opts.Limit` advanced the offset by zero, so a more=true
// response refetched page one forever until the context deadline expired.
// GetIncidents must apply defaultPageLimit when Limit is unset and advance
// the offset between pages.
func TestGetIncidents_DefaultsLimitAndAdvancesOffset(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				APIListObject: pagerduty.APIListObject{More: true},
				Incidents:     []pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "PAGE1INCIDENT"}}},
			},
			{
				Incidents: []pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "PAGE2INCIDENT"}}},
			},
		},
	}

	incidents, err := GetIncidents(mockClient, pagerduty.ListIncidentsOptions{})

	assert.NoError(t, err)
	assert.Len(t, incidents, 2)
	if assert.Len(t, mockClient.RecordedListIncidentsOpts, 2) {
		assert.Equal(t, uint(defaultPageLimit), mockClient.RecordedListIncidentsOpts[0].Limit)
		assert.Equal(t, uint(defaultPageLimit), mockClient.RecordedListIncidentsOpts[1].Offset)
	}
}

// TestGetAlerts_DefaultsLimitWhenZero guards against the same
// zero-Limit infinite pagination loop in GetAlerts.
func TestGetAlerts_DefaultsLimitWhenZero(t *testing.T) {
	mockClient := new(MockPagerDutyClient)

	_, err := GetAlerts(mockClient, "INCIDENT1", pagerduty.ListIncidentAlertsOptions{})

	assert.NoError(t, err)
	if assert.Len(t, mockClient.RecordedListAlertsOpts, 1) {
		assert.Equal(t, uint(defaultPageLimit), mockClient.RecordedListAlertsOpts[0].Limit)
	}
}

// TestGetUserOnCalls_DefaultsLimitWhenZero guards against the same
// zero-Limit infinite pagination loop in GetUserOnCalls.
func TestGetUserOnCalls_DefaultsLimitWhenZero(t *testing.T) {
	mockClient := new(MockPagerDutyClient)

	_, err := GetUserOnCalls(mockClient, "USER1", pagerduty.ListOnCallOptions{})

	assert.NoError(t, err)
	if assert.Len(t, mockClient.RecordedListOnCallOpts, 1) {
		assert.Equal(t, uint(defaultPageLimit), mockClient.RecordedListOnCallOpts[0].Limit)
	}
}

// TestGetTeamMemberIDs_DefaultsLimitWhenZero guards against the same
// zero-Limit infinite pagination loop in GetTeamMemberIDs: with a more=true
// first page and no explicit Limit, the second request must advance to
// offset defaultPageLimit rather than refetching offset 0.
func TestGetTeamMemberIDs_DefaultsLimitWhenZero(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		ListMembersResponses: []ListMembersResponse{
			{Response: &pagerduty.ListTeamMembersResponse{
				APIListObject: pagerduty.APIListObject{More: true},
				Members:       []pagerduty.Member{{User: pagerduty.APIObject{ID: "U1"}}},
			}},
			{Response: &pagerduty.ListTeamMembersResponse{
				Members: []pagerduty.Member{{User: pagerduty.APIObject{ID: "U2"}}},
			}},
		},
	}
	teams := []*pagerduty.Team{{APIObject: pagerduty.APIObject{ID: "TEAM1"}}}

	ids, _, err := GetTeamMemberIDs(mockClient, teams, pagerduty.ListTeamMembersOptions{})

	assert.NoError(t, err)
	assert.Equal(t, []string{"U1", "U2"}, ids)
	assert.Equal(t, []uint{0, uint(defaultPageLimit)}, mockClient.ListMembersOffsets)
}

func TestGetNotes_Success(t *testing.T) {
	mockClient := new(MockPagerDutyClient)

	notes, err := GetNotes(mockClient, "INCIDENT1")

	assert.NoError(t, err)
	assert.Len(t, notes, 2)
	assert.Equal(t, "QABCDEFG1234567", notes[0].ID)
	assert.Equal(t, "QABCDEFG7654321", notes[1].ID)
}

func TestGetNotes_Error(t *testing.T) {
	mockClient := new(MockPagerDutyClient)

	notes, err := GetNotes(mockClient, "err")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetNotes()")
	assert.Empty(t, notes)
}

func TestGetUser_Success(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.GetUserOptions{}

	user, err := GetUser(mockClient, "USER1", opts)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "USER1", user.ID)
}

func TestGetUser_Error(t *testing.T) {
	mockClient := new(MockPagerDutyClient)
	opts := pagerduty.GetUserOptions{}

	user, err := GetUser(mockClient, "err", opts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetUser()")
	assert.Nil(t, user)
}

// Tests for NewClient (returns non-nil RateLimitedClient)
func TestNewClient(t *testing.T) {
	client := NewClient("test-token")
	assert.NotNil(t, client, "NewClient should return a non-nil client")
}

func TestGetCurrentUserTeams_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}

	teams, err := GetCurrentUserTeams(mockClient)

	assert.NoError(t, err)
	assert.Len(t, teams, 2)
	assert.Equal(t, "TEAM_001", teams[0].ID)
	assert.Equal(t, "Mock Team Alpha", teams[0].Name)
	assert.Equal(t, "TEAM_002", teams[1].ID)
	assert.Equal(t, "Mock Team Beta", teams[1].Name)
}

func TestGetCurrentUserTeams_Error(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		GetCurrentUserErr: ErrMockError,
	}

	teams, err := GetCurrentUserTeams(mockClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetCurrentUserTeams()")
	assert.Nil(t, teams)
}

func TestGetCurrentUser_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}

	user, err := GetCurrentUser(mockClient)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "MOCK_USER", user.ID)
	assert.Equal(t, "mock@example.com", user.Email)
	// The wrapper must apply a timeout context, so it routes through
	// GetCurrentUserWithContext (not a bare Background call).
	assert.GreaterOrEqual(t, mockClient.CallCounts["GetCurrentUserWithContext"], 1)
}

func TestGetCurrentUser_Error(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		GetCurrentUserErr: ErrMockError,
	}

	user, err := GetCurrentUser(mockClient)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.GetCurrentUser()")
	assert.Nil(t, user)
}

func TestNewConfig_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil, "", nil)

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.NotNil(t, config.CurrentUser)
	assert.Equal(t, "MOCK_USER", config.CurrentUser.ID)
	assert.Len(t, config.Teams, 1)
	assert.Equal(t, "team1", config.Teams[0].Name)
	assert.NotEmpty(t, config.TeamsMemberIDs)
	assert.Len(t, config.EscalationPolicies, 2)
	assert.NotNil(t, config.EscalationPolicies["DEFAULT"])
	assert.NotNil(t, config.EscalationPolicies["SILENT_DEFAULT"])
}

func TestNewConfig_MissingDefaultPolicy(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"silent_default": "POLICY2",
	}

	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil, "", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must contain a `default` key")
}

func TestNewConfig_MissingSilentDefaultPolicy(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default": "POLICY1",
	}

	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil, "", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must contain a `silent_default` key")
}

func TestNewConfig_BadEscalationPolicy(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	// "err" ID triggers mock error
	policies := map[string]string{
		"default":        "err",
		"silent_default": "POLICY2",
	}

	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil, "", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get escalation policy")
}

func TestNewConfig_WithIgnoredUsers(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, []string{"USER1", "USER2"}, "", nil)

	assert.NoError(t, err)
	assert.Len(t, config.IgnoredUsers, 2)
	assert.Equal(t, "USER1", config.IgnoredUsers[0].ID)
	assert.Equal(t, "USER2", config.IgnoredUsers[1].ID)
}

func TestNewConfig_BadIgnoredUser(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	// "err" ID triggers mock error in GetUserWithContext
	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, []string{"err"}, "", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user for ignore list")
}

func TestNewConfig_MultipleTeams(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1", "team2", "team3"}, policies, nil, "", nil)

	assert.NoError(t, err)
	assert.Len(t, config.Teams, 3)
}

func TestNewConfig_PolicyKeysUppercased(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
		"custom_service": "POLICY3",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil, "", nil)

	assert.NoError(t, err)
	// Keys should be uppercased
	assert.NotNil(t, config.EscalationPolicies["DEFAULT"])
	assert.NotNil(t, config.EscalationPolicies["SILENT_DEFAULT"])
	assert.NotNil(t, config.EscalationPolicies["CUSTOM_SERVICE"])
}

func TestNewConfig_AutoDiscoverIgnoredUsers(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		EscalationPolicyResponses: map[string]*pagerduty.EscalationPolicy{
			"POLICY_REAL": {
				APIObject: pagerduty.APIObject{ID: "POLICY_REAL"},
				Name:      "OpenShift Escalation Policy",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "NOBODY_SREP", Type: "user_reference"},
					}},
					{Targets: []pagerduty.APIObject{
						{ID: "ONCALL_SCHED", Type: "schedule_reference"},
					}},
				},
			},
			"POLICY_SILENT": {
				APIObject: pagerduty.APIObject{ID: "POLICY_SILENT"},
				Name:      "Silent Test - Non-Actionable",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT_USER_1", Type: "user_reference"},
					}},
					{Targets: []pagerduty.APIObject{
						{ID: "BOT_USER_2", Type: "user_reference"},
					}},
				},
			},
		},
	}
	policies := map[string]string{
		"default":        "POLICY_REAL",
		"silent_default": "POLICY_SILENT",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil, "", nil)

	assert.NoError(t, err)
	assert.Len(t, config.IgnoredUsers, 2)
	assert.Equal(t, "BOT_USER_1", config.IgnoredUsers[0].ID)
	assert.Equal(t, "BOT_USER_2", config.IgnoredUsers[1].ID)
}

func TestNewConfig_ManualIgnoredUsersTakePrecedence(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		EscalationPolicyResponses: map[string]*pagerduty.EscalationPolicy{
			"POLICY_REAL": {
				APIObject: pagerduty.APIObject{ID: "POLICY_REAL"},
				Name:      "OpenShift Escalation Policy",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "ONCALL_SCHED", Type: "schedule_reference"},
					}},
				},
			},
			"POLICY_SILENT": {
				APIObject: pagerduty.APIObject{ID: "POLICY_SILENT"},
				Name:      "Silent Test",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT_USER_1", Type: "user_reference"},
					}},
				},
			},
		},
	}
	policies := map[string]string{
		"default":        "POLICY_REAL",
		"silent_default": "POLICY_SILENT",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, []string{"MANUAL_USER"}, "", nil)

	assert.NoError(t, err)
	assert.Len(t, config.IgnoredUsers, 1)
	assert.Equal(t, "MANUAL_USER", config.IgnoredUsers[0].ID)
}

func TestNewConfig_AutoDiscoverNoSilentPolicies(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		EscalationPolicyResponses: map[string]*pagerduty.EscalationPolicy{
			"POLICY1": {
				APIObject: pagerduty.APIObject{ID: "POLICY1"},
				Name:      "Real Policy 1",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "SCHED1", Type: "schedule_reference"},
					}},
				},
			},
			"POLICY2": {
				APIObject: pagerduty.APIObject{ID: "POLICY2"},
				Name:      "Real Policy 2",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "SCHED2", Type: "schedule_reference"},
					}},
				},
			},
		},
	}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil, "", nil)

	assert.NoError(t, err)
	assert.Empty(t, config.IgnoredUsers)
}

func TestNewConfig_DefaultSilentPolicy(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		EscalationPolicyResponses: map[string]*pagerduty.EscalationPolicy{
			"SILENT_POL": {
				APIObject: pagerduty.APIObject{ID: "SILENT_POL"},
				Name:      "Silent Test - Non-Actionable",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT1", Type: "user_reference"},
					}},
				},
			},
		},
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, nil, nil, "SILENT_POL", nil)

	assert.NoError(t, err)
	assert.NotNil(t, config.EscalationPolicies["SILENT_DEFAULT"])
	assert.Equal(t, "SILENT_POL", config.EscalationPolicies["SILENT_DEFAULT"].ID)
	assert.Len(t, config.IgnoredUsers, 1)
	assert.Equal(t, "BOT1", config.IgnoredUsers[0].ID)
}

func TestNewConfig_DefaultSilentWithCustomOverrides(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		EscalationPolicyResponses: map[string]*pagerduty.EscalationPolicy{
			"SILENT_POL": {
				APIObject: pagerduty.APIObject{ID: "SILENT_POL"},
				Name:      "Silent Test",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT1", Type: "user_reference"},
					}},
				},
			},
			"DMS_SILENT": {
				APIObject: pagerduty.APIObject{ID: "DMS_SILENT"},
				Name:      "DMS Silent Test",
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT2", Type: "user_reference"},
					}},
				},
			},
		},
	}

	customOverrides := map[string]string{
		"svc_dms": "DMS_SILENT",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, nil, nil, "SILENT_POL", customOverrides)

	assert.NoError(t, err)
	assert.NotNil(t, config.EscalationPolicies["SILENT_DEFAULT"])
	assert.Equal(t, "SILENT_POL", config.EscalationPolicies["SILENT_DEFAULT"].ID)
	assert.NotNil(t, config.EscalationPolicies["SVC_DMS"])
	assert.Equal(t, "DMS_SILENT", config.EscalationPolicies["SVC_DMS"].ID)
	// Both bot users should be auto-discovered
	assert.Len(t, config.IgnoredUsers, 2)
}

func TestNewConfig_NoSilentPolicyConfigured(t *testing.T) {
	mockClient := &MockPagerDutyClient{}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, nil, nil, "", nil)

	assert.NoError(t, err)
	assert.Empty(t, config.EscalationPolicies)
	assert.Empty(t, config.IgnoredUsers)
}

func TestAcknowledgeIncident_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1", Type: "user_reference"},
		Email:     "user@example.com",
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
		{APIObject: pagerduty.APIObject{ID: "INCIDENT2"}},
	}

	result, err := AcknowledgeIncident(mockClient, incidents, user, user)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestAcknowledgeIncident_Error(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	// "err" ID triggers mock error
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "err"}},
	}

	result, err := AcknowledgeIncident(mockClient, incidents, user, user)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAcknowledgeIncident_UnAcknowledge(t *testing.T) {
	// When user is nil, it should un-acknowledge (set escalation level only)
	mockClient := &MockPagerDutyClient{}
	currentUser := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}

	result, err := AcknowledgeIncident(mockClient, incidents, nil, currentUser)

	assert.NoError(t, err)
	assert.Len(t, result, 2) // Mock always returns 2 incidents
}

func TestAcknowledgeIncident_NilCurrentUser(t *testing.T) {
	// When currentUser is nil, email should be empty string
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}

	result, err := AcknowledgeIncident(mockClient, incidents, user, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestReassignIncidents_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
		{APIObject: pagerduty.APIObject{ID: "INCIDENT2"}},
	}
	assignees := []*pagerduty.User{
		{APIObject: pagerduty.APIObject{ID: "USER2"}},
		{APIObject: pagerduty.APIObject{ID: "USER3"}},
	}

	result, err := ReassignIncidents(mockClient, incidents, user, assignees)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestReassignIncidents_NilUser(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	incidents := []pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}}}

	// A nil "from" user must return an error, not panic on user.Email.
	result, err := ReassignIncidents(mockClient, incidents, nil, []*pagerduty.User{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.ReassignIncidents()")
	assert.Nil(t, result)
}

func TestReEscalateIncidents_NilUser(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	incidents := []pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}}}
	policy := &pagerduty.EscalationPolicy{APIObject: pagerduty.APIObject{ID: "POL1"}}

	result, err := ReEscalateIncidents(mockClient, incidents, nil, policy, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pd.ReEscalateIncidents()")
	assert.Nil(t, result)
}

func TestReassignIncidents_EmptyIncidents(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}

	// Empty incidents should still call ManageIncidents with empty opts
	result, err := ReassignIncidents(mockClient, []pagerduty.Incident{}, user, []*pagerduty.User{})

	assert.NoError(t, err)
	assert.Len(t, result, 2) // Mock always returns 2
}

func TestReassignIncidents_EmptyIncidentID(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	// Incident with empty ID should trigger error
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: ""}},
	}

	_, err := ReassignIncidents(mockClient, incidents, user, []*pagerduty.User{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incident is nil")
}

func TestReassignIncidents_Error(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	// "err" ID triggers mock error
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "err"}},
	}

	result, err := ReassignIncidents(mockClient, incidents, user, []*pagerduty.User{})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReEscalateIncidents_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	result, err := ReEscalateIncidents(mockClient, incidents, user, policy, 1)

	assert.NoError(t, err)
	assert.Len(t, result, 2) // Mock returns 2
}

func TestReEscalateIncidents_SamePolicy_OmitsPolicySendsLevelOnly(t *testing.T) {
	// Re-escalating in place: the target policy is the incident's CURRENT policy.
	// PagerDuty restarts escalation at level 1 whenever escalation_policy is set, so
	// re-sending the same policy would override the requested level (dropping the
	// incident back to level 1 = "Nobody"). To jump to level 2 in place we must send
	// escalation_level ONLY and OMIT escalation_policy.
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}, Email: "user@example.com"}
	incidents := []pagerduty.Incident{
		{
			APIObject:        pagerduty.APIObject{ID: "INCIDENT1"},
			EscalationPolicy: pagerduty.APIObject{ID: "POLICY1"},
		},
	}
	policy := &pagerduty.EscalationPolicy{APIObject: pagerduty.APIObject{ID: "POLICY1"}}

	_, err := ReEscalateIncidents(mockClient, incidents, user, policy, 2)

	assert.NoError(t, err)
	require.Len(t, mockClient.LastManageIncidentsOpts, 1)
	opt := mockClient.LastManageIncidentsOpts[0]
	assert.Equal(t, uint(2), opt.EscalationLevel, "should send the requested escalation level")
	assert.Nil(t, opt.EscalationPolicy,
		"must omit escalation_policy for in-place re-escalation, or PD resets to level 1")
}

func TestReEscalateIncidents_DifferentPolicy_SendsPolicy(t *testing.T) {
	// Moving to a DIFFERENT policy (e.g. silencing to a silent policy) must still send
	// escalation_policy — that is the whole point of the move.
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}, Email: "user@example.com"}
	incidents := []pagerduty.Incident{
		{
			APIObject:        pagerduty.APIObject{ID: "INCIDENT1"},
			EscalationPolicy: pagerduty.APIObject{ID: "POLICY1"},
		},
	}
	silentPolicy := &pagerduty.EscalationPolicy{APIObject: pagerduty.APIObject{ID: "SILENT_POLICY"}}

	_, err := ReEscalateIncidents(mockClient, incidents, user, silentPolicy, 1)

	assert.NoError(t, err)
	require.Len(t, mockClient.LastManageIncidentsOpts, 1)
	opt := mockClient.LastManageIncidentsOpts[0]
	require.NotNil(t, opt.EscalationPolicy, "must send escalation_policy when moving to a different policy")
	assert.Equal(t, "SILENT_POLICY", opt.EscalationPolicy.ID)
}

func TestReEscalateIncidents_EmptyIncidentID(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: ""}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	_, err := ReEscalateIncidents(mockClient, incidents, user, policy, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incident is nil")
}

func TestReEscalateIncidents_Error(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	// "err" triggers mock error
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "err"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	result, err := ReEscalateIncidents(mockClient, incidents, user, policy, 1)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReEscalateIncidents_MultipleIncidents(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
		{APIObject: pagerduty.APIObject{ID: "INCIDENT2"}},
		{APIObject: pagerduty.APIObject{ID: "INCIDENT3"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	result, err := ReEscalateIncidents(mockClient, incidents, user, policy, 2)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestReEscalateIncidents_LevelZero(t *testing.T) {
	// Level 0 is technically valid to pass to the API; the function does not validate it
	mockClient := &MockPagerDutyClient{}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "INCIDENT1"}},
	}
	policy := &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: "POLICY1"},
	}

	result, err := ReEscalateIncidents(mockClient, incidents, user, policy, 0)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewListIncidentOptsFromDefaults(t *testing.T) {
	opts := NewListIncidentOptsFromDefaults()

	assert.Equal(t, uint(defaultPageLimit), opts.Limit)
	assert.Equal(t, uint(defaultOffset), opts.Offset)
	assert.Equal(t, defaultIncidentStatuses, opts.Statuses)
}

func TestGetUserOnCalls_MultiplePages(t *testing.T) {
	// Test that GetUserOnCalls appends results across pages instead of
	// overwriting them.
	mockClient := &MockPagerDutyClient{
		ListOnCallsResponses: []pagerduty.ListOnCallsResponse{
			// Page 1 (more pages follow)
			{
				APIListObject: pagerduty.APIListObject{More: true},
				OnCalls: []pagerduty.OnCall{
					{
						User:            pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
						EscalationLevel: 1,
					},
					{
						User:            pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER2"}},
						EscalationLevel: 1,
					},
				},
			},
			// Page 2 (last page)
			{
				APIListObject: pagerduty.APIListObject{More: false},
				OnCalls: []pagerduty.OnCall{
					{
						User:            pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER3"}},
						EscalationLevel: 2,
					},
				},
			},
		},
	}

	opts := pagerduty.ListOnCallOptions{Limit: 2, Offset: 0}

	onCalls, err := GetUserOnCalls(mockClient, "USER1", opts)

	assert.NoError(t, err)
	// With the bug (o = response.OnCalls), only the last page (1 entry) would be kept.
	// With the fix (o = append(o, response.OnCalls...)), all 3 entries should be present.
	assert.Len(t, onCalls, 3)
	assert.Equal(t, "USER1", onCalls[0].User.ID)
	assert.Equal(t, "USER2", onCalls[1].User.ID)
	assert.Equal(t, "USER3", onCalls[2].User.ID)
	// Verify the mock was called 2 times
	assert.Equal(t, 2, mockClient.CallCounts["ListOnCallsWithContext"])
}

func TestClassifyEscalationPolicy(t *testing.T) {
	tests := []struct {
		name     string
		policy   *pagerduty.EscalationPolicy
		expected string
	}{
		{
			name: "policy with schedule_reference is REAL",
			policy: &pagerduty.EscalationPolicy{
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "SCHED1", Type: "schedule_reference"},
					}},
				},
			},
			expected: PolicyClassReal,
		},
		{
			name: "policy with schedule_reference in later rule is REAL",
			policy: &pagerduty.EscalationPolicy{
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT_USER", Type: "user_reference"},
					}},
					{Targets: []pagerduty.APIObject{
						{ID: "ONCALL_SCHED", Type: "schedule_reference"},
					}},
				},
			},
			expected: PolicyClassReal,
		},
		{
			name: "policy with mixed targets in one rule is REAL",
			policy: &pagerduty.EscalationPolicy{
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT_USER", Type: "user_reference"},
						{ID: "SCHED1", Type: "schedule_reference"},
					}},
				},
			},
			expected: PolicyClassReal,
		},
		{
			name: "policy with only user_reference targets is SILENT",
			policy: &pagerduty.EscalationPolicy{
				EscalationRules: []pagerduty.EscalationRule{
					{Targets: []pagerduty.APIObject{
						{ID: "BOT_USER_1", Type: "user_reference"},
					}},
					{Targets: []pagerduty.APIObject{
						{ID: "BOT_USER_2", Type: "user_reference"},
					}},
				},
			},
			expected: PolicyClassSilent,
		},
		{
			name: "policy with empty escalation rules is SILENT",
			policy: &pagerduty.EscalationPolicy{
				EscalationRules: []pagerduty.EscalationRule{},
			},
			expected: PolicyClassSilent,
		},
		{
			name:     "nil policy is SILENT",
			policy:   nil,
			expected: PolicyClassSilent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyEscalationPolicy(tt.policy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSilentPolicyUsers(t *testing.T) {
	silentPolicy := &pagerduty.EscalationPolicy{
		EscalationRules: []pagerduty.EscalationRule{
			{Targets: []pagerduty.APIObject{
				{ID: "BOT_USER_1", Type: "user_reference"},
			}},
			{Targets: []pagerduty.APIObject{
				{ID: "BOT_USER_2", Type: "user_reference"},
			}},
		},
	}
	realPolicy := &pagerduty.EscalationPolicy{
		EscalationRules: []pagerduty.EscalationRule{
			{Targets: []pagerduty.APIObject{
				{ID: "NOBODY_SREP", Type: "user_reference"},
			}},
			{Targets: []pagerduty.APIObject{
				{ID: "ONCALL_SCHED", Type: "schedule_reference"},
			}},
		},
	}

	tests := []struct {
		name     string
		policies map[string]*pagerduty.EscalationPolicy
		expected []string
	}{
		{
			name: "extracts users from SILENT policy",
			policies: map[string]*pagerduty.EscalationPolicy{
				"SILENT_DEFAULT": silentPolicy,
			},
			expected: []string{"BOT_USER_1", "BOT_USER_2"},
		},
		{
			name: "skips REAL policy users",
			policies: map[string]*pagerduty.EscalationPolicy{
				"DEFAULT":        realPolicy,
				"SILENT_DEFAULT": silentPolicy,
			},
			expected: []string{"BOT_USER_1", "BOT_USER_2"},
		},
		{
			name: "all REAL policies returns empty",
			policies: map[string]*pagerduty.EscalationPolicy{
				"DEFAULT": realPolicy,
			},
			expected: nil,
		},
		{
			name: "deduplicates across rules",
			policies: map[string]*pagerduty.EscalationPolicy{
				"SILENT_DEFAULT": {
					EscalationRules: []pagerduty.EscalationRule{
						{Targets: []pagerduty.APIObject{
							{ID: "BOT_USER_1", Type: "user_reference"},
						}},
						{Targets: []pagerduty.APIObject{
							{ID: "BOT_USER_1", Type: "user_reference"},
						}},
					},
				},
			},
			expected: []string{"BOT_USER_1"},
		},
		{
			name: "union across multiple SILENT policies",
			policies: map[string]*pagerduty.EscalationPolicy{
				"SILENT_DEFAULT": silentPolicy,
				"DMS_SILENT": {
					EscalationRules: []pagerduty.EscalationRule{
						{Targets: []pagerduty.APIObject{
							{ID: "BOT_USER_2", Type: "user_reference"},
							{ID: "BOT_USER_3", Type: "user_reference"},
						}},
					},
				},
			},
			expected: []string{"BOT_USER_1", "BOT_USER_2", "BOT_USER_3"},
		},
		{
			name:     "empty map returns nil",
			policies: map[string]*pagerduty.EscalationPolicy{},
			expected: nil,
		},
		{
			name:     "nil map returns nil",
			policies: nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSilentPolicyUsers(tt.policies)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetTeamEscalationPolicies_SinglePage(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		ListEscalationPoliciesResponses: []pagerduty.ListEscalationPoliciesResponse{
			{
				APIListObject: pagerduty.APIListObject{More: false},
				EscalationPolicies: []pagerduty.EscalationPolicy{
					{APIObject: pagerduty.APIObject{ID: "POL1"}, Name: "Policy 1"},
					{APIObject: pagerduty.APIObject{ID: "POL2"}, Name: "Policy 2"},
				},
			},
		},
	}

	policies, err := GetTeamEscalationPolicies(mockClient, []string{"TEAM1"})

	assert.NoError(t, err)
	assert.Len(t, policies, 2)
	assert.Equal(t, "POL1", policies[0].ID)
	assert.Equal(t, "POL2", policies[1].ID)
}

func TestGetTeamEscalationPolicies_Paginated(t *testing.T) {
	mockClient := &MockPagerDutyClient{
		ListEscalationPoliciesResponses: []pagerduty.ListEscalationPoliciesResponse{
			{
				APIListObject: pagerduty.APIListObject{More: true},
				EscalationPolicies: []pagerduty.EscalationPolicy{
					{APIObject: pagerduty.APIObject{ID: "POL1"}},
				},
			},
			{
				APIListObject: pagerduty.APIListObject{More: false},
				EscalationPolicies: []pagerduty.EscalationPolicy{
					{APIObject: pagerduty.APIObject{ID: "POL2"}},
				},
			},
		},
	}

	policies, err := GetTeamEscalationPolicies(mockClient, []string{"TEAM1"})

	assert.NoError(t, err)
	assert.Len(t, policies, 2)
	assert.Equal(t, 2, mockClient.CallCounts["ListEscalationPoliciesWithContext"])
}

func TestGetTeamEscalationPolicies_EmptyTeams(t *testing.T) {
	mockClient := &MockPagerDutyClient{}

	policies, err := GetTeamEscalationPolicies(mockClient, []string{})

	assert.NoError(t, err)
	assert.Empty(t, policies)
}

func TestUpdateIncidentTitle_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	currentUser := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}

	result, err := UpdateIncidentTitle(mockClient, "INCIDENT1", "[HCP] SomeAlert", currentUser)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	require.NotNil(t, mockClient.LastManageIncidentsOpts)
	assert.Len(t, mockClient.LastManageIncidentsOpts, 1)
	assert.Equal(t, "INCIDENT1", mockClient.LastManageIncidentsOpts[0].ID)
	assert.Equal(t, "[HCP] SomeAlert", mockClient.LastManageIncidentsOpts[0].Title)
}

func TestUpdateIncidentTitle_Error(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	currentUser := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}

	result, err := UpdateIncidentTitle(mockClient, "err", "[HCP] SomeAlert", currentUser)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateIncidentTitle_NilUser(t *testing.T) {
	mockClient := &MockPagerDutyClient{}

	result, err := UpdateIncidentTitle(mockClient, "INCIDENT1", "[HCP] SomeAlert", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateIncidentTitle_EmptyTitle(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	currentUser := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}

	result, err := UpdateIncidentTitle(mockClient, "INCIDENT1", "", currentUser)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateIncidentTitle_EmptyIncidentID(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	currentUser := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "USER1"},
		Email:     "user@example.com",
	}

	result, err := UpdateIncidentTitle(mockClient, "", "[HCP] SomeAlert", currentUser)

	assert.Error(t, err)
	assert.Nil(t, result)
}
