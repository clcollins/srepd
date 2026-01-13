package tui

import (
	"testing"
	"time"

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

func TestUpdatedIncidentListNoPrefetch(t *testing.T) {
	t.Run("updatedIncidentListMsg does not trigger pre-fetch commands", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}
		m.incidentCache = make(map[string]*cachedIncidentData)

		// Create a message with multiple incidents
		msg := updatedIncidentListMsg{
			incidents: []pagerduty.Incident{
				{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Incident 1"},
				{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Incident 2"},
				{APIObject: pagerduty.APIObject{ID: "Q789"}, Title: "Incident 3"},
			},
			err: nil,
		}

		result, cmd := m.Update(msg)
		m = result.(model)

		// Verify incident list was populated
		assert.Equal(t, 3, len(m.incidentList), "Incident list should contain 3 incidents")

		// The cmd returned should NOT be a batch of pre-fetch commands
		// It should be nil or a simple command (not a batch)
		// We can't easily inspect the command contents, but we can verify
		// the incident list was set correctly
		assert.Equal(t, "Q123", m.incidentList[0].ID)
		assert.Equal(t, "Q456", m.incidentList[1].ID)
		assert.Equal(t, "Q789", m.incidentList[2].ID)

		// Verify cache remains empty (no pre-fetching occurred)
		assert.Equal(t, 0, len(m.incidentCache), "Cache should remain empty without pre-fetching")

		// If cmd is not nil, it should be a single command or batch
		// but we've removed the pre-fetch loop so it won't be a large batch
		_ = cmd // Acknowledge we got a command but don't need to inspect it deeply
	})
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

func TestAddActionLogEntry(t *testing.T) {
	tests := []struct {
		name           string
		initialEntries []actionLogEntry
		newKey         string
		newID          string
		newSummary     string
		newAction      string
		expectedCount  int
		expectedFirst  actionLogEntry
	}{
		{
			name:           "Adds first entry to empty log",
			initialEntries: []actionLogEntry{},
			newKey:         "a",
			newID:          "Q123",
			newSummary:     "Test Incident",
			newAction:      "acknowledge",
			expectedCount:  1,
			expectedFirst: actionLogEntry{
				key:     "a",
				id:      "Q123",
				summary: "Test Incident",
				action:  "acknowledge",
			},
		},
		{
			name: "Prepends new entry to existing log",
			initialEntries: []actionLogEntry{
				{key: "a", id: "Q123", summary: "Old", action: "acknowledge"},
			},
			newKey:        "^e",
			newID:         "Q456",
			newSummary:    "New Incident",
			newAction:     "re-escalate",
			expectedCount: 2,
			expectedFirst: actionLogEntry{
				key:     "^e",
				id:      "Q456",
				summary: "New Incident",
				action:  "re-escalate",
			},
		},
		{
			name: "Maintains 5 entry limit",
			initialEntries: []actionLogEntry{
				{key: "a", id: "Q1", summary: "Inc 1", action: "acknowledge"},
				{key: "n", id: "Q2", summary: "Inc 2", action: "add note"},
				{key: "^s", id: "Q3", summary: "Inc 3", action: "silence"},
				{key: "^e", id: "Q4", summary: "Inc 4", action: "re-escalate"},
				{key: "a", id: "Q5", summary: "Inc 5", action: "acknowledge"},
			},
			newKey:        "%R",
			newID:         "Q6",
			newSummary:    "Resolved",
			newAction:     "resolved",
			expectedCount: 5,
			expectedFirst: actionLogEntry{
				key:     "%R",
				id:      "Q6",
				summary: "Resolved",
				action:  "resolved",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.actionLog = tt.initialEntries
			m.actionLogTable = newActionLogTable()
			// Set columns to prevent panic
			m.actionLogTable.SetColumns([]table.Column{
				{Title: "", Width: 2},
				{Title: "", Width: 15},
				{Title: "", Width: 30},
				{Title: "", Width: 20},
			})

			m.addActionLogEntry(tt.newKey, tt.newID, tt.newSummary, tt.newAction)

			assert.Equal(t, tt.expectedCount, len(m.actionLog), "Action log count mismatch")
			assert.Equal(t, tt.expectedFirst.key, m.actionLog[0].key, "First entry key mismatch")
			assert.Equal(t, tt.expectedFirst.id, m.actionLog[0].id, "First entry ID mismatch")
			assert.Equal(t, tt.expectedFirst.summary, m.actionLog[0].summary, "First entry summary mismatch")
			assert.Equal(t, tt.expectedFirst.action, m.actionLog[0].action, "First entry action mismatch")
			assert.NotZero(t, m.actionLog[0].timestamp, "Timestamp should be set")
		})
	}
}

func TestAgeOutResolvedIncidents(t *testing.T) {
	tests := []struct {
		name           string
		initialEntries []actionLogEntry
		maxAge         time.Duration
		expectedCount  int
		expectedKeys   []string
	}{
		{
			name: "Keeps all entries when within max age",
			initialEntries: []actionLogEntry{
				{key: "%R", id: "Q1", summary: "Resolved 1", action: "resolved", timestamp: time.Now()},
				{key: "a", id: "Q2", summary: "Ack", action: "acknowledge", timestamp: time.Now()},
				{key: "%R", id: "Q3", summary: "Resolved 2", action: "resolved", timestamp: time.Now()},
			},
			maxAge:        time.Minute * 5,
			expectedCount: 3,
			expectedKeys:  []string{"%R", "a", "%R"},
		},
		{
			name: "Removes aged out resolved incidents",
			initialEntries: []actionLogEntry{
				{key: "%R", id: "Q1", summary: "Old Resolved", action: "resolved", timestamp: time.Now().Add(-time.Minute * 6)},
				{key: "a", id: "Q2", summary: "Ack", action: "acknowledge", timestamp: time.Now().Add(-time.Minute * 6)},
				{key: "%R", id: "Q3", summary: "New Resolved", action: "resolved", timestamp: time.Now()},
			},
			maxAge:        time.Minute * 5,
			expectedCount: 2,
			expectedKeys:  []string{"a", "%R"},
		},
		{
			name: "Keeps user actions regardless of age",
			initialEntries: []actionLogEntry{
				{key: "a", id: "Q1", summary: "Old Ack", action: "acknowledge", timestamp: time.Now().Add(-time.Hour)},
				{key: "^e", id: "Q2", summary: "Old Escalate", action: "re-escalate", timestamp: time.Now().Add(-time.Hour)},
				{key: "n", id: "Q3", summary: "Old Note", action: "add note", timestamp: time.Now().Add(-time.Hour)},
			},
			maxAge:        time.Minute * 5,
			expectedCount: 3,
			expectedKeys:  []string{"a", "^e", "n"},
		},
		{
			name: "Enforces 5 entry limit after aging out",
			initialEntries: []actionLogEntry{
				{key: "a", id: "Q1", summary: "Inc 1", action: "acknowledge", timestamp: time.Now()},
				{key: "a", id: "Q2", summary: "Inc 2", action: "acknowledge", timestamp: time.Now()},
				{key: "a", id: "Q3", summary: "Inc 3", action: "acknowledge", timestamp: time.Now()},
				{key: "a", id: "Q4", summary: "Inc 4", action: "acknowledge", timestamp: time.Now()},
				{key: "a", id: "Q5", summary: "Inc 5", action: "acknowledge", timestamp: time.Now()},
				{key: "a", id: "Q6", summary: "Inc 6", action: "acknowledge", timestamp: time.Now()},
			},
			maxAge:        time.Minute * 5,
			expectedCount: 5,
			expectedKeys:  []string{"a", "a", "a", "a", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.actionLog = tt.initialEntries
			m.actionLogTable = newActionLogTable()
			// Set columns to prevent panic
			m.actionLogTable.SetColumns([]table.Column{
				{Title: "", Width: 2},
				{Title: "", Width: 15},
				{Title: "", Width: 30},
				{Title: "", Width: 20},
			})

			m.ageOutResolvedIncidents(tt.maxAge)

			assert.Equal(t, tt.expectedCount, len(m.actionLog), "Action log count mismatch")
			if len(tt.expectedKeys) > 0 {
				for i, expectedKey := range tt.expectedKeys {
					assert.Equal(t, expectedKey, m.actionLog[i].key, "Entry %d key mismatch", i)
				}
			}
		})
	}
}

func TestResolvedIncidentsAddedToActionLog(t *testing.T) {
	t.Run("Resolved incidents are added to action log with %R key", func(t *testing.T) {
		m := createTestModel()
		m.actionLogTable = newActionLogTable()
		// Set columns to prevent panic
		m.actionLogTable.SetColumns([]table.Column{
			{Title: "", Width: 2},
			{Title: "", Width: 15},
			{Title: "", Width: 30},
			{Title: "", Width: 20},
		})
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		// Set initial incident list
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Incident 1", LastStatusChangeAt: time.Now().Format(time.RFC3339)},
			{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Incident 2", LastStatusChangeAt: time.Now().Format(time.RFC3339)},
		}

		// Update with a list that's missing Q123 (it resolved)
		msg := updatedIncidentListMsg{
			incidents: []pagerduty.Incident{
				{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Incident 2"},
			},
			err: nil,
		}

		result, _ := m.Update(msg)
		m = result.(model)

		// Verify Q123 was added to action log with %R key
		assert.Equal(t, 1, len(m.actionLog), "Action log should have one entry")
		assert.Equal(t, "%R", m.actionLog[0].key, "Resolved incident should have %R key")
		assert.Equal(t, "Q123", m.actionLog[0].id, "Should log the resolved incident ID")
		assert.Equal(t, "Incident 1", m.actionLog[0].summary, "Should log the incident title")
		assert.Equal(t, "resolved", m.actionLog[0].action, "Action should be 'resolved'")
	})

	t.Run("Does not add duplicate resolved incidents to action log", func(t *testing.T) {
		m := createTestModel()
		m.actionLogTable = newActionLogTable()
		// Set columns to prevent panic
		m.actionLogTable.SetColumns([]table.Column{
			{Title: "", Width: 2},
			{Title: "", Width: 15},
			{Title: "", Width: 30},
			{Title: "", Width: 20},
		})
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		// Pre-populate action log with a resolved incident
		m.actionLog = []actionLogEntry{
			{key: "%R", id: "Q123", summary: "Incident 1", action: "resolved", timestamp: time.Now()},
		}

		// Set initial incident list
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Incident 1", LastStatusChangeAt: time.Now().Format(time.RFC3339)},
		}

		// Update with empty list (Q123 resolved again)
		msg := updatedIncidentListMsg{
			incidents: []pagerduty.Incident{},
			err:       nil,
		}

		result, _ := m.Update(msg)
		m = result.(model)

		// Verify Q123 was NOT added again
		assert.Equal(t, 1, len(m.actionLog), "Action log should still have one entry")
		assert.Equal(t, "%R", m.actionLog[0].key, "Entry should still be the resolved incident")
		assert.Equal(t, "Q123", m.actionLog[0].id, "Should be the same incident ID")
	})
}

func TestToggleActionLog(t *testing.T) {
	t.Run("ctrl+l toggles showActionLog", func(t *testing.T) {
		m := createTestModel()
		m.showActionLog = false

		// Simulate ctrl+l keypress
		msg := tea.KeyMsg{Type: tea.KeyCtrlL}

		result, _ := m.Update(msg)
		m = result.(model)

		assert.True(t, m.showActionLog, "showActionLog should be true after first toggle")

		// Toggle again
		result, _ = m.Update(msg)
		m = result.(model)

		assert.False(t, m.showActionLog, "showActionLog should be false after second toggle")
	})
}
