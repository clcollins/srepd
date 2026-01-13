package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
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
		name        string
		autoRefresh bool
		autoAck     bool
		expected    string
	}{
		{
			name:        "both enabled",
			autoRefresh: true,
			autoAck:     true,
			expected:    "Watching for updates...  [auto-acknowledge]",
		},
		{
			name:        "auto-refresh enabled, auto-ack disabled",
			autoRefresh: true,
			autoAck:     false,
			expected:    "Watching for updates... ",
		},
		{
			name:        "auto-refresh disabled",
			autoRefresh: false,
			autoAck:     false,
			expected:    "Watching for updates...  [PAUSED]",
		},
		{
			name:        "auto-refresh disabled, auto-ack enabled (paused takes precedence)",
			autoRefresh: false,
			autoAck:     true,
			expected:    "Watching for updates...  [PAUSED]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := refreshArea(test.autoRefresh, test.autoAck)
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

func TestAddNoteTemplate(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		title       string
		service     string
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
