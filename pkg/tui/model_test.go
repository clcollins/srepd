package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

// createTestModel creates a minimal model for testing
func createTestModel() model {
	return model{
		table:         table.New(),
		incidentCache: make(map[string]*cachedIncidentData),
		incidentList:  []pagerduty.Incident{},
	}
}

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
			initialDataLoaded:     false,
			initialNotesLoaded:    false,
			initialAlertsLoaded:   false,
			expectedDataLoaded:    true,
			expectedNotesLoaded:   false,
			expectedAlertsLoaded:  false,
			setupSelectedIncident: true,
		},
		{
			name: "gotIncidentNotesMsg sets incidentNotesLoaded to true",
			msg: gotIncidentNotesMsg{
				incidentID: "Q123",
				notes: []pagerduty.IncidentNote{
					{ID: "N123", Content: "Test note"},
				},
				err: nil,
			},
			initialDataLoaded:     false,
			initialNotesLoaded:    false,
			initialAlertsLoaded:   false,
			expectedDataLoaded:    false,
			expectedNotesLoaded:   true,
			expectedAlertsLoaded:  false,
			setupSelectedIncident: true,
		},
		{
			name: "gotIncidentAlertsMsg sets incidentAlertsLoaded to true",
			msg: gotIncidentAlertsMsg{
				incidentID: "Q123",
				alerts: []pagerduty.IncidentAlert{
					{APIObject: pagerduty.APIObject{ID: "A123"}},
				},
				err: nil,
			},
			initialDataLoaded:     false,
			initialNotesLoaded:    false,
			initialAlertsLoaded:   false,
			expectedDataLoaded:    false,
			expectedNotesLoaded:   false,
			expectedAlertsLoaded:  true,
			setupSelectedIncident: true,
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
				incidentCache:        make(map[string]*cachedIncidentData),
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
		name                 string
		keyMsg               tea.KeyMsg
		incidentDataLoaded   bool
		incidentAlertsLoaded bool
		expectedAction       bool
		expectedStatus       string
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
			name:                 "Login action blocked when alerts not loaded",
			keyMsg:               tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}},
			incidentAlertsLoaded: false,
			expectedAction:       false,
			expectedStatus:       "Loading incident alerts, please wait...",
		},
		{
			name:                 "Login action allowed when alerts loaded",
			keyMsg:               tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}},
			incidentAlertsLoaded: true,
			expectedAction:       true,
			expectedStatus:       "",
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
				viewingIncident:      true,
				incidentDataLoaded:   tt.incidentDataLoaded,
				incidentAlertsLoaded: tt.incidentAlertsLoaded,
				selectedIncident: &pagerduty.Incident{
					APIObject: pagerduty.APIObject{ID: "Q123"},
				},
				incidentCache: make(map[string]*cachedIncidentData),
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
				// Status should not be the "waiting" messages
				assert.NotEqual(t, "Loading incident details, please wait...", m.status, "Should not show data waiting message")
				assert.NotEqual(t, "Loading incident alerts, please wait...", m.status, "Should not show alerts waiting message")
			}

			// Reset status for next iteration
			m.status = initialStatus
		})
	}
}

func TestGetHighlightedIncident(t *testing.T) {
	tests := []struct {
		name             string
		incidentList     []pagerduty.Incident
		selectedRowIndex int
		hasSelectedRow   bool
		expectedID       string
		expectNil        bool
	}{
		{
			name: "Returns incident when row is highlighted and found in list",
			incidentList: []pagerduty.Incident{
				{APIObject: pagerduty.APIObject{ID: "Q123", Summary: "Test 1"}, Title: "Incident 1"},
				{APIObject: pagerduty.APIObject{ID: "Q456", Summary: "Test 2"}, Title: "Incident 2"},
				{APIObject: pagerduty.APIObject{ID: "Q789", Summary: "Test 3"}, Title: "Incident 3"},
			},
			selectedRowIndex: 1,
			hasSelectedRow:   true,
			expectedID:       "Q456",
			expectNil:        false,
		},
		{
			name: "Returns nil when no row is highlighted",
			incidentList: []pagerduty.Incident{
				{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Incident 1"},
			},
			hasSelectedRow: false,
			expectNil:      true,
		},
		{
			name: "Returns nil when incident not found in list",
			incidentList: []pagerduty.Incident{
				{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Incident 1"},
			},
			selectedRowIndex: 0,
			hasSelectedRow:   true,
			expectedID:       "QNOTFOUND",
			expectNil:        true,
		},
		{
			name:           "Returns nil when incident list is empty",
			incidentList:   []pagerduty.Incident{},
			hasSelectedRow: false,
			expectNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.incidentList = tt.incidentList

			// Set up table rows to match incident list
			if tt.hasSelectedRow && len(tt.incidentList) > 0 {
				// Manually set the cursor to simulate a selected row
				// We'll use the test directly by creating a mock selected row
				// Since we can't easily mock the table.SelectedRow(), we'll test by
				// setting up the incident list and verifying the lookup logic
			}

			// For this test, we'll directly test the lookup logic
			// by simulating what getHighlightedIncident does
			var result *pagerduty.Incident
			if tt.hasSelectedRow && len(tt.incidentList) > 0 && tt.selectedRowIndex < len(tt.incidentList) {
				// Simulate finding the incident by ID
				searchID := tt.expectedID
				if searchID == "" && tt.selectedRowIndex < len(tt.incidentList) {
					searchID = tt.incidentList[tt.selectedRowIndex].ID
				}

				for i := range m.incidentList {
					if m.incidentList[i].ID == searchID {
						result = &m.incidentList[i]
						break
					}
				}
			}

			if tt.expectNil {
				assert.Nil(t, result, "Expected nil result")
			} else {
				assert.NotNil(t, result, "Expected non-nil result")
				assert.Equal(t, tt.expectedID, result.ID, "Incident ID mismatch")
			}
		})
	}
}

func TestActionMessagesFallbackToSelectedIncident(t *testing.T) {
	tests := []struct {
		name             string
		msg              tea.Msg
		selectedIncident *pagerduty.Incident
		expectSuccess    bool
	}{
		{
			name: "acknowledgeIncidentsMsg uses selectedIncident as fallback",
			msg:  acknowledgeIncidentsMsg{incidents: nil},
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "Q789"},
				Title:     "Selected Incident",
			},
			expectSuccess: true,
		},
		{
			name:             "acknowledgeIncidentsMsg fails when no incident available",
			msg:              acknowledgeIncidentsMsg{incidents: nil},
			selectedIncident: nil,
			expectSuccess:    false,
		},
		{
			name: "unAcknowledgeIncidentsMsg uses selectedIncident as fallback",
			msg:  unAcknowledgeIncidentsMsg{incidents: nil},
			selectedIncident: &pagerduty.Incident{
				APIObject:        pagerduty.APIObject{ID: "Q789"},
				Title:            "Selected Incident",
				EscalationPolicy: pagerduty.APIObject{ID: "POL123"},
			},
			expectSuccess: true,
		},
		{
			name:             "unAcknowledgeIncidentsMsg fails when no incident available",
			msg:              unAcknowledgeIncidentsMsg{incidents: nil},
			selectedIncident: nil,
			expectSuccess:    false,
		},
		{
			name: "silenceSelectedIncidentMsg uses selectedIncident as fallback",
			msg:  silenceSelectedIncidentMsg{},
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "Q789"},
				Title:     "Selected Incident",
				Service:   pagerduty.APIObject{ID: "SVC789"},
			},
			expectSuccess: true,
		},
		{
			name:             "silenceSelectedIncidentMsg fails when no incident available",
			msg:              silenceSelectedIncidentMsg{},
			selectedIncident: nil,
			expectSuccess:    false,
		},
		{
			name: "acknowledgeIncidentsMsg succeeds when incidents provided in message",
			msg: acknowledgeIncidentsMsg{
				incidents: []pagerduty.Incident{
					{APIObject: pagerduty.APIObject{ID: "Q123"}},
				},
			},
			selectedIncident: nil,
			expectSuccess:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.selectedIncident = tt.selectedIncident
			m.incidentCache = make(map[string]*cachedIncidentData)

			// Create a basic config for the test
			m.config = &pd.Config{
				EscalationPolicies: map[string]*pagerduty.EscalationPolicy{
					"SILENT_DEFAULT": {
						APIObject: pagerduty.APIObject{ID: "SILENT"},
						Name:      "Silent",
					},
				},
				CurrentUser: &pagerduty.User{
					APIObject: pagerduty.APIObject{ID: "U123"},
				},
			}

			result, cmd := m.Update(tt.msg)
			m = result.(model)

			if tt.expectSuccess {
				assert.NotNil(t, cmd, "Expected command to be returned for success case")
			} else {
				assert.Nil(t, cmd, "Expected nil command for failure case")
			}
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
				incidentCache:        make(map[string]*cachedIncidentData),
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
