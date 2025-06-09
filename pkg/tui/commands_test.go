package tui

import (
	"fmt"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/clcollins/srepd/pkg/rand"
	"github.com/stretchr/testify/assert"
)

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
		name       string
		incidents  []pagerduty.Incident
		reEscalate bool
		expected   tea.Msg
	}{
		{
			name:       "return unAcknowledgedIncidentMsg with non-nil error if error occurs while re-escalating",
			incidents:  errIncidents,
			reEscalate: true,
			expected: unAcknowledgedIncidentsMsg{
				incidents: []pagerduty.Incident(nil),
				err:       pd.ErrMockError,
			},
		},
		{
			name:       "return unAcknowledgedIncidentMsg with an incident list if no error occurs while re-escalating",
			incidents:  incidents,
			reEscalate: true,
			expected: unAcknowledgedIncidentsMsg{
				incidents: incidents,
			},
		},
		{
			name:       "return acknowledgedIncidentMsg with non-nil error if error occurs while acknowledging",
			incidents:  errIncidents,
			reEscalate: false,
			expected: acknowledgedIncidentsMsg{
				incidents: []pagerduty.Incident(nil),
				err:       pd.ErrMockError,
			},
		},
		{
			name:       "return acknowledgedIncidentMsg with an incident list if no error occurs while acknowledging",
			incidents:  incidents,
			reEscalate: false,
			expected: acknowledgedIncidentsMsg{
				incidents: incidents,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := acknowledgeIncidents(mockConfig, test.incidents, test.reEscalate)
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
		{
			name:   "return gotIncidentMsg with not-nil error if error occurs",
			config: mockConfig,
			id:     "err", // "err" signals the mock client to produce a mock error
			expected: gotIncidentMsg{
				incident: &pagerduty.Incident{},
				err:      pd.ErrMockError,
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

func TestGetIncidentAlerts(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
	}
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
			id:     rand.ID("Q"),
			expected: gotIncidentAlertsMsg{
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
				alerts: nil,
				err:    fmt.Errorf("pd.GetAlerts(): failed to get alerts for incident `%v`: %v", "err", pd.ErrMockError),
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
			id:     rand.ID("Q"),
			expected: gotIncidentNotesMsg{
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
				notes: []pagerduty.IncidentNote{},
				err:   fmt.Errorf("pd.GetNotes(): failed to get incident notes `%v`: %v", "err", pd.ErrMockError),
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

func TestUpdateIncidentList(t *testing.T) {
	tests := []struct {
		name     string
		config   *pd.Config
		expected tea.Msg
	}{
		{
			name: "return updatedIncidentListMsg with non-nil error if error occurs",
			config: &pd.Config{
				Client:         &pd.MockPagerDutyClient{},
				TeamsMemberIDs: []string{"err"}, // "err" signals the mock client to produce a mock error
			},
			expected: updatedIncidentListMsg{
				incidents: []pagerduty.Incident(nil),
				err:       fmt.Errorf("pd.GetIncidents(): failed to get incidents: %v", pd.ErrMockError),
			},
		},
		{
			name:   "return updatedIncidentListMsg with an incident list if no error occurs",
			config: &pd.Config{Client: &pd.MockPagerDutyClient{}},
			expected: updatedIncidentListMsg{
				// These incidents are defined in the Mock for ListIncidentsWithContext
				incidents: []pagerduty.Incident{
					{APIObject: pagerduty.APIObject{ID: "QABCDEFG1234567"}},
					{APIObject: pagerduty.APIObject{ID: "QABCDEFG7654321"}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := updateIncidentList(test.config)
			actual := cmd()
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestNewListIncidentOptsFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *pd.Config
		expected pagerduty.ListIncidentsOptions
	}{
		{
			name:     "return default ListIncidentsOptions if config is nil",
			config:   nil,
			expected: pagerduty.ListIncidentsOptions{},
		},
		{
			name: "p.TeamsMembers is properly converted to ListIncidentsOptions.UserIDs",
			config: &pd.Config{
				TeamsMemberIDs: []string{
					"PABC123",
					"PDEF456",
				},
			},
			expected: pagerduty.ListIncidentsOptions{
				UserIDs: []string{"PABC123", "PDEF456"},
			},
		},
		{
			name: "p.IgnopredUsers are properly excluded from ListIncidentsOptions.UserIDs, and extra Ignored users have no effect",
			config: &pd.Config{
				TeamsMemberIDs: []string{
					"PABC123",
					"PDEF456",
				},
				IgnoredUsers: []*pagerduty.User{
					{APIObject: pagerduty.APIObject{ID: "PDEF456"}},
					{APIObject: pagerduty.APIObject{ID: "PXYZ789"}},
				},
			},
			expected: pagerduty.ListIncidentsOptions{
				UserIDs: []string{"PABC123"},
			},
		},
		{
			name: "p.Teams is properly converted to ListIncidentsOptions.TeamIDs",
			config: &pd.Config{
				Teams: []*pagerduty.Team{
					{APIObject: pagerduty.APIObject{ID: "PABC123"}},
					{APIObject: pagerduty.APIObject{ID: "PDEF456"}},
				},
			},
			expected: pagerduty.ListIncidentsOptions{
				TeamIDs: []string{"PABC123", "PDEF456"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := newListIncidentOptsFromConfig(test.config)
			assert.Equal(t, test.expected, actual)
		})
	}
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
