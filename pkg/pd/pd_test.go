package pd

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
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

	memberIDs, err := GetTeamMemberIDs(mockClient, teams, opts)

	assert.NoError(t, err)
	assert.Len(t, memberIDs, 5)
	assert.Contains(t, memberIDs, "USER_A1")
	assert.Contains(t, memberIDs, "USER_A2")
	assert.Contains(t, memberIDs, "USER_B1")
	assert.Contains(t, memberIDs, "USER_B2")
	assert.Contains(t, memberIDs, "USER_B3")
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

	memberIDs, err := GetTeamMemberIDs(mockClient, teams, opts)

	assert.NoError(t, err)
	// With the bug, team B would start at offset 2 (after team A's two pages)
	// and would miss members. With the fix, all 3 members should be present.
	assert.Len(t, memberIDs, 3)
	assert.Contains(t, memberIDs, "USER_A1")
	assert.Contains(t, memberIDs, "USER_A2")
	assert.Contains(t, memberIDs, "USER_B1")
	// Verify the mock was called 3 times total
	assert.Equal(t, 3, mockClient.CallCounts["ListMembersWithContext"])
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

// Tests for newClient (returns non-nil RateLimitedClient)
func TestNewClient(t *testing.T) {
	client := newClient("test-token")
	assert.NotNil(t, client, "newClient should return a non-nil client")
}

func TestNewConfig_Success(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil)

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

	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must contain a `default` key")
}

func TestNewConfig_MissingSilentDefaultPolicy(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default": "POLICY1",
	}

	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil)

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

	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get escalation policy")
}

func TestNewConfig_WithIgnoredUsers(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, []string{"USER1", "USER2"})

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
	_, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, []string{"err"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user for ignore list")
}

func TestNewConfig_MultipleTeams(t *testing.T) {
	mockClient := &MockPagerDutyClient{}
	policies := map[string]string{
		"default":        "POLICY1",
		"silent_default": "POLICY2",
	}

	config, err := NewConfigWithClient(mockClient, []string{"team1", "team2", "team3"}, policies, nil)

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

	config, err := NewConfigWithClient(mockClient, []string{"team1"}, policies, nil)

	assert.NoError(t, err)
	// Keys should be uppercased
	assert.NotNil(t, config.EscalationPolicies["DEFAULT"])
	assert.NotNil(t, config.EscalationPolicies["SILENT_DEFAULT"])
	assert.NotNil(t, config.EscalationPolicies["CUSTOM_SERVICE"])
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
