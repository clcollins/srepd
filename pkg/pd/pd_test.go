package pd

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
)

type MockPagerDutyClient struct {
	PagerDutyClient
}

func (m *MockPagerDutyClient) GetTeamWithContext(ctx context.Context, team string) (*pagerduty.Team, error) {
	return &pagerduty.Team{Name: team}, nil
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
