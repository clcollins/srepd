package tui

import (
	"bytes"
	"html/template"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssigneeArea(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "formats 'You' assignee",
			input:    "You",
			expected: "Showing assigned to You",
		},
		{
			name:     "formats 'Team' assignee",
			input:    "Team",
			expected: "Showing assigned to Team",
		},
		{
			name:     "formats custom assignee",
			input:    "John Doe",
			expected: "Showing assigned to John Doe",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := assigneeArea(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestStatusArea(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		showSpinner bool
		spinnerView string
		expected    string
	}{
		{
			name:        "formats simple status without spinner",
			input:       "Loading...",
			showSpinner: false,
			spinnerView: "",
			expected:    "> Loading...",
		},
		{
			name:        "formats status with numbers without spinner",
			input:       "showing 2/5 incidents",
			showSpinner: false,
			spinnerView: "",
			expected:    "> showing 2/5 incidents",
		},
		{
			name:        "formats empty status without spinner",
			input:       "",
			showSpinner: false,
			spinnerView: "",
			expected:    "> ",
		},
		{
			name:        "formats status with spinner",
			input:       "Loading...",
			showSpinner: true,
			spinnerView: "⣾",
			expected:    "⣾ Loading...",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := statusArea(test.input, test.showSpinner, test.spinnerView)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRefreshArea(t *testing.T) {
	tests := []struct {
		name           string
		autoRefresh    bool
		autoAck        bool
		showLowUrgency bool
		expected       string
	}{
		{
			name:           "both enabled, all urgencies",
			autoRefresh:    true,
			autoAck:        true,
			showLowUrgency: true,
			expected:       "Watching for updates...  [auto-acknowledge]",
		},
		{
			name:           "auto-refresh enabled, auto-ack disabled, all urgencies",
			autoRefresh:    true,
			autoAck:        false,
			showLowUrgency: true,
			expected:       "Watching for updates... ",
		},
		{
			name:           "auto-refresh disabled, all urgencies",
			autoRefresh:    false,
			autoAck:        false,
			showLowUrgency: true,
			expected:       "Watching for updates...  [PAUSED]",
		},
		{
			name:           "auto-refresh disabled, auto-ack enabled (paused takes precedence), all urgencies",
			autoRefresh:    false,
			autoAck:        true,
			showLowUrgency: true,
			expected:       "Watching for updates...  [PAUSED]",
		},
		{
			name:           "high urgency only filter shown",
			autoRefresh:    true,
			autoAck:        false,
			showLowUrgency: false,
			expected:       "Watching for updates...  [high urgency only]",
		},
		{
			name:           "auto-ack and high urgency only",
			autoRefresh:    true,
			autoAck:        true,
			showLowUrgency: false,
			expected:       "Watching for updates...  [auto-acknowledge] [high urgency only]",
		},
		{
			name:           "paused and high urgency only",
			autoRefresh:    false,
			autoAck:        false,
			showLowUrgency: false,
			expected:       "Watching for updates...  [PAUSED] [high urgency only]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := refreshArea(test.autoRefresh, test.autoAck, test.showLowUrgency)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestSummarizeNotes(t *testing.T) {
	tests := []struct {
		name     string
		input    []pagerduty.IncidentNote
		expected []noteSummary
	}{
		{
			name:     "empty notes list",
			input:    []pagerduty.IncidentNote{},
			expected: nil,
		},
		{
			name: "single note",
			input: []pagerduty.IncidentNote{
				{
					ID:        "NOTE123",
					User:      pagerduty.APIObject{Summary: "John Doe"},
					Content:   "This is a test note",
					CreatedAt: "2025-01-01T00:00:00Z",
				},
			},
			expected: []noteSummary{
				{
					ID:      "NOTE123",
					User:    "John Doe",
					Content: "This is a test note",
					Created: "2025-01-01T00:00:00Z",
				},
			},
		},
		{
			name: "multiple notes",
			input: []pagerduty.IncidentNote{
				{
					ID:        "NOTE1",
					User:      pagerduty.APIObject{Summary: "User 1"},
					Content:   "First note",
					CreatedAt: "2025-01-01T00:00:00Z",
				},
				{
					ID:        "NOTE2",
					User:      pagerduty.APIObject{Summary: "User 2"},
					Content:   "Second note",
					CreatedAt: "2025-01-02T00:00:00Z",
				},
			},
			expected: []noteSummary{
				{
					ID:      "NOTE1",
					User:    "User 1",
					Content: "First note",
					Created: "2025-01-01T00:00:00Z",
				},
				{
					ID:      "NOTE2",
					User:    "User 2",
					Content: "Second note",
					Created: "2025-01-02T00:00:00Z",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := summarizeNotes(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestSummarizeIncident(t *testing.T) {
	tests := []struct {
		name     string
		input    *pagerduty.Incident
		expected incidentSummary
	}{
		{
			name: "basic incident without priority",
			input: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{
					ID:      "INC123",
					HTMLURL: "https://example.pagerduty.com/incidents/INC123",
				},
				Title:            "Test incident",
				Service:          pagerduty.APIObject{Summary: "Test Service"},
				EscalationPolicy: pagerduty.APIObject{Summary: "Test Policy"},
				CreatedAt:        "2025-01-01T00:00:00Z",
				Urgency:          "high",
				Status:           "triggered",
			},
			expected: incidentSummary{
				ID:               "INC123",
				Title:            "Test incident",
				HTMLURL:          "https://example.pagerduty.com/incidents/INC123",
				Service:          "Test Service",
				EscalationPolicy: "Test Policy",
				Created:          "2025-01-01T00:00:00Z",
				Urgency:          "high",
				Priority:         "",
				Status:           "triggered",
				Teams:            nil,
				Assigned:         nil,
				Acknowledged:     nil,
			},
		},
		{
			name: "incident with priority and assignments",
			input: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{
					ID:      "INC456",
					HTMLURL: "https://example.pagerduty.com/incidents/INC456",
				},
				Title:            "High priority incident",
				Service:          pagerduty.APIObject{Summary: "Critical Service"},
				EscalationPolicy: pagerduty.APIObject{Summary: "Critical Policy"},
				CreatedAt:        "2025-01-01T00:00:00Z",
				Urgency:          "high",
				Status:           "acknowledged",
				Priority:         &pagerduty.Priority{APIObject: pagerduty.APIObject{Summary: "P1"}},
				Assignments: []pagerduty.Assignment{
					{Assignee: pagerduty.APIObject{Summary: "John Doe"}},
					{Assignee: pagerduty.APIObject{Summary: "Jane Smith"}},
				},
				Acknowledgements: []pagerduty.Acknowledgement{
					{Acknowledger: pagerduty.APIObject{Summary: "John Doe"}},
				},
			},
			expected: incidentSummary{
				ID:               "INC456",
				Title:            "High priority incident",
				HTMLURL:          "https://example.pagerduty.com/incidents/INC456",
				Service:          "Critical Service",
				EscalationPolicy: "Critical Policy",
				Created:          "2025-01-01T00:00:00Z",
				Urgency:          "high",
				Priority:         "P1",
				Status:           "acknowledged",
				Teams:            nil,
				Assigned:         []string{"John Doe", "Jane Smith"},
				Acknowledged:     []string{"John Doe"},
			},
		},
		{
			name: "incident with duplicate acknowledgements",
			input: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{
					ID:      "INC789",
					HTMLURL: "https://example.com",
				},
				Title:            "Test",
				Service:          pagerduty.APIObject{Summary: "Service"},
				EscalationPolicy: pagerduty.APIObject{Summary: "Policy"},
				CreatedAt:        "2025-01-01T00:00:00Z",
				Urgency:          "low",
				Status:           "acknowledged",
				Acknowledgements: []pagerduty.Acknowledgement{
					{Acknowledger: pagerduty.APIObject{Summary: "John Doe"}},
					{Acknowledger: pagerduty.APIObject{Summary: "John Doe"}},
					{Acknowledger: pagerduty.APIObject{Summary: "Jane Smith"}},
				},
			},
			expected: incidentSummary{
				ID:               "INC789",
				Title:            "Test",
				HTMLURL:          "https://example.com",
				Service:          "Service",
				EscalationPolicy: "Policy",
				Created:          "2025-01-01T00:00:00Z",
				Urgency:          "low",
				Priority:         "",
				Status:           "acknowledged",
				Teams:            nil,
				Assigned:         nil,
				Acknowledged:     []string{"John Doe", "Jane Smith"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := summarizeIncident(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRenderFooter_ContainsRefreshStatus(t *testing.T) {
	// renderFooter wraps refreshArea output; verify the footer contains it
	m := model{
		autoRefresh:     true,
		autoAcknowledge: false,
		showLowUrgency:  true,
	}

	result := m.renderFooter()

	assert.Contains(t, result, "Watching for updates...", "footer should contain the refresh status text from refreshArea")
}

func TestRenderBottomStatus_ShowsIncidentID(t *testing.T) {
	// When a selected incident is set, renderBottomStatus should include its ID
	m := model{
		selectedIncident: &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "PABC123456"},
		},
	}

	result := m.renderBottomStatus()

	assert.Contains(t, result, "PABC123456", "bottom status should contain the selected incident ID")
}

func TestRenderBottomStatus_ShowsGitSHA(t *testing.T) {
	// renderBottomStatus should include the GitSHA variable
	m := model{}

	result := m.renderBottomStatus()

	assert.Contains(t, result, GitSHA, "bottom status should contain the GitSHA value")
}

func TestSummarizeAlerts_EmptyAlerts(t *testing.T) {
	// Empty alert slice should return nil (empty) summary slice
	result := summarizeAlerts([]pagerduty.IncidentAlert{})

	assert.Nil(t, result, "summarizeAlerts with empty input should return nil")
}

func TestSummarizeAlerts_AlertWithNilBody(t *testing.T) {
	// Alert with nil Body should not panic and should return a summary with empty fields
	alerts := []pagerduty.IncidentAlert{
		{
			APIObject: pagerduty.APIObject{
				ID:      "ALERT_NIL_BODY",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT_NIL_BODY",
			},
			Service:   pagerduty.APIObject{Summary: "Test Service"},
			CreatedAt: "2025-06-01T00:00:00Z",
			Status:    "triggered",
			Incident:  pagerduty.APIReference{ID: "INC999"},
			Body:      nil,
		},
	}

	// Should not panic
	result := summarizeAlerts(alerts)

	assert.Len(t, result, 1, "should return exactly one alert summary")
	assert.Equal(t, "ALERT_NIL_BODY", result[0].ID, "alert ID should be preserved")
	assert.Equal(t, "", result[0].Name, "name should be empty when body is nil")
	assert.Equal(t, "", result[0].Link, "link should be empty when body is nil")
	assert.Equal(t, "", result[0].Cluster, "cluster should be empty when body is nil")
	assert.Nil(t, result[0].Details, "details should be nil when body is nil")
	assert.Equal(t, "Test Service", result[0].Service, "service summary should be preserved")
}

func TestAddNoteTemplate(t *testing.T) {
	tests := []struct {
		name             string
		id               string
		title            string
		service          string
		expectedContains []string
	}{
		{
			name:    "generates note template",
			id:      "INC123",
			title:   "Test Incident",
			service: "Test Service",
			expectedContains: []string{
				"Incident: INC123",
				"Summary: Test Incident",
				"Service: Test Service",
				"Please enter the note message content above",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := addNoteTemplate(test.id, test.title, test.service)
			assert.NoError(t, err)
			for _, expected := range test.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

// renderTestTemplate is a helper that executes the incidentTemplate with funcMap
// against the given incidentSummary and returns the rendered string.
func renderTestTemplate(t *testing.T, summary incidentSummary) string {
	t.Helper()
	tmpl, err := template.New("incident").Funcs(funcMap).Parse(incidentTemplate)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, summary)
	require.NoError(t, err)

	return buf.String()
}

func TestIncidentTemplate_AlertRendersAsMarkdownLink(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC001",
		Title:        "Test Incident",
		HTMLURL:      "https://example.pagerduty.com/incidents/INC001",
		Service:      "Test Service",
		Urgency:      "high",
		Created:      "2025-01-01T00:00:00Z",
		Status:       "triggered",
		Acknowledged: []string{"SRE User"},
		Alerts: []alertSummary{
			{
				ID:      "ALERT001",
				Name:    "ClusterOperatorDown",
				Link:    "https://example.com/sop/cluster-operator-down",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT001",
				Service: "Alert Service",
				Created: "2025-01-01T00:00:00Z",
				Status:  "triggered",
				Cluster: "abc-123-def",
			},
		},
	}

	result := renderTestTemplate(t, summary)

	// SOP should render as a markdown link using ToLink
	assert.Contains(t, result, "[SOP](https://example.com/sop/cluster-operator-down)")

	// Alert PD URL should render as a markdown link using ToLink
	assert.Contains(t, result, "[ALERT001](https://example.pagerduty.com/alerts/ALERT001)")
}

func TestIncidentTemplate_AlertRendersSOPNoneWhenMissing(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC002",
		Title:        "Test Incident No SOP",
		HTMLURL:      "https://example.pagerduty.com/incidents/INC002",
		Service:      "Test Service",
		Urgency:      "high",
		Created:      "2025-01-01T00:00:00Z",
		Status:       "triggered",
		Acknowledged: []string{"SRE User"},
		Alerts: []alertSummary{
			{
				ID:      "ALERT002",
				Name:    "SomeAlert",
				Link:    "",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT002",
				Service: "Alert Service",
				Created: "2025-01-01T00:00:00Z",
				Status:  "triggered",
				Cluster: "xyz-789",
			},
		},
	}

	result := renderTestTemplate(t, summary)

	// When Link is empty, SOP should show _none_
	assert.Contains(t, result, "_none_")
	// Should NOT contain an empty SOP link
	assert.NotContains(t, result, "[SOP]()")
}

func TestIncidentTemplate_NoDetailsSection(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC003",
		Title:        "Test Incident",
		HTMLURL:      "https://example.pagerduty.com/incidents/INC003",
		Service:      "Test Service",
		Urgency:      "high",
		Created:      "2025-01-01T00:00:00Z",
		Status:       "triggered",
		Acknowledged: []string{"SRE User"},
		Alerts: []alertSummary{
			{
				ID:      "ALERT003",
				Name:    "TestAlert",
				Link:    "https://example.com/sop",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT003",
				Service: "Alert Service",
				Created: "2025-01-01T00:00:00Z",
				Status:  "triggered",
				Cluster: "cluster-id",
			},
		},
	}

	result := renderTestTemplate(t, summary)

	// The rendered output should NOT contain a "Details" section
	assert.NotContains(t, result, "Details")
}

func TestIncidentTemplate_AlertNameAsHeading(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC004",
		Title:        "Test Incident",
		HTMLURL:      "https://example.pagerduty.com/incidents/INC004",
		Service:      "Test Service",
		Urgency:      "high",
		Created:      "2025-01-01T00:00:00Z",
		Status:       "triggered",
		Acknowledged: []string{"SRE User"},
		Alerts: []alertSummary{
			{
				ID:      "ALERT004",
				Name:    "KubePersistentVolumeFillingUp",
				Link:    "https://example.com/sop",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT004",
				Service: "Alert Service",
				Created: "2025-01-01T00:00:00Z",
				Status:  "triggered",
				Cluster: "cluster-id",
			},
		},
	}

	result := renderTestTemplate(t, summary)

	// Alert name should appear as a ### heading
	assert.Contains(t, result, "### KubePersistentVolumeFillingUp (triggered)")
}

func TestIncidentTemplate_AlertWithEmptyName(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC005",
		Title:        "Test Incident",
		HTMLURL:      "https://example.pagerduty.com/incidents/INC005",
		Service:      "Test Service",
		Urgency:      "high",
		Created:      "2025-01-01T00:00:00Z",
		Status:       "triggered",
		Acknowledged: []string{"SRE User"},
		Alerts: []alertSummary{
			{
				ID:      "ALERT005",
				Name:    "",
				Link:    "",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT005",
				Service: "Alert Service",
				Created: "2025-01-01T00:00:00Z",
				Status:  "triggered",
				Cluster: "",
			},
		},
	}

	result := renderTestTemplate(t, summary)

	// Should still render gracefully with empty fields
	// The heading should fall back to the alert ID when Name is empty
	assert.Contains(t, result, "### ALERT005 (triggered)")
	// Should contain _none_ for SOP
	assert.Contains(t, result, "_none_")
}

func TestSummarizeAlerts_NoDetailsField(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			APIObject: pagerduty.APIObject{
				ID:      "ALERT001",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT001",
			},
			Service: pagerduty.APIObject{Summary: "Test Service"},
			Status:  "triggered",
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "abc-123",
					"alert_name": "TestAlert",
					"link":       "https://example.com/sop",
					"extra_key":  "extra_value",
				},
			},
			Incident: pagerduty.APIReference{ID: "INC001"},
		},
	}

	result := summarizeAlerts(alerts)

	assert.Len(t, result, 1)
	assert.Equal(t, "ALERT001", result[0].ID)
	assert.Equal(t, "TestAlert", result[0].Name)
	assert.Equal(t, "abc-123", result[0].Cluster)
	assert.Equal(t, "https://example.com/sop", result[0].Link)
}
