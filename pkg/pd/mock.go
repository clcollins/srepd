package pd

import (
	"context"
	"fmt"

	"github.com/PagerDuty/go-pagerduty"
)

var ErrMockError = fmt.Errorf("pd.Mock(): mock error") // Used to mock errors in unit tests

// ListMembersResponse holds a single response (or error) for ListMembersWithContext.
// Use a slice of these in MockPagerDutyClient.ListMembersResponses to simulate
// paginated responses across successive calls.
type ListMembersResponse struct {
	Response *pagerduty.ListTeamMembersResponse
	Err      error
}

type MockPagerDutyClient struct {
	PagerDutyClient

	// CallCounts tracks how many times each method has been called, keyed by
	// method name (e.g. "GetIncidentWithContext"). The map is lazily
	// initialized on first use so a zero-value MockPagerDutyClient works
	// without any setup.
	CallCounts map[string]int

	// ListMembersResponses is an optional response queue for
	// ListMembersWithContext. When populated, successive calls pop from the
	// front of the slice. When empty or nil, a default empty response is
	// returned instead.
	ListMembersResponses []ListMembersResponse

	// listMembersIndex tracks the current position in ListMembersResponses.
	listMembersIndex int
}

// recordCall increments the call count for the named method, lazily
// initializing the CallCounts map if needed.
func (m *MockPagerDutyClient) recordCall(method string) {
	if m.CallCounts == nil {
		m.CallCounts = make(map[string]int)
	}
	m.CallCounts[method]++
}

func (m *MockPagerDutyClient) GetTeamWithContext(ctx context.Context, team string) (*pagerduty.Team, error) {
	m.recordCall("GetTeamWithContext")
	return &pagerduty.Team{Name: team}, nil
}

func (m *MockPagerDutyClient) GetIncidentWithContext(ctx context.Context, id string) (*pagerduty.Incident, error) {
	m.recordCall("GetIncidentWithContext")
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

func (m *MockPagerDutyClient) ListIncidentsWithContext(ctx context.Context, opts pagerduty.ListIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	m.recordCall("ListIncidentsWithContext")
	// Provided so we can mock error responses for unit tests
	if opts.UserIDs != nil && opts.UserIDs[0] == "err" {
		return &pagerduty.ListIncidentsResponse{}, ErrMockError
	}
	return &pagerduty.ListIncidentsResponse{
		Incidents: []pagerduty.Incident{
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

func (m *MockPagerDutyClient) ListIncidentAlertsWithContext(ctx context.Context, id string, opts pagerduty.ListIncidentAlertsOptions) (*pagerduty.ListAlertsResponse, error) {
	m.recordCall("ListIncidentAlertsWithContext")
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
	m.recordCall("ListIncidentNotesWithContext")
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

func (m *MockPagerDutyClient) ManageIncidentsWithContext(ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	m.recordCall("ManageIncidentsWithContext")
	var response = &pagerduty.ListIncidentsResponse{}

	for _, opt := range opts {
		// Provided so we can mock error responses for unit tests
		if opt.ID == "err" {
			return response, ErrMockError
		}
	}

	response.Incidents = []pagerduty.Incident{
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
	}

	return response, nil
}

// ListMembersWithContext returns responses from the ListMembersResponses queue
// when configured, otherwise returns a default empty response. This allows
// tests to simulate paginated member listings by enqueuing multiple responses
// with different More values.
func (m *MockPagerDutyClient) ListMembersWithContext(ctx context.Context, id string, opts pagerduty.ListTeamMembersOptions) (*pagerduty.ListTeamMembersResponse, error) {
	m.recordCall("ListMembersWithContext")

	// If a response queue is configured, pop from it
	if len(m.ListMembersResponses) > 0 && m.listMembersIndex < len(m.ListMembersResponses) {
		entry := m.ListMembersResponses[m.listMembersIndex]
		m.listMembersIndex++
		if entry.Err != nil {
			return &pagerduty.ListTeamMembersResponse{}, entry.Err
		}
		return entry.Response, nil
	}

	// Default fallback: return an empty, non-paginated response
	return &pagerduty.ListTeamMembersResponse{}, nil
}
