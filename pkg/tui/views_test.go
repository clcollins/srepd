package tui

import (
	"bytes"
	"html/template"
	"testing"

	"charm.land/glamour/v2"
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
			result := statusArea(test.input, test.showSpinner, test.spinnerView, DefaultTheme().Text)
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
	result := summarizeAlerts([]pagerduty.IncidentAlert{}, nil)

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
	result := summarizeAlerts(alerts, nil)

	assert.Len(t, result, 1, "should return exactly one alert summary")
	assert.Equal(t, "ALERT_NIL_BODY", result[0].ID, "alert ID should be preserved")
	assert.Equal(t, "", result[0].Name, "name should be empty when body is nil")
	assert.Equal(t, "", result[0].Link, "link should be empty when body is nil")
	assert.Equal(t, "", result[0].Cluster, "cluster should be empty when body is nil")
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

// renderTestTemplate is a helper that executes the detailsTabTemplate with funcMap
// against the given incidentSummary and returns the rendered string.
func renderTestTemplate(t *testing.T, summary incidentSummary) string {
	t.Helper()
	tmpl, err := template.New("details").Funcs(funcMap).Parse(detailsTabTemplate)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, summary)
	require.NoError(t, err)

	return buf.String()
}

func renderAlertsTestContent(t *testing.T, summary incidentSummary) string {
	t.Helper()
	m := model{
		selectedIncident:       &pagerduty.Incident{},
		incidentAlertsLoaded:   true,
		incidentNotesLoaded:    true,
		activeTab:              tabAlerts,
		selectedIncidentAlerts: []pagerduty.IncidentAlert{},
		incidentCache:          make(map[string]*cachedIncidentData),
	}
	result, err := m.renderAlertsTab(summary)
	require.NoError(t, err)
	return result
}

func renderNotesTestContent(t *testing.T, summary incidentSummary) string {
	t.Helper()
	m := model{
		selectedIncident:      &pagerduty.Incident{},
		incidentAlertsLoaded:  true,
		incidentNotesLoaded:   true,
		activeTab:             tabNotes,
		selectedIncidentNotes: []pagerduty.IncidentNote{},
		incidentCache:         make(map[string]*cachedIncidentData),
	}
	result, err := m.renderNotesTab(summary)
	require.NoError(t, err)
	return result
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

	result := renderAlertsTestContent(t, summary)

	// SOP should render as a markdown link with filename
	assert.Contains(t, result, "[cluster-operator-down](https://example.com/sop/cluster-operator-down)")

	// Alert name should render as a markdown link to the alert
	assert.Contains(t, result, "[ClusterOperatorDown](https://example.pagerduty.com/alerts/ALERT001)")
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

	result := renderAlertsTestContent(t, summary)

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

	result := renderAlertsTestContent(t, summary)

	// Alert name should appear as a link
	assert.Contains(t, result, "[KubePersistentVolumeFillingUp](https://example.pagerduty.com/alerts/ALERT004)")
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

	result := renderAlertsTestContent(t, summary)

	// Should fall back to alert ID as link text when Name is empty
	assert.Contains(t, result, "[ALERT005](https://example.pagerduty.com/alerts/ALERT005)")
	// Should contain _none_ for SOP
	assert.Contains(t, result, "_none_")
}

func TestIncidentTemplate_BoldIncidentID(t *testing.T) {
	tests := []struct {
		name     string
		summary  incidentSummary
		expected string
	}{
		{
			name: "incident ID is bold without priority",
			summary: incidentSummary{
				ID:           "INC001",
				Title:        "Test",
				HTMLURL:      "https://example.com",
				Service:      "Svc",
				Urgency:      "high",
				Created:      "2025-01-01",
				Status:       "triggered",
				Acknowledged: []string{"User"},
			},
			expected: "# INC001 - triggered",
		},
		{
			name: "incident ID shown with priority",
			summary: incidentSummary{
				ID:           "INC002",
				Title:        "Test",
				HTMLURL:      "https://example.com",
				Service:      "Svc",
				Urgency:      "high",
				Created:      "2025-01-01",
				Status:       "triggered",
				Priority:     "P1",
				Acknowledged: []string{"User"},
			},
			expected: "# P1 INC002 - triggered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTestTemplate(t, tt.summary)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestDetailsTab_Template(t *testing.T) {
	t.Run("details tab renders metadata without alerts or notes", func(t *testing.T) {
		summary := incidentSummary{
			ID:           "INC010",
			Title:        "Test Incident for Details Tab",
			HTMLURL:      "https://example.pagerduty.com/incidents/INC010",
			Service:      "Test Service",
			Urgency:      "high",
			Created:      "2025-01-01T00:00:00Z",
			Status:       "triggered",
			Acknowledged: []string{"SRE User"},
			Alerts: []alertSummary{
				{
					ID:      "ALERT010",
					Name:    "SomeAlert",
					HTMLURL: "https://example.pagerduty.com/alerts/ALERT010",
					Service: "Alert Service",
					Created: "2025-01-01T00:00:00Z",
					Status:  "triggered",
					Cluster: "cluster-id",
				},
			},
			Notes: []noteSummary{
				{
					ID:      "NOTE010",
					User:    "Someone",
					Content: "A note",
					Created: "2025-01-01T00:00:00Z",
				},
			},
		}

		tmpl, err := template.New("details").Funcs(funcMap).Parse(detailsTabTemplate)
		require.NoError(t, err)

		var buf bytes.Buffer
		err = tmpl.Execute(&buf, summary)
		require.NoError(t, err)
		result := buf.String()

		// Should contain incident metadata
		assert.Contains(t, result, "INC010")
		assert.Contains(t, result, "Test Incident for Details Tab")
		assert.Contains(t, result, "Test Service")
		assert.Contains(t, result, "high")
		assert.Contains(t, result, "triggered")

		// Should NOT contain alert or note content
		assert.NotContains(t, result, "ALERT010")
		assert.NotContains(t, result, "SomeAlert")
		assert.NotContains(t, result, "NOTE010")
		assert.NotContains(t, result, "A note")
	})
}

func TestAlertTab_ShowsSingleAlert(t *testing.T) {
	t.Run("alert tab template renders a single alert", func(t *testing.T) {
		data := struct {
			Alert alertSummary
			Index int
			Total int
		}{
			Alert: alertSummary{
				ID:      "ALERT020",
				Name:    "KubePodNotReady",
				Link:    "https://example.com/sop/kube-pod-not-ready",
				HTMLURL: "https://example.pagerduty.com/alerts/ALERT020",
				Service: "Alert Service",
				Created: "2025-06-01T10:00:00Z",
				Status:  "triggered",
				Cluster: "my-cluster-id",
			},
			Index: 0,
			Total: 5,
		}

		tmpl, err := template.New("alert").Funcs(funcMap).Parse(alertTabTemplate)
		require.NoError(t, err)

		var buf bytes.Buffer
		err = tmpl.Execute(&buf, data)
		require.NoError(t, err)
		result := buf.String()

		assert.Contains(t, result, "[KubePodNotReady](https://example.pagerduty.com/alerts/ALERT020)")
		assert.Contains(t, result, "my-cluster-id")
		assert.Contains(t, result, "1/5")
		assert.NotContains(t, result, "ALERT999")
	})
}

func TestNoteTab_ShowsSingleNote(t *testing.T) {
	t.Run("note tab template renders a single note", func(t *testing.T) {
		data := struct {
			Note  noteSummary
			Index int
			Total int
		}{
			Note: noteSummary{
				ID:      "NOTE020",
				User:    "Jane SRE",
				Content: "Investigated and found root cause",
				Created: "2025-06-01T12:00:00Z",
			},
			Index: 1,
			Total: 3,
		}

		tmpl, err := template.New("note").Funcs(funcMap).Parse(noteTabTemplate)
		require.NoError(t, err)

		var buf bytes.Buffer
		err = tmpl.Execute(&buf, data)
		require.NoError(t, err)
		result := buf.String()

		assert.Contains(t, result, "Jane SRE")
		assert.Contains(t, result, "Investigated and found root cause")
		assert.Contains(t, result, "2/3") // 1-based display of index 1 out of 3
	})
}

func TestTabHeader_Rendering(t *testing.T) {
	tests := []struct {
		name                string
		activeTab           int
		alertCount          int
		noteCount           int
		alertsLoading       bool
		notesLoading        bool
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name:       "Details tab active shows highlighted Details",
			activeTab:  0,
			alertCount: 5,
			noteCount:  3,
			expectedContains: []string{
				"Details",
				"Alerts (5)",
				"Notes (3)",
			},
		},
		{
			name:       "Alerts tab active shows count",
			activeTab:  1,
			alertCount: 10,
			noteCount:  0,
			expectedContains: []string{
				"Details",
				"Alerts (10)",
				"Notes (0)",
			},
		},
		{
			name:          "Loading alerts shows loading indicator",
			activeTab:     1,
			alertCount:    0,
			noteCount:     2,
			alertsLoading: true,
			expectedContains: []string{
				"Alerts (...)",
			},
		},
		{
			name:         "Loading notes shows loading indicator",
			activeTab:    2,
			alertCount:   1,
			noteCount:    0,
			notesLoading: true,
			expectedContains: []string{
				"Notes (...)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.activeTab = tt.activeTab
			m.incidentAlertsLoaded = !tt.alertsLoading
			m.incidentNotesLoaded = !tt.notesLoading
			for i := 0; i < tt.alertCount; i++ {
				m.selectedIncidentAlerts = append(m.selectedIncidentAlerts, pagerduty.IncidentAlert{})
			}
			for i := 0; i < tt.noteCount; i++ {
				m.selectedIncidentNotes = append(m.selectedIncidentNotes, pagerduty.IncidentNote{})
			}
			result := m.renderTabBar()

			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected, "tab header should contain %q", expected)
			}
			for _, notExpected := range tt.expectedNotContains {
				assert.NotContains(t, result, notExpected, "tab header should not contain %q", notExpected)
			}
		})
	}
}

func TestIncidentTemplate_NotesIndented(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC001",
		Title:        "Test",
		HTMLURL:      "https://example.com",
		Service:      "Svc",
		Urgency:      "high",
		Created:      "2025-01-01",
		Status:       "triggered",
		Acknowledged: []string{"User"},
		Notes: []noteSummary{
			{
				ID:      "N1",
				User:    "John Doe",
				Content: "This is a note",
				Created: "2025-01-02",
			},
		},
	}

	result := renderNotesTestContent(t, summary)

	assert.Contains(t, result, "> This is a note")
	assert.Contains(t, result, "-- John Doe @ 2025-01-02")
}

func TestIncidentTemplate_AlertDetailsIndented(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC001",
		Title:        "Test",
		HTMLURL:      "https://example.com",
		Service:      "Svc",
		Urgency:      "high",
		Created:      "2025-01-01",
		Status:       "triggered",
		Acknowledged: []string{"User"},
		Alerts: []alertSummary{
			{
				ID:      "ALERT001",
				Name:    "TestAlert",
				Link:    "https://example.com/sop",
				HTMLURL: "https://example.com/alert",
				Service: "AlertSvc",
				Created: "2025-01-01",
				Status:  "triggered",
				Cluster: "abc-123",
			},
		},
	}

	result := renderAlertsTestContent(t, summary)

	assert.Contains(t, result, "* Cluster: abc-123")
	assert.Contains(t, result, "* Service: AlertSvc")
	assert.Contains(t, result, "* Created: 2025-01-01")
}

func TestIncidentTemplate_SpacingBetweenAlerts(t *testing.T) {
	summary := incidentSummary{
		ID:           "INC001",
		Title:        "Test",
		HTMLURL:      "https://example.com",
		Service:      "Svc",
		Urgency:      "high",
		Created:      "2025-01-01",
		Status:       "triggered",
		Acknowledged: []string{"User"},
		Alerts: []alertSummary{
			{
				ID:      "ALERT001",
				Name:    "FirstAlert",
				HTMLURL: "https://example.com/alert1",
				Service: "Svc1",
				Created: "2025-01-01",
				Status:  "triggered",
				Cluster: "abc-123",
			},
			{
				ID:      "ALERT002",
				Name:    "SecondAlert",
				HTMLURL: "https://example.com/alert2",
				Service: "Svc2",
				Created: "2025-01-02",
				Status:  "triggered",
				Cluster: "def-456",
			},
		},
	}

	result := renderAlertsTestContent(t, summary)

	// Both alerts should be present as links
	assert.Contains(t, result, "[FirstAlert]")
	assert.Contains(t, result, "[SecondAlert]")

	// There should be a horizontal rule (---) between alerts
	assert.Contains(t, result, "---")
	assert.Contains(t, result, "* Created: 2025-01-01\n\n---\n")
	assert.Contains(t, result, "---\n\n### Alert 2/2")
}

func TestIncidentViewerNoBorder(t *testing.T) {
	vp := newIncidentViewer()

	// The viewport should not have a border style set
	// GetBorderTop returns false when no border is configured
	assert.False(t, vp.Style.GetBorderTop(), "viewport should not have a top border")
	assert.False(t, vp.Style.GetBorderBottom(), "viewport should not have a bottom border")
	assert.False(t, vp.Style.GetBorderLeft(), "viewport should not have a left border")
	assert.False(t, vp.Style.GetBorderRight(), "viewport should not have a right border")
}

func TestRenderTabBar(t *testing.T) {
	tests := []struct {
		name             string
		activeTab        int
		alertCount       int
		noteCount        int
		alertsLoading    bool
		notesLoading     bool
		expectedContains []string
	}{
		{
			name:             "Details tab active",
			activeTab:        tabDetails,
			alertCount:       3,
			noteCount:        2,
			expectedContains: []string{"Details", "Alerts (3)", "Notes (2)"},
		},
		{
			name:             "Alerts loading shows ellipsis",
			activeTab:        tabAlerts,
			alertCount:       0,
			noteCount:        1,
			alertsLoading:    true,
			expectedContains: []string{"Alerts (...)", "Notes (1)"},
		},
		{
			name:             "Notes loading shows ellipsis",
			activeTab:        tabNotes,
			alertCount:       2,
			noteCount:        0,
			notesLoading:     true,
			expectedContains: []string{"Alerts (2)", "Notes (...)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.activeTab = tt.activeTab
			m.incidentAlertsLoaded = !tt.alertsLoading
			m.incidentNotesLoaded = !tt.notesLoading
			for i := 0; i < tt.alertCount; i++ {
				m.selectedIncidentAlerts = append(m.selectedIncidentAlerts, pagerduty.IncidentAlert{})
			}
			for i := 0; i < tt.noteCount; i++ {
				m.selectedIncidentNotes = append(m.selectedIncidentNotes, pagerduty.IncidentNote{})
			}
			result := m.renderTabBar()

			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected, "tab bar should contain %q", expected)
			}
		})
	}
}

func TestRenderTabContent_DetailsTab(t *testing.T) {
	t.Run("details tab shows incident metadata", func(t *testing.T) {
		incident := &pagerduty.Incident{}
		incident.ID = "Q999"
		incident.HTMLURL = "https://example.com/incidents/Q999"
		incident.Title = "Tab Test Incident"
		incident.Status = "triggered"
		incident.Urgency = "high"
		incident.Service.Summary = "test-service"

		m := model{
			selectedIncident:     incident,
			activeTab:            tabDetails,
			incidentNotesLoaded:  true,
			incidentAlertsLoaded: true,
			incidentCache:        make(map[string]*cachedIncidentData),
		}

		content, err := m.renderTabContent()
		assert.NoError(t, err)
		assert.Contains(t, content, "Q999")
		assert.Contains(t, content, "Tab Test Incident")
		assert.Contains(t, content, "test-service")
	})
}

func TestRenderTabContent_AlertsTab(t *testing.T) {
	t.Run("alerts tab shows all alerts", func(t *testing.T) {
		incident := &pagerduty.Incident{}
		incident.ID = "Q999"
		incident.Service.Summary = "svc"

		m := model{
			selectedIncident:     incident,
			activeTab:            tabAlerts,
			incidentAlertsLoaded: true,
			incidentNotesLoaded:  true,
			incidentCache:        make(map[string]*cachedIncidentData),
			selectedIncidentAlerts: []pagerduty.IncidentAlert{
				{
					APIObject: pagerduty.APIObject{ID: "A1", HTMLURL: "https://example.com/alerts/A1"},
					Service:   pagerduty.APIObject{Summary: "Alert Service"},
					Status:    "triggered",
				},
				{
					APIObject: pagerduty.APIObject{ID: "A2", HTMLURL: "https://example.com/alerts/A2"},
					Service:   pagerduty.APIObject{Summary: "Alert Service 2"},
					Status:    "resolved",
				},
			},
		}

		content, err := m.renderTabContent()
		assert.NoError(t, err)
		assert.Contains(t, content, "**A1**")
		assert.Contains(t, content, "**A2**")
		assert.Contains(t, content, "1/2")
		assert.Contains(t, content, "2/2")
	})
}

func TestRenderIncidentMarkdown_RendersContent(t *testing.T) {
	t.Run("renders markdown and returns non-empty result", func(t *testing.T) {
		m := createTestModel()

		content := "# Test Heading\n\nSome **bold** text"
		result, err := renderIncidentMarkdown(&m, content)

		assert.NoError(t, err, "should not error")
		assert.NotEmpty(t, result, "should return rendered content")
		assert.NotEqual(t, content, result, "should transform the input")
	})
}

func TestRenderIncidentMarkdown_WithRenderer(t *testing.T) {
	t.Run("returns rendered markdown content when renderer is available", func(t *testing.T) {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithWordWrap(100),
		)
		require.NoError(t, err, "should create renderer without error")

		m := &model{
			markdownRenderer: renderer,
		}

		content := "# Test Heading\n\nSome **bold** text"
		result, err := renderIncidentMarkdown(m, content)

		assert.NoError(t, err, "should not error with valid renderer")
		assert.NotEqual(t, content, result, "rendered content should differ from raw content")
		assert.Contains(t, result, "Test Heading", "rendered content should contain the heading text")
		assert.Contains(t, result, "bold", "rendered content should contain the bold text")
	})
}

func TestRenderIncidentMarkdown_EmptyContent(t *testing.T) {
	t.Run("handles empty content gracefully", func(t *testing.T) {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithWordWrap(100),
		)
		require.NoError(t, err, "should create renderer without error")

		m := &model{
			markdownRenderer: renderer,
		}

		result, err := renderIncidentMarkdown(m, "")

		assert.NoError(t, err, "should not error on empty content")
		// Glamour may add whitespace but should not error
		_ = result
	})
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

	result := summarizeAlerts(alerts, nil)

	assert.Len(t, result, 1)
	assert.Equal(t, "ALERT001", result[0].ID)
	assert.Equal(t, "TestAlert", result[0].Name)
	assert.Equal(t, "abc-123", result[0].Cluster)
	assert.Equal(t, "https://example.com/sop", result[0].Link)
}

func TestRenderIncidentMarkdown_PlainText(t *testing.T) {
	t.Run("plain text returns without error", func(t *testing.T) {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithWordWrap(100),
		)
		require.NoError(t, err)

		m := &model{
			markdownRenderer: renderer,
		}
		content := "Just plain text with no markdown formatting"
		result, err := renderIncidentMarkdown(m, content)

		assert.NoError(t, err, "should not error on plain text")
		assert.Contains(t, result, "Just plain text", "plain text content should be preserved")
	})
}

func TestRenderHeader_Layout(t *testing.T) {
	t.Run("header uses 4/6 and 2/6 proportional widths", func(t *testing.T) {
		windowSize.Width = 120

		m := createTestModel()
		m.status = "showing 5/5 incidents"

		result := m.renderHeader()

		assert.Contains(t, result, "showing 5/5 incidents", "header should contain status")
		assert.Contains(t, result, "Showing assigned to You", "header should contain assignee")
	})

	t.Run("header shows Team when teamMode is true", func(t *testing.T) {
		windowSize.Width = 120

		m := createTestModel()
		m.teamMode = true

		result := m.renderHeader()

		assert.Contains(t, result, "Showing assigned to Team")
	})
}

func TestRenderBottomStatus_Layout(t *testing.T) {
	t.Run("shows version string when no update available", func(t *testing.T) {
		windowSize.Width = 120

		m := createTestModel()
		m.updateAvailable = false

		result := m.renderBottomStatus()

		assert.Contains(t, result, versionString(), "should show version string")
	})

	t.Run("shows selected incident ID on left", func(t *testing.T) {
		windowSize.Width = 120

		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q123TEST"},
		}

		result := m.renderBottomStatus()

		assert.Contains(t, result, "Q123TEST", "should contain selected incident ID")
	})

	t.Run("shows three columns when update available", func(t *testing.T) {
		windowSize.Width = 120

		m := createTestModel()
		m.updateAvailable = true
		m.updateVersion = "v2.0.0"
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "QINC001"},
		}

		result := m.renderBottomStatus()

		assert.Contains(t, result, "QINC001", "left column should show incident ID")
		assert.Contains(t, result, "An update is available: v2.0.0", "center column should show update notice")
		assert.Contains(t, result, Version, "right column should show current version")
		assert.Contains(t, result, "v2.0.0", "right column should show new version")
	})

	t.Run("footer scales with window width", func(t *testing.T) {
		m := createTestModel()
		m.updateAvailable = true
		m.updateVersion = "v3.0.0"

		windowSize.Width = 80
		narrow := m.renderBottomStatus()

		windowSize.Width = 200
		wide := m.renderBottomStatus()

		assert.NotEqual(t, len(narrow), len(wide), "footer should have different widths at different terminal sizes")
		assert.Contains(t, narrow, "An update is available: v3.0.0")
		assert.Contains(t, wide, "An update is available: v3.0.0")
	})

	t.Run("empty when no incident selected and no update", func(t *testing.T) {
		windowSize.Width = 120

		m := createTestModel()

		result := m.renderBottomStatus()

		assert.Contains(t, result, versionString(), "should still show version")
		assert.NotContains(t, result, "An update is available", "should not show update notice")
	})
}
