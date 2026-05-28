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
