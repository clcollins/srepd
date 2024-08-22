package pd

import (
	"context"
	"fmt"

	"github.com/PagerDuty/go-pagerduty"
)

var ErrMockError = fmt.Errorf("pd.Mock(): mock error") // Used to mock errors in unit tests

type MockPagerDutyClient struct {
	PagerDutyClient
}

func (m *MockPagerDutyClient) GetTeamWithContext(ctx context.Context, team string) (*pagerduty.Team, error) {
	return &pagerduty.Team{Name: team}, nil
}

func (m *MockPagerDutyClient) GetIncidentWithContext(ctx context.Context, id string) (*pagerduty.Incident, error) {
	// Provided so we can mock error responses for unit tests
	if id == "err" {
		return &pagerduty.Incident{}, ErrMockError
	}
	return &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID: id, // Incidents will always come back with the same ID as the request
		},
	}, nil
}

func (m *MockPagerDutyClient) ListIncidentAlertsWithContext(ctx context.Context, id string, opts pagerduty.ListIncidentAlertsOptions) (*pagerduty.ListAlertsResponse, error) {
	if id == "err" {
		return &pagerduty.ListAlertsResponse{}, ErrMockError
	}
	return &pagerduty.ListAlertsResponse{
		Alerts: []pagerduty.IncidentAlert{
			{
				APIObject: pagerduty.APIObject{
					ID: "QABCDEFG1234567",
				},
			},
			{
				APIObject: pagerduty.APIObject{
					ID: "QABCDEFG7654321",
				},
			},
		},
	}, nil
}

func (m *MockPagerDutyClient) ListIncidentNotesWithContext(ctx context.Context, id string) ([]pagerduty.IncidentNote, error) {
	if id == "err" {
		return []pagerduty.IncidentNote{}, ErrMockError
	}
	return []pagerduty.IncidentNote{
		{
			ID: "QABCDEFG1234567",
		},
		{
			ID: "QABCDEFG7654321",
		},
	}, nil
}
