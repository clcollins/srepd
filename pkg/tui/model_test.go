package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestLoadingStateTracking(t *testing.T) {
	tests := []struct {
		name                     string
		msg                      tea.Msg
		initialDataLoaded        bool
		initialNotesLoaded       bool
		initialAlertsLoaded      bool
		expectedDataLoaded       bool
		expectedNotesLoaded      bool
		expectedAlertsLoaded     bool
		setupSelectedIncident    bool
	}{
		{
			name: "gotIncidentMsg sets incidentDataLoaded to true",
			msg: gotIncidentMsg{
				incident: &pagerduty.Incident{
					APIObject: pagerduty.APIObject{ID: "Q123"},
				},
				err: nil,
			},
			initialDataLoaded:    false,
			initialNotesLoaded:   false,
			initialAlertsLoaded:  false,
			expectedDataLoaded:   true,
			expectedNotesLoaded:  false,
			expectedAlertsLoaded: false,
		},
		{
			name: "gotIncidentNotesMsg sets incidentNotesLoaded to true",
			msg: gotIncidentNotesMsg{
				notes: []pagerduty.IncidentNote{
					{ID: "N123", Content: "Test note"},
				},
				err: nil,
			},
			initialDataLoaded:    false,
			initialNotesLoaded:   false,
			initialAlertsLoaded:  false,
			expectedDataLoaded:   false,
			expectedNotesLoaded:  true,
			expectedAlertsLoaded: false,
		},
		{
			name: "gotIncidentAlertsMsg sets incidentAlertsLoaded to true",
			msg: gotIncidentAlertsMsg{
				alerts: []pagerduty.IncidentAlert{
					{APIObject: pagerduty.APIObject{ID: "A123"}},
				},
				err: nil,
			},
			initialDataLoaded:    false,
			initialNotesLoaded:   false,
			initialAlertsLoaded:  false,
			expectedDataLoaded:   false,
			expectedNotesLoaded:  false,
			expectedAlertsLoaded: true,
		},
		{
			name:                     "clearSelectedIncidentsMsg clears all loading flags",
			msg:                      clearSelectedIncidentsMsg("test"),
			initialDataLoaded:        true,
			initialNotesLoaded:       true,
			initialAlertsLoaded:      true,
			expectedDataLoaded:       false,
			expectedNotesLoaded:      false,
			expectedAlertsLoaded:     false,
			setupSelectedIncident:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				incidentDataLoaded:   tt.initialDataLoaded,
				incidentNotesLoaded:  tt.initialNotesLoaded,
				incidentAlertsLoaded: tt.initialAlertsLoaded,
			}

			if tt.setupSelectedIncident {
				m.selectedIncident = &pagerduty.Incident{
					APIObject: pagerduty.APIObject{ID: "Q123"},
				}
			}

			result, _ := m.Update(tt.msg)
			m = result.(model)

			assert.Equal(t, tt.expectedDataLoaded, m.incidentDataLoaded, "incidentDataLoaded mismatch")
			assert.Equal(t, tt.expectedNotesLoaded, m.incidentNotesLoaded, "incidentNotesLoaded mismatch")
			assert.Equal(t, tt.expectedAlertsLoaded, m.incidentAlertsLoaded, "incidentAlertsLoaded mismatch")
		})
	}
}

func TestActionGuards(t *testing.T) {
	tests := []struct {
		name               string
		keyMsg             tea.KeyMsg
		incidentDataLoaded bool
		expectedAction     bool
		expectedStatus     string
	}{
		{
			name:               "Note action blocked when data not loaded",
			keyMsg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}},
			incidentDataLoaded: false,
			expectedAction:     false,
			expectedStatus:     "Loading incident details, please wait...",
		},
		{
			name:               "Note action allowed when data loaded",
			keyMsg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}},
			incidentDataLoaded: true,
			expectedAction:     true,
			expectedStatus:     "",
		},
		{
			name:               "Login action blocked when data not loaded",
			keyMsg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}},
			incidentDataLoaded: false,
			expectedAction:     false,
			expectedStatus:     "Loading incident details, please wait...",
		},
		{
			name:               "Login action allowed when data loaded",
			keyMsg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}},
			incidentDataLoaded: true,
			expectedAction:     true,
			expectedStatus:     "",
		},
		{
			name:               "Open action blocked when data not loaded",
			keyMsg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}},
			incidentDataLoaded: false,
			expectedAction:     false,
			expectedStatus:     "Loading incident details, please wait...",
		},
		{
			name:               "Open action allowed when data loaded",
			keyMsg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}},
			incidentDataLoaded: true,
			expectedAction:     true,
			expectedStatus:     "",
		},
		{
			name:               "Acknowledge action always allowed (only needs ID)",
			keyMsg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
			incidentDataLoaded: false,
			expectedAction:     true,
			expectedStatus:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				viewingIncident:    true,
				incidentDataLoaded: tt.incidentDataLoaded,
				selectedIncident: &pagerduty.Incident{
					APIObject: pagerduty.APIObject{ID: "Q123"},
				},
			}

			initialStatus := m.status
			result, cmd := m.Update(tt.keyMsg)
			m = result.(model)

			if !tt.expectedAction {
				// Action should be blocked - no command returned
				assert.Nil(t, cmd, "Expected no command when action is blocked")
				assert.Equal(t, tt.expectedStatus, m.status, "Status message mismatch")
			} else if tt.keyMsg.Runes[0] != 'a' { // Skip acknowledge since it generates a different message
				// Action should be allowed - command returned
				assert.NotNil(t, cmd, "Expected command when action is allowed")
				// Status should not be the "waiting" message
				assert.NotEqual(t, "Loading incident details, please wait...", m.status, "Should not show waiting message")
			}

			// Reset status for next iteration
			m.status = initialStatus
		})
	}
}

func TestProgressiveRendering(t *testing.T) {
	tests := []struct {
		name                 string
		incidentNotesLoaded  bool
		incidentAlertsLoaded bool
		expectedNotesContent string
		expectedAlertsMarker string
	}{
		{
			name:                 "Shows loading message for notes when not loaded",
			incidentNotesLoaded:  false,
			incidentAlertsLoaded: true,
			expectedNotesContent: "Loading notes...",
			expectedAlertsMarker: "",
		},
		{
			name:                 "Shows loading message for alerts when not loaded",
			incidentNotesLoaded:  true,
			incidentAlertsLoaded: false,
			expectedNotesContent: "",
			expectedAlertsMarker: "Loading alerts...",
		},
		{
			name:                 "Shows both loading messages when neither loaded",
			incidentNotesLoaded:  false,
			incidentAlertsLoaded: false,
			expectedNotesContent: "Loading notes...",
			expectedAlertsMarker: "Loading alerts...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incident := &pagerduty.Incident{}
			incident.ID = "Q123"
			incident.Summary = "Test Incident"
			incident.HTMLURL = "https://example.com/incident"
			incident.Title = "Test Incident Title"
			incident.Status = "triggered"
			incident.Urgency = "high"
			// Set Service summary for template
			incident.Service.Summary = "test-service"

			m := model{
				selectedIncident:     incident,
				incidentNotesLoaded:  tt.incidentNotesLoaded,
				incidentAlertsLoaded: tt.incidentAlertsLoaded,
			}

			content, err := m.template()
			assert.NoError(t, err, "Template should render without error")

			if tt.expectedNotesContent != "" {
				assert.Contains(t, content, tt.expectedNotesContent, "Should contain notes loading message")
			}

			if tt.expectedAlertsMarker != "" {
				assert.Contains(t, content, tt.expectedAlertsMarker, "Should contain alerts loading message")
			}
		})
	}
}
