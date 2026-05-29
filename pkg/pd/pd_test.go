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
