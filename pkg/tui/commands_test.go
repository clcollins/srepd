package tui

import (
	"fmt"
	"strings"
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
		name      string
		incidents []pagerduty.Incident
		expected  tea.Msg
	}{
		{
			name:      "return acknowledgedIncidentMsg with non-nil error if error occurs while acknowledging",
			incidents: errIncidents,
			expected: acknowledgedIncidentsMsg{
				incidents: []pagerduty.Incident(nil),
				err:       pd.ErrMockError,
			},
		},
		{
			name:      "return acknowledgedIncidentMsg with an incident list if no error occurs while acknowledging",
			incidents: incidents,
			expected: acknowledgedIncidentsMsg{
				incidents: incidents,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := acknowledgeIncidents(mockConfig, test.incidents)
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
	testID := rand.ID("Q")
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
			id:     testID,
			expected: gotIncidentAlertsMsg{
				incidentID: testID,
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
				incidentID: "err",
				alerts:     nil,
				err:        fmt.Errorf("pd.GetAlerts(): failed to get alerts for incident `%v`: %v", "err", pd.ErrMockError),
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
	testID := rand.ID("Q")
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
			id:     testID,
			expected: gotIncidentNotesMsg{
				incidentID: testID,
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
				incidentID: "err",
				notes:      []pagerduty.IncidentNote{},
				err:        fmt.Errorf("pd.GetNotes(): failed to get incident notes `%v`: %v", "err", pd.ErrMockError),
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

func TestOpenBrowserCmd(t *testing.T) {
	tests := []struct {
		name          string
		browser       []string
		url           string
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful command execution",
			browser:     []string{"echo", "browser"},
			url:         "https://example.com",
			expectError: false,
		},
		{
			name:          "command not found returns error",
			browser:       []string{"nonexistent-browser-command-xyz"},
			url:           "https://example.com",
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := openBrowserCmd(tt.browser, tt.url)
			result := cmd()

			msg, ok := result.(browserFinishedMsg)
			assert.True(t, ok, "Expected browserFinishedMsg type")

			if tt.expectError {
				assert.NotNil(t, msg.err, "Expected error but got nil")
				if tt.errorContains != "" {
					assert.Contains(t, msg.err.Error(), tt.errorContains, "Error message mismatch")
				}
			} else {
				assert.Nil(t, msg.err, "Expected no error but got: %v", msg.err)
			}
		})
	}
}

// envVarMap converts a buildPagerDutyEnvVars result slice into a map of
// variable name to value for easier assertion. Each pair is "-e", "KEY=VALUE".
func envVarMap(flags []string) map[string]string {
	m := make(map[string]string)
	for i := 0; i < len(flags)-1; i += 2 {
		if flags[i] != "-e" {
			continue
		}
		parts := strings.SplitN(flags[i+1], "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func TestBuildPagerDutyEnvVars_FullIncident(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID:      "PD123",
			HTMLURL: "https://pagerduty.com/incidents/PD123",
		},
		Title:   "Test Incident",
		Urgency: "high",
		Status:  "triggered",
		Service: pagerduty.APIObject{
			Summary: "test-service",
		},
	}

	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "HighCPU",
					"link":       "https://example.com/sop/highcpu",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id":  "cluster-abc",
					"alert_name":  "LowMemory",
					"runbook_url": "https://example.com/sop/lowmem",
				},
			},
		},
	}

	notes := []pagerduty.IncidentNote{
		{ID: "NOTE1", Content: "Investigating"},
		{ID: "NOTE2", Content: "Found root cause"},
	}

	result := buildPagerDutyEnvVars(incident, alerts, notes, "cluster-abc")
	vars := envVarMap(result)

	assert.Equal(t, "PD123", vars["PAGERDUTY_INCIDENT_ID"])
	assert.Equal(t, "Test Incident", vars["PAGERDUTY_INCIDENT_TITLE"])
	assert.Equal(t, "https://pagerduty.com/incidents/PD123", vars["PAGERDUTY_INCIDENT_URL"])
	assert.Equal(t, "test-service", vars["PAGERDUTY_INCIDENT_SERVICE"])
	assert.Equal(t, "high", vars["PAGERDUTY_INCIDENT_URGENCY"])
	assert.Equal(t, "triggered", vars["PAGERDUTY_INCIDENT_STATUS"])
	assert.Equal(t, "https://pagerduty.com/incidents/PD123", vars["REASON"])
	assert.Equal(t, "cluster-abc", vars["PAGERDUTY_CLUSTER_ID"])
	assert.Equal(t, "2", vars["PAGERDUTY_ALERT_COUNT"])
	assert.Equal(t, "HighCPU,LowMemory", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "https://example.com/sop/highcpu,https://example.com/sop/lowmem", vars["PAGERDUTY_ALERT_LINKS"])
	assert.Equal(t, "true", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "2", vars["PAGERDUTY_NOTE_COUNT"])

	// Every entry should be a "-e" / "KEY=VALUE" pair
	for i := 0; i < len(result); i += 2 {
		assert.Equal(t, "-e", result[i], "Expected -e flag at position %d", i)
	}
}

func TestBuildPagerDutyEnvVars_FiltersByCluster(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "PD456"},
		Service:   pagerduty.APIObject{Summary: "svc"},
	}

	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "AlertA",
					"link":       "https://sop/a",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-xyz",
					"alert_name": "AlertX",
					"link":       "https://sop/x",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id":  "cluster-abc",
					"alert_name":  "AlertB",
					"runbook_url": "https://sop/b",
				},
			},
		},
	}

	result := buildPagerDutyEnvVars(incident, alerts, nil, "cluster-abc")
	vars := envVarMap(result)

	assert.Equal(t, "2", vars["PAGERDUTY_ALERT_COUNT"], "Should only count alerts matching cluster-abc")
	assert.Equal(t, "AlertA,AlertB", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "https://sop/a,https://sop/b", vars["PAGERDUTY_ALERT_LINKS"])
}

func TestBuildPagerDutyEnvVars_NilIncident(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "OrphanAlert",
				},
			},
		},
	}

	result := buildPagerDutyEnvVars(nil, alerts, nil, "cluster-abc")
	vars := envVarMap(result)

	// Should not contain incident-level vars
	_, hasIncidentID := vars["PAGERDUTY_INCIDENT_ID"]
	assert.False(t, hasIncidentID, "Nil incident should not produce PAGERDUTY_INCIDENT_ID")

	// Should still have cluster and alert vars
	assert.Equal(t, "cluster-abc", vars["PAGERDUTY_CLUSTER_ID"])
	assert.Equal(t, "1", vars["PAGERDUTY_ALERT_COUNT"])
	assert.Equal(t, "OrphanAlert", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "false", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "0", vars["PAGERDUTY_NOTE_COUNT"])
}

func TestBuildPagerDutyEnvVars_NoMatchingAlerts(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "PD789"},
		Service:   pagerduty.APIObject{Summary: "svc"},
	}

	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-other",
					"alert_name": "OtherAlert",
					"link":       "https://sop/other",
				},
			},
		},
	}

	result := buildPagerDutyEnvVars(incident, alerts, nil, "cluster-abc")
	vars := envVarMap(result)

	assert.Equal(t, "0", vars["PAGERDUTY_ALERT_COUNT"])
	assert.Equal(t, "", vars["PAGERDUTY_ALERT_NAMES"])
	assert.Equal(t, "", vars["PAGERDUTY_ALERT_LINKS"])
}

func TestBuildPagerDutyEnvVars_NotesExist(t *testing.T) {
	notes := []pagerduty.IncidentNote{
		{ID: "N1", Content: "note one"},
		{ID: "N2", Content: "note two"},
		{ID: "N3", Content: "note three"},
	}

	result := buildPagerDutyEnvVars(nil, nil, notes, "")
	vars := envVarMap(result)

	assert.Equal(t, "true", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "3", vars["PAGERDUTY_NOTE_COUNT"])
}

func TestBuildPagerDutyEnvVars_NoNotes(t *testing.T) {
	result := buildPagerDutyEnvVars(nil, nil, nil, "")
	vars := envVarMap(result)

	assert.Equal(t, "false", vars["PAGERDUTY_NOTES_EXIST"])
	assert.Equal(t, "0", vars["PAGERDUTY_NOTE_COUNT"])
}

func TestBuildPagerDutyEnvVars_ManyAlerts(t *testing.T) {
	// 50 alerts for the same cluster should not cause size issues
	var alerts []pagerduty.IncidentAlert
	for i := 0; i < 50; i++ {
		alerts = append(alerts, pagerduty.IncidentAlert{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-big",
					"alert_name": fmt.Sprintf("Alert%d", i),
					"link":       fmt.Sprintf("https://sop/%d", i),
				},
			},
		})
	}

	result := buildPagerDutyEnvVars(nil, alerts, nil, "cluster-big")
	vars := envVarMap(result)

	assert.Equal(t, "50", vars["PAGERDUTY_ALERT_COUNT"])

	names := strings.Split(vars["PAGERDUTY_ALERT_NAMES"], ",")
	assert.Equal(t, 50, len(names), "Should have 50 comma-separated alert names")

	links := strings.Split(vars["PAGERDUTY_ALERT_LINKS"], ",")
	assert.Equal(t, 50, len(links), "Should have 50 comma-separated alert links")

	// Each individual env var value is a simple string, no base64 encoding
	for _, flag := range result {
		if flag == "-e" {
			continue
		}
		// No value should contain base64-only artifacts from the old approach
		assert.NotContains(t, flag, "ALERT_DETAILS", "Should not use old ALERT_DETAILS variable")
	}
}

func TestGetDetailFieldFromAlert_NilBody(t *testing.T) {
	// Alert with nil Body map should return ""
	alert := pagerduty.IncidentAlert{
		Body: nil,
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_EmptyBody(t *testing.T) {
	// Alert with empty Body map (no "details" key) should return ""
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_MissingDetails(t *testing.T) {
	// Body exists but has no "details" key
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"other_key": "some_value",
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_DetailsNotMap(t *testing.T) {
	// Body["details"] is a string, not a map
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": "this is a string, not a map",
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_FieldNotString(t *testing.T) {
	// Field exists in details but is an int, not a string
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"cluster_id": 12345,
			},
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_FieldIsBool(t *testing.T) {
	// Field exists in details but is a bool, not a string
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"active": true,
			},
		},
	}
	result := getDetailFieldFromAlert("active", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_MissingField(t *testing.T) {
	// Details map exists but requested field is not present
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"alert_name": "TestAlert",
			},
		},
	}
	result := getDetailFieldFromAlert("cluster_id", alert)
	assert.Equal(t, "", result)
}

func TestGetDetailFieldFromAlert_HappyPath(t *testing.T) {
	// Valid structure with string field returns the value
	alert := pagerduty.IncidentAlert{
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"cluster_id": "abc-123-def",
				"alert_name": "TestAlert",
				"link":       "https://example.com/sop",
			},
		},
	}
	assert.Equal(t, "abc-123-def", getDetailFieldFromAlert("cluster_id", alert))
	assert.Equal(t, "TestAlert", getDetailFieldFromAlert("alert_name", alert))
	assert.Equal(t, "https://example.com/sop", getDetailFieldFromAlert("link", alert))
}

func TestFilterByUrgency_ShowAll(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "low"},
		{APIObject: pagerduty.APIObject{ID: "Q003"}, Urgency: "high"},
	}

	result := filterByUrgency(incidents, true)
	assert.Equal(t, 3, len(result), "showLow=true should return all incidents unchanged")
	assert.Equal(t, "Q001", result[0].ID)
	assert.Equal(t, "Q002", result[1].ID)
	assert.Equal(t, "Q003", result[2].ID)
}

func TestFilterByUrgency_HighOnly(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "low"},
		{APIObject: pagerduty.APIObject{ID: "Q003"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q004"}, Urgency: "low"},
	}

	result := filterByUrgency(incidents, false)
	assert.Equal(t, 2, len(result), "showLow=false should return only high-urgency incidents")
	assert.Equal(t, "Q001", result[0].ID)
	assert.Equal(t, "Q003", result[1].ID)
}

func TestFilterByUrgency_EmptyList(t *testing.T) {
	var incidents []pagerduty.Incident

	result := filterByUrgency(incidents, true)
	assert.Empty(t, result, "empty input should return empty result")

	result = filterByUrgency(incidents, false)
	assert.Empty(t, result, "empty input should return empty result regardless of filter")
}

func TestFilterByUrgency_AllLow(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "low"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "low"},
	}

	result := filterByUrgency(incidents, false)
	assert.Empty(t, result, "all low urgency with filter on should return empty")
}

func TestFilterByUrgency_AllHigh(t *testing.T) {
	incidents := []pagerduty.Incident{
		{APIObject: pagerduty.APIObject{ID: "Q001"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q002"}, Urgency: "high"},
		{APIObject: pagerduty.APIObject{ID: "Q003"}, Urgency: "high"},
	}

	result := filterByUrgency(incidents, false)
	assert.Equal(t, 3, len(result), "all high urgency with filter on should return all")
	assert.Equal(t, "Q001", result[0].ID)
	assert.Equal(t, "Q002", result[1].ID)
	assert.Equal(t, "Q003", result[2].ID)
}

func TestLoginCommandStructureWithEnvVars(t *testing.T) {
	// This test validates that environment variables are inserted at the correct
	// position in the command - after the terminal separator but as arguments to
	// ocm-container, not to the terminal itself

	testCases := []struct {
		name           string
		inputCommand   []string
		expectEnvFlags bool
		description    string
	}{
		{
			name:           "gnome-terminal with separator",
			inputCommand:   []string{"gnome-terminal", "--", "ocm-container", "--cluster-id", "ABC123"},
			expectEnvFlags: true,
			description:    "Should insert env flags after -- but before ocm-container args",
		},
		{
			name:           "direct ocm-container command",
			inputCommand:   []string{"ocm-container", "--cluster-id", "ABC123"},
			expectEnvFlags: true,
			description:    "Should insert env flags after ocm-container command",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envFlags := []string{"-e", "PAGERDUTY_INCIDENT_ID=PD123"}

			// Find separator position
			var separatorIdx = -1
			for i, arg := range tc.inputCommand {
				if arg == "--" {
					separatorIdx = i
					break
				}
			}

			// Expected structure:
			// If separator exists: [terminal] [--] [command] [env-flags] [other-args]
			// If no separator: [command] [env-flags] [other-args]

			if separatorIdx >= 0 {
				assert.Greater(t, len(tc.inputCommand), separatorIdx+1,
					"Command should have elements after separator")
			}

			assert.NotEmpty(t, envFlags, "Env flags should not be empty")
		})
	}
}

func TestGetSOPLink_HasLink(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"link": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/some-alert.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/some-alert.md", link)
}

func TestGetSOPLink_NoLink(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "abc-123",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.False(t, ok)
	assert.Equal(t, "", link)
}

func TestGetSOPLink_EmptyAlerts(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{}
	link, ok := getSOPLink(alerts)
	assert.False(t, ok)
	assert.Equal(t, "", link)
}

func TestGetSOPLink_NilAlerts(t *testing.T) {
	link, ok := getSOPLink(nil)
	assert.False(t, ok)
	assert.Equal(t, "", link)
}

func TestGetSOPLink_MultipleAlerts(t *testing.T) {
	// First alert has no link, second does - should return second's link
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"link": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/second-alert.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/second-alert.md", link)
}

func TestGetSOPLink_RunbookURL(t *testing.T) {
	// Alert uses "runbook_url" instead of "link" (Prometheus annotation convention)
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"runbook_url": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/UpgradeStateNotificationFailureSRE.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/UpgradeStateNotificationFailureSRE.md", link)
}

func TestGetSOPLink_LinkTakesPriorityOverRunbookURL(t *testing.T) {
	// Alert has both "link" and "runbook_url" - "link" should take priority
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"link":        "https://github.com/openshift/ops-sop/blob/master/v4/alerts/primary.md",
					"runbook_url": "https://github.com/openshift/ops-sop/blob/master/v4/alerts/fallback.md",
				},
			},
		},
	}
	link, ok := getSOPLink(alerts)
	assert.True(t, ok)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/primary.md", link)
}

func TestGetUniqueClusters_SingleCluster(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-abc-123"}, result)
}

func TestGetUniqueClusters_MultipleDifferent(t *testing.T) {
	// 3 alerts with 2 distinct cluster_ids should return 2 entries
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-def-456",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, 2, len(result))
	assert.Contains(t, result, "cluster-abc-123")
	assert.Contains(t, result, "cluster-def-456")
}

func TestGetUniqueClusters_NoClusterID(t *testing.T) {
	// Alerts without cluster_id should return empty slice
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"alert_name": "TestAlert",
				},
			},
		},
		{
			Body: nil,
		},
	}
	result := getUniqueClusters(alerts)
	assert.Empty(t, result)
}

func TestGetUniqueClusters_Deduplication(t *testing.T) {
	// Same cluster in multiple alerts should return only one entry
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-abc-123"}, result)
}

func TestGetUniqueClusters_PreservesOrder(t *testing.T) {
	// Order should match first appearance of each cluster
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-first",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-second",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-first",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-third",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-first", "cluster-second", "cluster-third"}, result)
}

func TestGetUniqueClusters_EmptyAlerts(t *testing.T) {
	result := getUniqueClusters([]pagerduty.IncidentAlert{})
	assert.Empty(t, result)
}

func TestGetUniqueClusters_NilAlerts(t *testing.T) {
	result := getUniqueClusters(nil)
	assert.Empty(t, result)
}

func TestStateShorthand_Triggered(t *testing.T) {
	// Incident with no acknowledgements should return dot
	incident := pagerduty.Incident{
		APIObject:        pagerduty.APIObject{ID: "INC001"},
		Acknowledgements: []pagerduty.Acknowledgement{},
	}

	result := stateShorthand(incident, "USER123")
	assert.Equal(t, dot, result, "triggered incident (no acknowledgements) should return dot")
}

func TestStateShorthand_AckedByUser(t *testing.T) {
	// Incident acknowledged by the current user should return "A"
	incident := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC002"},
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "USER123"}},
		},
	}

	result := stateShorthand(incident, "USER123")
	assert.Equal(t, "A", result, "incident acknowledged by current user should return 'A'")
}

func TestStateShorthand_AckedByOther(t *testing.T) {
	// Incident acknowledged by someone else should return "a"
	incident := pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "INC003"},
		Acknowledgements: []pagerduty.Acknowledgement{
			{Acknowledger: pagerduty.APIObject{ID: "OTHER_USER"}},
		},
	}

	result := stateShorthand(incident, "USER123")
	assert.Equal(t, "a", result, "incident acknowledged by another user should return 'a'")
}

func TestGetUniqueClusters_MixedWithAndWithoutClusterID(t *testing.T) {
	// Some alerts have cluster_id, some don't - should only return those that do
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc-123",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"alert_name": "NoCluster",
				},
			},
		},
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-def-456",
				},
			},
		},
	}
	result := getUniqueClusters(alerts)
	assert.Equal(t, []string{"cluster-abc-123", "cluster-def-456"}, result)
}

func TestSanitizeEnvValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "ClusterOperatorDown", "ClusterOperatorDown"},
		{"with spaces", "High CPU Alert", "High CPU Alert"},
		{"with newlines", "line1\nline2\rline3", "line1 line2line3"},
		{"with quotes", `alert "name" here`, "alert name here"},
		{"with backticks", "alert `cmd` here", "alert cmd here"},
		{"with dollar signs", "alert $VAR here", "alert VAR here"},
		{"with backslashes", `alert \n here`, "alert n here"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeEnvValue(tt.input))
		})
	}
}
