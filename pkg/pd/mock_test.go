package pd

import (
	"context"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
)

func TestEnhancedMock_BackwardCompatible(t *testing.T) {
	// Creating a mock the old way (zero-value struct) must still behave identically
	// to the current mock behavior: "err" ID triggers ErrMockError, normal IDs return
	// fixed responses.
	mock := &MockPagerDutyClient{}
	ctx := context.Background()

	t.Run("GetIncidentWithContext returns incident for normal ID", func(t *testing.T) {
		incident, err := mock.GetIncidentWithContext(ctx, "P12345")
		assert.NoError(t, err)
		assert.Equal(t, "P12345", incident.ID)
	})

	t.Run("GetIncidentWithContext returns error for err ID", func(t *testing.T) {
		_, err := mock.GetIncidentWithContext(ctx, "err")
		assert.ErrorIs(t, err, ErrMockError)
	})

	t.Run("ListIncidentsWithContext returns fixed incidents", func(t *testing.T) {
		resp, err := mock.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
		assert.NoError(t, err)
		assert.Len(t, resp.Incidents, 2)
		assert.Equal(t, "QABCDEFG1234567", resp.Incidents[0].ID)
		assert.Equal(t, "QABCDEFG7654321", resp.Incidents[1].ID)
	})

	t.Run("GetTeamWithContext returns team with name matching ID", func(t *testing.T) {
		team, err := mock.GetTeamWithContext(ctx, "myteam")
		assert.NoError(t, err)
		assert.Equal(t, "myteam", team.Name)
	})

	t.Run("ManageIncidentsWithContext returns error for err ID", func(t *testing.T) {
		opts := []pagerduty.ManageIncidentsOptions{
			{ID: "err"},
		}
		_, err := mock.ManageIncidentsWithContext(ctx, "test@example.com", opts)
		assert.ErrorIs(t, err, ErrMockError)
	})

	t.Run("ManageIncidentsWithContext returns incidents for normal IDs", func(t *testing.T) {
		opts := []pagerduty.ManageIncidentsOptions{
			{ID: "P12345"},
		}
		resp, err := mock.ManageIncidentsWithContext(ctx, "test@example.com", opts)
		assert.NoError(t, err)
		assert.Len(t, resp.Incidents, 2)
	})
}

func TestEnhancedMock_PaginatedListMembers(t *testing.T) {
	// Configure mock to return More=true on first call, More=false on second.
	// This exercises the response queue feature for ListMembersWithContext.
	mock := &MockPagerDutyClient{
		ListMembersResponses: []ListMembersResponse{
			{
				Response: &pagerduty.ListTeamMembersResponse{
					APIListObject: pagerduty.APIListObject{More: true},
					Members: []pagerduty.Member{
						{User: pagerduty.APIObject{ID: "USER1"}},
						{User: pagerduty.APIObject{ID: "USER2"}},
					},
				},
			},
			{
				Response: &pagerduty.ListTeamMembersResponse{
					APIListObject: pagerduty.APIListObject{More: false},
					Members: []pagerduty.Member{
						{User: pagerduty.APIObject{ID: "USER3"}},
					},
				},
			},
		},
	}

	// Use GetTeamMemberIDs which loops over ListMembersWithContext pagination
	teams := []*pagerduty.Team{
		{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
	}
	opts := pagerduty.ListTeamMembersOptions{Limit: 2, Offset: 0}
	memberIDs, err := GetTeamMemberIDs(mock, teams, opts)

	assert.NoError(t, err)
	assert.Equal(t, []string{"USER1", "USER2", "USER3"}, memberIDs)
}

func TestEnhancedMock_PaginatedListMembersError(t *testing.T) {
	// Configure mock to return an error on the second page
	mock := &MockPagerDutyClient{
		ListMembersResponses: []ListMembersResponse{
			{
				Response: &pagerduty.ListTeamMembersResponse{
					APIListObject: pagerduty.APIListObject{More: true},
					Members: []pagerduty.Member{
						{User: pagerduty.APIObject{ID: "USER1"}},
					},
				},
			},
			{
				Err: ErrMockError,
			},
		},
	}

	teams := []*pagerduty.Team{
		{APIObject: pagerduty.APIObject{ID: "TEAM1"}},
	}
	opts := pagerduty.ListTeamMembersOptions{Limit: 1, Offset: 0}
	_, err := GetTeamMemberIDs(mock, teams, opts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve users")
}

func TestEnhancedMock_CallCounting(t *testing.T) {
	mock := &MockPagerDutyClient{}
	ctx := context.Background()

	// Initially all counts should be zero
	assert.Equal(t, 0, mock.CallCounts["GetIncidentWithContext"])
	assert.Equal(t, 0, mock.CallCounts["ListIncidentsWithContext"])

	// Make some calls
	_, _ = mock.GetIncidentWithContext(ctx, "P1")
	_, _ = mock.GetIncidentWithContext(ctx, "P2")
	_, _ = mock.GetIncidentWithContext(ctx, "P3")

	assert.Equal(t, 3, mock.CallCounts["GetIncidentWithContext"])

	_, _ = mock.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
	assert.Equal(t, 1, mock.CallCounts["ListIncidentsWithContext"])

	// Methods not called should still be zero
	assert.Equal(t, 0, mock.CallCounts["GetTeamWithContext"])

	// Call GetTeamWithContext once
	_, _ = mock.GetTeamWithContext(ctx, "team1")
	assert.Equal(t, 1, mock.CallCounts["GetTeamWithContext"])
}

func TestEnhancedMock_CallCountingAllMethods(t *testing.T) {
	mock := &MockPagerDutyClient{}
	ctx := context.Background()

	_, _ = mock.ListIncidentAlertsWithContext(ctx, "P1", pagerduty.ListIncidentAlertsOptions{})
	_, _ = mock.ListIncidentNotesWithContext(ctx, "P1")
	_, _ = mock.ManageIncidentsWithContext(ctx, "test@example.com", []pagerduty.ManageIncidentsOptions{{ID: "P1"}})

	assert.Equal(t, 1, mock.CallCounts["ListIncidentAlertsWithContext"])
	assert.Equal(t, 1, mock.CallCounts["ListIncidentNotesWithContext"])
	assert.Equal(t, 1, mock.CallCounts["ManageIncidentsWithContext"])
}

func TestEnhancedMock_ResponseQueueFallback(t *testing.T) {
	// When ListMembersResponses is nil (not configured), the mock should still
	// satisfy the interface. We test that it doesn't panic.
	mock := &MockPagerDutyClient{}
	ctx := context.Background()

	// ListMembersWithContext without a response queue should return a default
	// empty response (backward compatible fallback)
	resp, err := mock.ListMembersWithContext(ctx, "TEAM1", pagerduty.ListTeamMembersOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.More)
}
