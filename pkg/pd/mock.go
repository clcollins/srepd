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

	// GetCurrentUserErr, when non-nil, causes GetCurrentUserWithContext to return this error.
	GetCurrentUserErr error

	// LastManageIncidentsOpts records the options from the most recent
	// ManageIncidentsWithContext call, so tests can assert exactly what was sent
	// (e.g. EscalationLevel and whether EscalationPolicy was included).
	LastManageIncidentsOpts []pagerduty.ManageIncidentsOptions

	// ListMembersResponses is an optional response queue for
	// ListMembersWithContext. When populated, successive calls pop from the
	// front of the slice. When empty or nil, a default response is returned.
	ListMembersResponses []ListMembersResponse

	// listMembersIndex tracks the current position in ListMembersResponses.
	listMembersIndex int

	// ListOnCallsResponses is a queue of responses for ListOnCallsWithContext.
	// When configured, responses are popped in order; when empty, falls back to default behavior.
	ListOnCallsResponses []pagerduty.ListOnCallsResponse

	// ListMembersOffsets records the offset value from each call to ListMembersWithContext,
	// in order. Useful for verifying pagination offset reset behavior.
	ListMembersOffsets []uint

	// EscalationPolicyResponses maps policy IDs to specific responses for
	// GetEscalationPolicyWithContext. When non-nil and a matching key exists,
	// that policy is returned. Otherwise falls back to the default response.
	EscalationPolicyResponses map[string]*pagerduty.EscalationPolicy

	// ListEscalationPoliciesResponses is a queue of responses for ListEscalationPoliciesWithContext.
	ListEscalationPoliciesResponses []pagerduty.ListEscalationPoliciesResponse

	// ListEscalationPoliciesErr, when non-nil, causes
	// ListEscalationPoliciesWithContext to return this error.
	ListEscalationPoliciesErr error

	// ListIncidentsResponses is an optional response queue for
	// ListIncidentsWithContext. When populated, successive calls pop from the
	// front. When empty or nil, the default hardcoded response is returned.
	ListIncidentsResponses []pagerduty.ListIncidentsResponse

	// RecordedListIncidentsOpts records the options from every
	// ListIncidentsWithContext call, in order, so tests can assert exactly
	// what was sent (e.g. Limit, Statuses, Offset, and user_ids chunking).
	RecordedListIncidentsOpts []pagerduty.ListIncidentsOptions

	// ListIncidentAlertsResponses maps incident ID to a specific alerts response
	// for ListIncidentAlertsWithContext. When non-nil and a matching key exists,
	// that response is returned. Otherwise falls back to the default response.
	ListIncidentAlertsResponses map[string]*pagerduty.ListAlertsResponse

	// RecordedListAlertsOpts records the options from every
	// ListIncidentAlertsWithContext call, in order.
	RecordedListAlertsOpts []pagerduty.ListIncidentAlertsOptions

	// RecordedListOnCallOpts records the options from every
	// ListOnCallsWithContext call, in order.
	RecordedListOnCallOpts []pagerduty.ListOnCallOptions
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
	m.RecordedListIncidentsOpts = append(m.RecordedListIncidentsOpts, opts)
	if opts.UserIDs != nil && opts.UserIDs[0] == "err" {
		return &pagerduty.ListIncidentsResponse{}, ErrMockError
	}

	if len(m.ListIncidentsResponses) > 0 {
		resp := m.ListIncidentsResponses[0]
		m.ListIncidentsResponses = m.ListIncidentsResponses[1:]
		return &resp, nil
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
	m.RecordedListAlertsOpts = append(m.RecordedListAlertsOpts, opts)
	if id == "err" {
		return &pagerduty.ListAlertsResponse{}, ErrMockError
	}

	if m.ListIncidentAlertsResponses != nil {
		if resp, ok := m.ListIncidentAlertsResponses[id]; ok {
			return resp, nil
		}
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
	m.LastManageIncidentsOpts = opts
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
// when configured, otherwise returns a default member response.
func (m *MockPagerDutyClient) ListMembersWithContext(ctx context.Context, id string, opts pagerduty.ListTeamMembersOptions) (*pagerduty.ListTeamMembersResponse, error) {
	m.recordCall("ListMembersWithContext")
	m.ListMembersOffsets = append(m.ListMembersOffsets, opts.Offset)

	// If a response queue is configured, pop from it
	if len(m.ListMembersResponses) > 0 && m.listMembersIndex < len(m.ListMembersResponses) {
		entry := m.ListMembersResponses[m.listMembersIndex]
		m.listMembersIndex++
		if entry.Err != nil {
			return &pagerduty.ListTeamMembersResponse{}, entry.Err
		}
		return entry.Response, nil
	}

	// Default behavior: return a single member with the team ID as the user ID
	return &pagerduty.ListTeamMembersResponse{
		Members: []pagerduty.Member{
			{
				User: pagerduty.APIObject{
					ID: id,
				},
			},
		},
	}, nil
}

func (m *MockPagerDutyClient) CreateIncidentNoteWithContext(ctx context.Context, id string, note pagerduty.IncidentNote) (*pagerduty.IncidentNote, error) {
	m.recordCall("CreateIncidentNoteWithContext")
	if id == "err" {
		return nil, ErrMockError
	}
	return &pagerduty.IncidentNote{
		ID:      "NOTE_MOCK_001",
		Content: note.Content,
		User:    note.User,
	}, nil
}

func (m *MockPagerDutyClient) GetCurrentUserWithContext(ctx context.Context, opts pagerduty.GetCurrentUserOptions) (*pagerduty.User, error) {
	m.recordCall("GetCurrentUserWithContext")
	if m.GetCurrentUserErr != nil {
		return nil, m.GetCurrentUserErr
	}
	user := &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: "MOCK_USER"},
		Name:      "Mock User",
		Email:     "mock@example.com",
	}
	for _, inc := range opts.Includes {
		if inc == "teams" {
			user.Teams = []pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "TEAM_001"}, Name: "Mock Team Alpha"},
				{APIObject: pagerduty.APIObject{ID: "TEAM_002"}, Name: "Mock Team Beta"},
			}
			break
		}
	}
	return user, nil
}

func (m *MockPagerDutyClient) GetEscalationPolicyWithContext(ctx context.Context, id string, opts *pagerduty.GetEscalationPolicyOptions) (*pagerduty.EscalationPolicy, error) {
	m.recordCall("GetEscalationPolicyWithContext")
	if id == "err" {
		return nil, ErrMockError
	}
	if m.EscalationPolicyResponses != nil {
		if policy, ok := m.EscalationPolicyResponses[id]; ok {
			return policy, nil
		}
	}
	return &pagerduty.EscalationPolicy{
		APIObject: pagerduty.APIObject{ID: id},
		Name:      "Mock Policy",
	}, nil
}

func (m *MockPagerDutyClient) GetUserWithContext(ctx context.Context, id string, opts pagerduty.GetUserOptions) (*pagerduty.User, error) {
	m.recordCall("GetUserWithContext")
	if id == "err" {
		return nil, ErrMockError
	}
	return &pagerduty.User{
		APIObject: pagerduty.APIObject{ID: id},
	}, nil
}

func (m *MockPagerDutyClient) ListEscalationPoliciesWithContext(ctx context.Context, opts pagerduty.ListEscalationPoliciesOptions) (*pagerduty.ListEscalationPoliciesResponse, error) {
	m.recordCall("ListEscalationPoliciesWithContext")

	if m.ListEscalationPoliciesErr != nil {
		return nil, m.ListEscalationPoliciesErr
	}

	if len(m.ListEscalationPoliciesResponses) > 0 {
		resp := m.ListEscalationPoliciesResponses[0]
		m.ListEscalationPoliciesResponses = m.ListEscalationPoliciesResponses[1:]
		return &resp, nil
	}

	return &pagerduty.ListEscalationPoliciesResponse{
		EscalationPolicies: []pagerduty.EscalationPolicy{},
	}, nil
}

func (m *MockPagerDutyClient) ListOnCallsWithContext(ctx context.Context, opts pagerduty.ListOnCallOptions) (*pagerduty.ListOnCallsResponse, error) {
	m.recordCall("ListOnCallsWithContext")
	m.RecordedListOnCallOpts = append(m.RecordedListOnCallOpts, opts)

	// If the response queue is configured, pop the first response
	if len(m.ListOnCallsResponses) > 0 {
		resp := m.ListOnCallsResponses[0]
		m.ListOnCallsResponses = m.ListOnCallsResponses[1:]
		return &resp, nil
	}

	// Default behavior: return empty on-calls
	return &pagerduty.ListOnCallsResponse{
		OnCalls: []pagerduty.OnCall{},
	}, nil
}

func (m *MockPagerDutyClient) MergeIncidentsWithContext(ctx context.Context, from string, id string, o []pagerduty.MergeIncidentsOptions) (*pagerduty.Incident, error) {
	m.recordCall("MergeIncidentsWithContext")
	if id == "err" {
		return nil, ErrMockError
	}
	return &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: id},
	}, nil
}
