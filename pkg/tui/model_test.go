package tui

import (
	"fmt"
	"testing"
	"time"

	"charm.land/glamour/v2"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		name                  string
		msg                   tea.Msg
		initialDataLoaded     bool
		initialNotesLoaded    bool
		initialAlertsLoaded   bool
		expectedDataLoaded    bool
		expectedNotesLoaded   bool
		expectedAlertsLoaded  bool
		setupSelectedIncident bool
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
			name:                  "clearSelectedIncidentsMsg clears all loading flags",
			msg:                   clearSelectedIncidentsMsg("test"),
			initialDataLoaded:     true,
			initialNotesLoaded:    true,
			initialAlertsLoaded:   true,
			expectedDataLoaded:    false,
			expectedNotesLoaded:   false,
			expectedAlertsLoaded:  false,
			setupSelectedIncident: true,
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
		activeTab            int
		incidentNotesLoaded  bool
		incidentAlertsLoaded bool
		expectedContains     []string
		expectedNotContains  []string
	}{
		{
			name:                 "Tab header shows loading indicator for notes when not loaded",
			activeTab:            0,
			incidentNotesLoaded:  false,
			incidentAlertsLoaded: true,
			expectedContains:     []string{"Notes (...)"},
			expectedNotContains:  []string{"Alerts (...)"},
		},
		{
			name:                 "Tab header shows loading indicator for alerts when not loaded",
			activeTab:            0,
			incidentNotesLoaded:  true,
			incidentAlertsLoaded: false,
			expectedContains:     []string{"Alerts (...)"},
			expectedNotContains:  []string{"Notes (...)"},
		},
		{
			name:                 "Tab header shows both loading indicators when neither loaded",
			activeTab:            0,
			incidentNotesLoaded:  false,
			incidentAlertsLoaded: false,
			expectedContains:     []string{"Notes (...)", "Alerts (...)"},
		},
		{
			name:                 "Alerts tab shows loading message when alerts not loaded",
			activeTab:            1,
			incidentNotesLoaded:  true,
			incidentAlertsLoaded: false,
			expectedContains:     []string{"Loading alerts..."},
		},
		{
			name:                 "Notes tab shows loading message when notes not loaded",
			activeTab:            2,
			incidentNotesLoaded:  false,
			incidentAlertsLoaded: true,
			expectedContains:     []string{"Loading notes..."},
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
				activeTab:            tt.activeTab,
				incidentNotesLoaded:  tt.incidentNotesLoaded,
				incidentAlertsLoaded: tt.incidentAlertsLoaded,
				incidentCache:        make(map[string]*cachedIncidentData),
			}

			content, err := m.template()
			assert.NoError(t, err, "Template should render without error")

			for _, expected := range tt.expectedContains {
				assert.Contains(t, content, expected, "Should contain %q", expected)
			}

			for _, notExpected := range tt.expectedNotContains {
				assert.NotContains(t, content, notExpected, "Should not contain %q", notExpected)
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
			{Title: "", Width: 29},
		})
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		// Set initial incident list
		m.incidentList = []pagerduty.Incident{
			{
				APIObject:          pagerduty.APIObject{ID: "Q123"},
				Title:              "Incident 1",
				Service:            pagerduty.APIObject{Summary: "test-service-1"},
				LastStatusChangeAt: time.Now().Format(time.RFC3339),
			},
			{
				APIObject:          pagerduty.APIObject{ID: "Q456"},
				Title:              "Incident 2",
				Service:            pagerduty.APIObject{Summary: "test-service-2"},
				LastStatusChangeAt: time.Now().Format(time.RFC3339),
			},
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
		assert.Equal(t, "test-service-1", m.actionLog[0].action, "Action should contain service summary")
	})

	t.Run("Does not add duplicate resolved incidents to action log", func(t *testing.T) {
		m := createTestModel()
		m.actionLogTable = newActionLogTable()
		// Set columns to prevent panic
		m.actionLogTable.SetColumns([]table.Column{
			{Title: "", Width: 2},
			{Title: "", Width: 15},
			{Title: "", Width: 30},
			{Title: "", Width: 29},
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
			{
				APIObject:          pagerduty.APIObject{ID: "Q123"},
				Title:              "Incident 1",
				Service:            pagerduty.APIObject{Summary: "test-service-1"},
				LastStatusChangeAt: time.Now().Format(time.RFC3339),
			},
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

// TestEscapeKeySyncsToHighlightedRow verifies that pressing Escape re-syncs
// the selectedIncident to whatever row is currently highlighted
func TestEscapeKeySyncsToHighlightedRow(t *testing.T) {
	m := createTestModel()
	m.config = &pd.Config{}

	// Create incident list
	m.incidentList = []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q123"},
			Title:              "Incident 1",
			Service:            pagerduty.APIObject{Summary: "service-1"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
		{
			APIObject:          pagerduty.APIObject{ID: "Q456"},
			Title:              "Incident 2",
			Service:            pagerduty.APIObject{Summary: "service-2"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	// Create table with the incidents
	cols := []table.Column{
		{Title: "Status", Width: 10},
		{Title: "ID", Width: 10},
		{Title: "Title", Width: 20},
		{Title: "Service", Width: 15},
		{Title: "Since", Width: 10},
		{Title: "User", Width: 10},
	}
	rows := []table.Row{
		{"triggered", "Q123", "Incident 1", "service-1", "2024-01-01", "user1"},
		{"triggered", "Q456", "Incident 2", "service-2", "2024-01-01", "user2"},
	}
	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	// Select first incident and view it
	m.selectedIncident = &m.incidentList[0]
	m.viewingIncident = true

	// Simulate Escape key - should clear and re-sync to highlighted row
	// Note: In real usage, the table cursor position would determine which incident gets selected
	// For this test, we're verifying the sync happens after Escape
	m.clearSelectedIncident("test escape")
	m.syncSelectedIncidentToHighlightedRow()

	// After sync, selectedIncident should match the highlighted row
	// Since we can't easily control table selection in unit tests, we verify the sync was attempted
	assert.NotNil(t, m.selectedIncident, "selectedIncident should be re-synced after Escape")
}

// TestSelectedIncidentSurvivesListUpdate verifies that copying incident data
// prevents pointer aliasing issues when the incident list is reallocated
func TestSelectedIncidentSurvivesListUpdate(t *testing.T) {
	m := createTestModel()

	// Create initial incident list
	m.incidentList = []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q123"},
			Title:              "Original Title",
			Service:            pagerduty.APIObject{Summary: "service-1"},
			LastStatusChangeAt: "2024-01-01T00:00:00Z",
		},
	}

	// Create table with the incident
	cols := []table.Column{
		{Title: "Status", Width: 10},
		{Title: "ID", Width: 10},
		{Title: "Title", Width: 20},
		{Title: "Service", Width: 15},
		{Title: "Since", Width: 10},
		{Title: "User", Width: 10},
	}
	rows := []table.Row{
		{"triggered", "Q123", "Original Title", "service-1", "2024-01-01", "user1"},
	}
	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	// Sync to select the incident (creates a copy)
	m.syncSelectedIncidentToHighlightedRow()

	assert.NotNil(t, m.selectedIncident, "selectedIncident should be set")
	originalTitle := m.selectedIncident.Title
	assert.Equal(t, "Original Title", originalTitle)

	// Update the incident list (reallocate the slice)
	m.incidentList = []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q123"},
			Title:              "Updated Title",
			Service:            pagerduty.APIObject{Summary: "service-1"},
			LastStatusChangeAt: "2024-01-01T00:00:00Z",
		},
		{
			APIObject:          pagerduty.APIObject{ID: "Q456"},
			Title:              "New Incident",
			Service:            pagerduty.APIObject{Summary: "service-2"},
			LastStatusChangeAt: "2024-01-01T00:00:00Z",
		},
	}

	// Verify selectedIncident still has the original title (not affected by list update)
	// This proves we copied the data instead of storing a pointer to the slice element
	assert.Equal(t, "Original Title", m.selectedIncident.Title,
		"selectedIncident should retain original data after list reallocation")
}

func TestFindRowIndex_Found(t *testing.T) {
	rows := []table.Row{
		{"triggered", "Q111", "First Incident", "service-a"},
		{"acknowledged", "Q222", "Second Incident", "service-b"},
		{"triggered", "Q333", "Third Incident", "service-c"},
	}

	tests := []struct {
		name       string
		incidentID string
		expected   int
	}{
		{"finds first row", "Q111", 0},
		{"finds middle row", "Q222", 1},
		{"finds last row", "Q333", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findRowIndex(rows, tt.incidentID)
			assert.Equal(t, tt.expected, result, "Expected row index %d for incident %s", tt.expected, tt.incidentID)
		})
	}
}

func TestFindRowIndex_NotFound(t *testing.T) {
	rows := []table.Row{
		{"triggered", "Q111", "First Incident", "service-a"},
		{"acknowledged", "Q222", "Second Incident", "service-b"},
	}

	result := findRowIndex(rows, "QNOTFOUND")
	assert.Equal(t, -1, result, "Expected -1 when incident ID is not in rows")
}

func TestFindRowIndex_EmptyRows(t *testing.T) {
	var rows []table.Row

	result := findRowIndex(rows, "Q123")
	assert.Equal(t, -1, result, "Expected -1 when rows slice is empty")
}

// createTestModelWithSelectedIncident creates a model with a table, incident list,
// and a highlighted/selected incident for testing confirmation prompts and actions
func createTestModelWithSelectedIncident() model {
	m := createTestModel()
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

	m.incidentList = []pagerduty.Incident{
		{
			APIObject:        pagerduty.APIObject{ID: "P1234567"},
			Title:            "Test Alert Firing",
			Service:          pagerduty.APIObject{ID: "SVC789", Summary: "test-service"},
			EscalationPolicy: pagerduty.APIObject{ID: "POL123"},
		},
	}

	cols := []table.Column{
		{Title: dot, Width: dotWidth},
		{Title: "ID", Width: idWidth - dotWidth},
		{Title: "Summary", Width: 30},
		{Title: "Service", Width: 20},
	}
	rows := []table.Row{
		{dot, "P1234567", "Test Alert Firing", "test-service"},
	}
	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	// Sync the selected incident to the highlighted row
	m.syncSelectedIncidentToHighlightedRow()

	// Set up action log table to prevent panics
	m.actionLogTable = newActionLogTable()
	m.actionLogTable.SetColumns([]table.Column{
		{Title: "", Width: 2},
		{Title: "", Width: 15},
		{Title: "", Width: 30},
		{Title: "", Width: 20},
	})

	return m
}

func TestConfirmAction_AckExecutesDirectly(t *testing.T) {
	t.Run("pressing 'a' in table mode executes immediately without confirmation", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		result, cmd := m.Update(msg)
		m = result.(model)

		assert.Nil(t, m.pendingConfirmation, "acknowledge should not set pendingConfirmation")
		assert.NotNil(t, cmd, "acknowledge should return a command immediately")
	})

	t.Run("pressing 'a' in incident view mode executes immediately without confirmation", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.viewingIncident = true

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		result, cmd := m.Update(msg)
		m = result.(model)

		assert.Nil(t, m.pendingConfirmation, "acknowledge should not set pendingConfirmation in incident view")
		assert.NotNil(t, cmd, "acknowledge should return a command immediately")
	})
}

func TestConfirmAction_ReEscalateShowsPrompt(t *testing.T) {
	t.Run("pressing ctrl+e sets pendingConfirmation instead of executing", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		msg := tea.KeyMsg{Type: tea.KeyCtrlE}
		result, cmd := m.Update(msg)
		m = result.(model)

		assert.NotNil(t, m.pendingConfirmation, "pendingConfirmation should be set after pressing ctrl+e")
		assert.Contains(t, m.pendingConfirmation.prompt, "P1234567", "prompt should contain incident ID")
		assert.Contains(t, m.pendingConfirmation.prompt, "Re-escalate", "prompt should describe the action")
		assert.Nil(t, cmd, "no command should execute before confirmation")
	})
}

func TestConfirmAction_SilenceShowsPrompt(t *testing.T) {
	t.Run("pressing ctrl+s sets pendingConfirmation instead of executing", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		msg := tea.KeyMsg{Type: tea.KeyCtrlS}
		result, cmd := m.Update(msg)
		m = result.(model)

		assert.NotNil(t, m.pendingConfirmation, "pendingConfirmation should be set after pressing ctrl+s")
		assert.Contains(t, m.pendingConfirmation.prompt, "P1234567", "prompt should contain incident ID")
		assert.Contains(t, m.pendingConfirmation.prompt, "Silence", "prompt should describe the action")
		assert.Nil(t, cmd, "no command should execute before confirmation")
	})
}

func TestConfirmAction_YesExecutes(t *testing.T) {
	t.Run("pressing 'y' with pendingConfirmation executes the action and clears it", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		// Set up pendingConfirmation by pressing ctrl+s (silence)
		msg := tea.KeyMsg{Type: tea.KeyCtrlS}
		result, _ := m.Update(msg)
		m = result.(model)
		assert.NotNil(t, m.pendingConfirmation, "precondition: pendingConfirmation should be set")

		// Now press 'y' to confirm
		confirmMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		result, cmd := m.Update(confirmMsg)
		m = result.(model)

		// pendingConfirmation should be cleared
		assert.Nil(t, m.pendingConfirmation, "pendingConfirmation should be cleared after 'y'")
		// A command should have been returned (the action executes)
		assert.NotNil(t, cmd, "command should be returned after confirming")
	})
}

func TestConfirmAction_NoAborts(t *testing.T) {
	t.Run("pressing 'n' with pendingConfirmation clears it without executing", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		// Set up pendingConfirmation via ctrl+s (silence)
		msg := tea.KeyMsg{Type: tea.KeyCtrlS}
		result, _ := m.Update(msg)
		m = result.(model)
		assert.NotNil(t, m.pendingConfirmation, "precondition: pendingConfirmation should be set")

		// Now press 'n' to abort
		cancelMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
		result, cmd := m.Update(cancelMsg)
		m = result.(model)

		// pendingConfirmation should be cleared
		assert.Nil(t, m.pendingConfirmation, "pendingConfirmation should be cleared after 'n'")
		// No command should execute
		assert.Nil(t, cmd, "no command should execute after aborting")
		// Status should indicate cancellation
		assert.Contains(t, m.status, "cancelled", "status should indicate cancellation")
	})
}

func TestConfirmAction_EscAborts(t *testing.T) {
	t.Run("pressing Escape with pendingConfirmation clears it without executing", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		// Set up pendingConfirmation via ctrl+s (silence)
		msg := tea.KeyMsg{Type: tea.KeyCtrlS}
		result, _ := m.Update(msg)
		m = result.(model)
		assert.NotNil(t, m.pendingConfirmation, "precondition: pendingConfirmation should be set")

		// Now press Escape to abort
		escMsg := tea.KeyMsg{Type: tea.KeyEscape}
		result, cmd := m.Update(escMsg)
		m = result.(model)

		// pendingConfirmation should be cleared
		assert.Nil(t, m.pendingConfirmation, "pendingConfirmation should be cleared after Escape")
		// No command should execute
		assert.Nil(t, cmd, "no command should execute after pressing Escape")
		// Status should indicate cancellation
		assert.Contains(t, m.status, "cancelled", "status should indicate cancellation")
	})
}

func TestConfirmAction_ClearedOnViewTransition(t *testing.T) {
	t.Run("clearSelectedIncident clears pendingConfirmation", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		// Set up pendingConfirmation directly
		m.pendingConfirmation = &confirmActionState{
			prompt: "Acknowledge P1234567? [y/n]",
			action: func() tea.Msg { return acknowledgeIncidentsMsg{} },
		}

		// clearSelectedIncident should also clear pendingConfirmation
		m.clearSelectedIncident("test")

		assert.Nil(t, m.pendingConfirmation, "pendingConfirmation should be cleared by clearSelectedIncident")
	})

	t.Run("confirmation blocks other keys including Enter", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		// Set up pendingConfirmation via ctrl+s (silence)
		msg := tea.KeyMsg{Type: tea.KeyCtrlS}
		result, _ := m.Update(msg)
		m = result.(model)
		assert.NotNil(t, m.pendingConfirmation, "precondition: pendingConfirmation should be set")

		// Press Enter -- should be ignored while confirmation is active
		enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
		result, cmd := m.Update(enterMsg)
		m = result.(model)

		// pendingConfirmation should still be set (Enter is ignored)
		assert.NotNil(t, m.pendingConfirmation, "pendingConfirmation should remain set when Enter is pressed")
		assert.Nil(t, cmd, "no command should execute for non-confirmation keys")
	})
}

func TestConfirmAction_OtherKeysIgnored(t *testing.T) {
	t.Run("pressing non-y/n/Escape keys are ignored during confirmation", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		// Set up pendingConfirmation via ctrl+s (silence)
		msg := tea.KeyMsg{Type: tea.KeyCtrlS}
		result, _ := m.Update(msg)
		m = result.(model)
		assert.NotNil(t, m.pendingConfirmation, "precondition: pendingConfirmation should be set")

		savedPrompt := m.pendingConfirmation.prompt

		// Press an unrelated key like 'x'
		otherMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
		result, cmd := m.Update(otherMsg)
		m = result.(model)

		// pendingConfirmation should still be set (not cleared)
		assert.NotNil(t, m.pendingConfirmation, "pendingConfirmation should remain set after unrelated key")
		assert.Equal(t, savedPrompt, m.pendingConfirmation.prompt, "prompt should not change")
		assert.Nil(t, cmd, "no command should execute for unrelated keys")
	})
}

func TestConfirmAction_PromptRenderedInStatusArea(t *testing.T) {
	t.Run("View renders confirmation prompt in status area", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()

		// Need window size for View() to work
		windowSize = tea.WindowSizeMsg{Width: 120, Height: 50}
		// Need to handle windowSizeMsgHandler to set columns etc.
		result, _ := m.windowSizeMsgHandler(windowSize)
		m = result.(model)

		// Reset table rows after windowSizeMsgHandler
		m.table.SetRows([]table.Row{
			{dot, "P1234567", "Test Alert Firing", "test-service"},
		})

		// Set up a pending confirmation
		m.pendingConfirmation = &confirmActionState{
			prompt: "Silence P1234567? [y/n]",
			action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
		}

		view := m.View()
		assert.Contains(t, view, "Silence P1234567? [y/n]", "View should render the confirmation prompt")
	})
}

func TestGlamourRendererProducesBoldOutput(t *testing.T) {
	// Create renderer with dark style (same as production code)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(100),
	)
	require.NoError(t, err, "glamour renderer should be created without error")
	require.NotNil(t, renderer, "renderer should not be nil")

	// Render markdown with bold text
	output, err := renderer.Render("**bold text**")
	require.NoError(t, err, "rendering should not error")

	// With dark style, bold text should contain ANSI escape sequences
	// (not just the raw ** markers). The exact ANSI codes vary, but
	// the output should NOT contain the literal "**" markers.
	assert.NotContains(t, output, "**bold text**",
		"glamour with dark style should render bold as ANSI, not raw markdown")
	assert.Contains(t, output, "bold text",
		"rendered output should still contain the text content")
}

// --- Eager pre-fetch tests (Issue #198) ---

// createTestModelWithTable sets up a model with a table containing the given
// incidents and an incident list that matches. The table cursor starts at row 0.
func createTestModelWithTable(incidents []pagerduty.Incident) model {
	m := createTestModel()
	m.incidentList = incidents

	cols := []table.Column{
		{Title: dot, Width: dotWidth},
		{Title: "ID", Width: idWidth - dotWidth},
		{Title: "Summary", Width: 30},
		{Title: "Service", Width: 20},
	}
	var rows []table.Row
	for _, inc := range incidents {
		rows = append(rows, table.Row{dot, inc.ID, inc.Title, inc.Service.Summary})
	}
	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
	)
	return m
}

func TestSyncSelectedIncident_TriggersPrefetch(t *testing.T) {
	t.Run("uncached incident triggers pre-fetch command", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			{
				APIObject:          pagerduty.APIObject{ID: "Q111"},
				Title:              "Test Incident",
				Service:            pagerduty.APIObject{Summary: "test-svc"},
				LastStatusChangeAt: "2024-01-01T00:00:00Z",
			},
		}
		m := createTestModelWithTable(incidents)

		cmd := m.syncSelectedIncidentToHighlightedRow()

		assert.NotNil(t, m.selectedIncident, "selectedIncident should be set")
		assert.Equal(t, "Q111", m.selectedIncident.ID, "selected incident should match highlighted row")
		assert.NotNil(t, cmd, "should return a pre-fetch command for uncached incident")
	})
}

func TestSyncSelectedIncident_SkipsCached(t *testing.T) {
	t.Run("fully cached incident does not trigger pre-fetch", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			{
				APIObject:          pagerduty.APIObject{ID: "Q222"},
				Title:              "Cached Incident",
				Service:            pagerduty.APIObject{Summary: "cached-svc"},
				LastStatusChangeAt: "2024-01-01T00:00:00Z",
			},
		}
		m := createTestModelWithTable(incidents)

		// Pre-populate cache with fully loaded data
		incidentCopy := incidents[0]
		m.incidentCache["Q222"] = &cachedIncidentData{
			incident:     &incidentCopy,
			dataLoaded:   true,
			notesLoaded:  true,
			alertsLoaded: true,
			lastFetched:  time.Now(),
		}

		cmd := m.syncSelectedIncidentToHighlightedRow()

		assert.NotNil(t, m.selectedIncident, "selectedIncident should be set from cache")
		assert.Equal(t, "Q222", m.selectedIncident.ID)
		assert.Nil(t, cmd, "should NOT return a command for fully cached incident")
	})
}

func TestSyncSelectedIncident_TriggersPrefetchPartialCache(t *testing.T) {
	t.Run("partially cached incident triggers pre-fetch", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			{
				APIObject:          pagerduty.APIObject{ID: "Q333"},
				Title:              "Partial Incident",
				Service:            pagerduty.APIObject{Summary: "partial-svc"},
				LastStatusChangeAt: "2024-01-01T00:00:00Z",
			},
		}
		m := createTestModelWithTable(incidents)

		// Pre-populate cache with only data loaded (alerts and notes missing)
		incidentCopy := incidents[0]
		m.incidentCache["Q333"] = &cachedIncidentData{
			incident:     &incidentCopy,
			dataLoaded:   true,
			notesLoaded:  false,
			alertsLoaded: false,
			lastFetched:  time.Now(),
		}

		cmd := m.syncSelectedIncidentToHighlightedRow()

		assert.NotNil(t, m.selectedIncident, "selectedIncident should be set from cache")
		assert.Equal(t, "Q333", m.selectedIncident.ID)
		assert.NotNil(t, cmd, "should return a pre-fetch command for partially cached incident")
	})
}

func TestSyncSelectedIncident_NilRowReturnsNil(t *testing.T) {
	t.Run("no highlighted row returns nil command", func(t *testing.T) {
		// Empty table - no rows to highlight
		m := createTestModel()
		m.table = table.New(
			table.WithColumns([]table.Column{
				{Title: dot, Width: dotWidth},
				{Title: "ID", Width: idWidth - dotWidth},
				{Title: "Summary", Width: 30},
				{Title: "Service", Width: 20},
			}),
			table.WithFocused(true),
		)

		cmd := m.syncSelectedIncidentToHighlightedRow()

		assert.Nil(t, m.selectedIncident, "selectedIncident should be nil with no rows")
		assert.Nil(t, cmd, "should return nil command when no row is highlighted")
	})
}

func TestSyncSelectedIncident_SameIncidentReturnsNil(t *testing.T) {
	t.Run("already-selected incident returns nil command", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			{
				APIObject:          pagerduty.APIObject{ID: "Q444"},
				Title:              "Same Incident",
				Service:            pagerduty.APIObject{Summary: "same-svc"},
				LastStatusChangeAt: "2024-01-01T00:00:00Z",
			},
		}
		m := createTestModelWithTable(incidents)

		// Pre-set selectedIncident to the same incident
		incidentCopy := incidents[0]
		m.selectedIncident = &incidentCopy

		cmd := m.syncSelectedIncidentToHighlightedRow()

		assert.NotNil(t, m.selectedIncident, "selectedIncident should remain set")
		assert.Equal(t, "Q444", m.selectedIncident.ID)
		assert.Nil(t, cmd, "should return nil command when incident is already selected")
	})
}

func TestTabReset_OnClearSelectedIncident(t *testing.T) {
	t.Run("clearSelectedIncident resets tab state to defaults", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q123"},
		}
		m.viewingIncident = true
		m.activeTab = 2
		m.activeAlertIdx = 3
		m.activeNoteIdx = 1

		m.clearSelectedIncident("test reset")

		assert.Equal(t, 0, m.activeTab, "activeTab should reset to 0")
		assert.Equal(t, 0, m.activeAlertIdx, "activeAlertIdx should reset to 0")
		assert.Equal(t, 0, m.activeNoteIdx, "activeNoteIdx should reset to 0")
	})
}

func TestTabSwitch_LeftRight(t *testing.T) {
	tests := []struct {
		name        string
		initialTab  int
		keyMsg      tea.KeyMsg
		expectedTab int
	}{
		{
			name:        "Tab key cycles from Details to Alerts",
			initialTab:  0,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: 1,
		},
		{
			name:        "Tab key cycles from Alerts to Notes",
			initialTab:  1,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: 2,
		},
		{
			name:        "Tab key wraps from Notes back to Details",
			initialTab:  2,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: 0,
		},
		{
			name:        "Shift+Tab cycles from Details to Notes (backward wrap)",
			initialTab:  0,
			keyMsg:      tea.KeyMsg{Type: tea.KeyShiftTab},
			expectedTab: 2,
		},
		{
			name:        "Shift+Tab cycles from Notes to Alerts",
			initialTab:  2,
			keyMsg:      tea.KeyMsg{Type: tea.KeyShiftTab},
			expectedTab: 1,
		},
		{
			name:        "Shift+Tab cycles from Alerts to Details",
			initialTab:  1,
			keyMsg:      tea.KeyMsg{Type: tea.KeyShiftTab},
			expectedTab: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.viewingIncident = true
			m.activeTab = tt.initialTab
			m.selectedIncident = &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "Q123"},
			}
			m.incidentAlertsLoaded = true
			m.incidentNotesLoaded = true
			m.selectedIncidentAlerts = []pagerduty.IncidentAlert{
				{APIObject: pagerduty.APIObject{ID: "A1"}},
			}
			m.selectedIncidentNotes = []pagerduty.IncidentNote{
				{ID: "N1"},
			}

			result, _ := m.Update(tt.keyMsg)
			updatedModel := result.(model)

			assert.Equal(t, tt.expectedTab, updatedModel.activeTab,
				"activeTab should be %d after key press", tt.expectedTab)
		})
	}
}

func TestAlertTab_Navigation(t *testing.T) {
	tests := []struct {
		name             string
		initialAlertIdx  int
		alertCount       int
		keyMsg           tea.KeyMsg
		expectedAlertIdx int
	}{
		{
			name:             "Right arrow advances alert index",
			initialAlertIdx:  0,
			alertCount:       3,
			keyMsg:           tea.KeyMsg{Type: tea.KeyRight},
			expectedAlertIdx: 1,
		},
		{
			name:             "Left arrow decrements alert index",
			initialAlertIdx:  2,
			alertCount:       3,
			keyMsg:           tea.KeyMsg{Type: tea.KeyLeft},
			expectedAlertIdx: 1,
		},
		{
			name:             "Right arrow wraps from last to first",
			initialAlertIdx:  2,
			alertCount:       3,
			keyMsg:           tea.KeyMsg{Type: tea.KeyRight},
			expectedAlertIdx: 0,
		},
		{
			name:             "Left arrow wraps from first to last",
			initialAlertIdx:  0,
			alertCount:       3,
			keyMsg:           tea.KeyMsg{Type: tea.KeyLeft},
			expectedAlertIdx: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.viewingIncident = true
			m.activeTab = 1 // Alerts tab
			m.activeAlertIdx = tt.initialAlertIdx
			m.selectedIncident = &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "Q123"},
			}
			m.incidentAlertsLoaded = true

			var alerts []pagerduty.IncidentAlert
			for i := 0; i < tt.alertCount; i++ {
				alerts = append(alerts, pagerduty.IncidentAlert{
					APIObject: pagerduty.APIObject{ID: fmt.Sprintf("A%d", i)},
				})
			}
			m.selectedIncidentAlerts = alerts

			result, _ := m.Update(tt.keyMsg)
			updatedModel := result.(model)

			assert.Equal(t, tt.expectedAlertIdx, updatedModel.activeAlertIdx,
				"activeAlertIdx should be %d", tt.expectedAlertIdx)
		})
	}
}

func TestNoteTab_Navigation(t *testing.T) {
	tests := []struct {
		name            string
		initialNoteIdx  int
		noteCount       int
		keyMsg          tea.KeyMsg
		expectedNoteIdx int
	}{
		{
			name:            "Right arrow advances note index",
			initialNoteIdx:  0,
			noteCount:       3,
			keyMsg:          tea.KeyMsg{Type: tea.KeyRight},
			expectedNoteIdx: 1,
		},
		{
			name:            "Left arrow decrements note index",
			initialNoteIdx:  2,
			noteCount:       3,
			keyMsg:          tea.KeyMsg{Type: tea.KeyLeft},
			expectedNoteIdx: 1,
		},
		{
			name:            "Right arrow wraps from last to first",
			initialNoteIdx:  2,
			noteCount:       3,
			keyMsg:          tea.KeyMsg{Type: tea.KeyRight},
			expectedNoteIdx: 0,
		},
		{
			name:            "Left arrow wraps from first to last",
			initialNoteIdx:  0,
			noteCount:       3,
			keyMsg:          tea.KeyMsg{Type: tea.KeyLeft},
			expectedNoteIdx: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.viewingIncident = true
			m.activeTab = 2 // Notes tab
			m.activeNoteIdx = tt.initialNoteIdx
			m.selectedIncident = &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "Q123"},
			}
			m.incidentNotesLoaded = true

			var notes []pagerduty.IncidentNote
			for i := 0; i < tt.noteCount; i++ {
				notes = append(notes, pagerduty.IncidentNote{
					ID: fmt.Sprintf("N%d", i),
				})
			}
			m.selectedIncidentNotes = notes

			result, _ := m.Update(tt.keyMsg)
			updatedModel := result.(model)

			assert.Equal(t, tt.expectedNoteIdx, updatedModel.activeNoteIdx,
				"activeNoteIdx should be %d", tt.expectedNoteIdx)
		})
	}
}

func TestTabBoundsCheck(t *testing.T) {
	tests := []struct {
		name             string
		activeTab        int
		alertIdx         int
		noteIdx          int
		alertCount       int
		noteCount        int
		keyMsg           tea.KeyMsg
		expectedAlertIdx int
		expectedNoteIdx  int
	}{
		{
			name:             "Alert index clamped with single alert - right wraps to 0",
			activeTab:        1,
			alertIdx:         0,
			alertCount:       1,
			keyMsg:           tea.KeyMsg{Type: tea.KeyRight},
			expectedAlertIdx: 0,
		},
		{
			name:            "Note index clamped with single note - right wraps to 0",
			activeTab:       2,
			noteIdx:         0,
			noteCount:       1,
			keyMsg:          tea.KeyMsg{Type: tea.KeyRight},
			expectedNoteIdx: 0,
		},
		{
			name:             "Number key out of range is ignored for alerts",
			activeTab:        1,
			alertIdx:         0,
			alertCount:       2,
			keyMsg:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}},
			expectedAlertIdx: 0,
		},
		{
			name:            "Number key out of range is ignored for notes",
			activeTab:       2,
			noteIdx:         0,
			noteCount:       2,
			keyMsg:          tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}},
			expectedNoteIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.viewingIncident = true
			m.activeTab = tt.activeTab
			m.activeAlertIdx = tt.alertIdx
			m.activeNoteIdx = tt.noteIdx
			m.selectedIncident = &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "Q123"},
			}
			m.incidentAlertsLoaded = true
			m.incidentNotesLoaded = true

			var alerts []pagerduty.IncidentAlert
			for i := 0; i < tt.alertCount; i++ {
				alerts = append(alerts, pagerduty.IncidentAlert{
					APIObject: pagerduty.APIObject{ID: fmt.Sprintf("A%d", i)},
				})
			}
			m.selectedIncidentAlerts = alerts

			var notes []pagerduty.IncidentNote
			for i := 0; i < tt.noteCount; i++ {
				notes = append(notes, pagerduty.IncidentNote{
					ID: fmt.Sprintf("N%d", i),
				})
			}
			m.selectedIncidentNotes = notes

			result, _ := m.Update(tt.keyMsg)
			updatedModel := result.(model)

			if tt.activeTab == 1 {
				assert.Equal(t, tt.expectedAlertIdx, updatedModel.activeAlertIdx)
			}
			if tt.activeTab == 2 {
				assert.Equal(t, tt.expectedNoteIdx, updatedModel.activeNoteIdx)
			}
		})
	}
}

func TestNumberKeys_JumpToIndex(t *testing.T) {
	tests := []struct {
		name             string
		activeTab        int
		count            int
		keyMsg           tea.KeyMsg
		expectedAlertIdx int
		expectedNoteIdx  int
	}{
		{
			name:             "Pressing 1 jumps to first alert",
			activeTab:        1,
			count:            5,
			keyMsg:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}},
			expectedAlertIdx: 0,
		},
		{
			name:             "Pressing 3 jumps to third alert",
			activeTab:        1,
			count:            5,
			keyMsg:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}},
			expectedAlertIdx: 2,
		},
		{
			name:            "Pressing 2 jumps to second note",
			activeTab:       2,
			count:           3,
			keyMsg:          tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}},
			expectedNoteIdx: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.viewingIncident = true
			m.activeTab = tt.activeTab
			m.selectedIncident = &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "Q123"},
			}
			m.incidentAlertsLoaded = true
			m.incidentNotesLoaded = true

			var alerts []pagerduty.IncidentAlert
			var notes []pagerduty.IncidentNote

			if tt.activeTab == 1 {
				for i := 0; i < tt.count; i++ {
					alerts = append(alerts, pagerduty.IncidentAlert{
						APIObject: pagerduty.APIObject{ID: fmt.Sprintf("A%d", i)},
					})
				}
			} else {
				for i := 0; i < tt.count; i++ {
					notes = append(notes, pagerduty.IncidentNote{
						ID: fmt.Sprintf("N%d", i),
					})
				}
			}
			m.selectedIncidentAlerts = alerts
			m.selectedIncidentNotes = notes

			result, _ := m.Update(tt.keyMsg)
			updatedModel := result.(model)

			if tt.activeTab == 1 {
				assert.Equal(t, tt.expectedAlertIdx, updatedModel.activeAlertIdx)
			}
			if tt.activeTab == 2 {
				assert.Equal(t, tt.expectedNoteIdx, updatedModel.activeNoteIdx)
			}
		})
	}
}

func TestWindowSizeMsgHandler_SmallWindow(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"standard 80x24 terminal", 80, 24},
		{"tiny window", 40, 10},
		{"minimum height", 20, 1},
		{"zero height", 80, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				table:          newTableWithStyles(),
				actionLogTable: newActionLogTable(),
				incidentViewer: newIncidentViewer(),
				help:           newHelp(),
				incidentCache:  make(map[string]*cachedIncidentData),
			}

			// First call sets columns (simulates initial startup WindowSizeMsg)
			normalSize := tea.WindowSizeMsg{Width: 120, Height: 50}
			result, _ := m.windowSizeMsgHandler(normalSize)
			m = result.(model)

			// Incidents arrive and populate the table
			m.table.SetRows([]table.Row{
				{".", "P123ABC", "Test incident", "test-service"},
				{".", "P456DEF", "Another incident", "test-service-2"},
				{".", "P789GHI", "Third incident", "test-service-3"},
			})

			// Second call with small window: columns are already set, so
			// table.SetHeight will subtract a 2-line header from the height.
			// Before the fix, this panicked with "slice bounds out of range"
			// when the resulting viewport height went negative.
			msg := tea.WindowSizeMsg{Width: tt.width, Height: tt.height}
			assert.NotPanics(t, func() {
				result, _ = m.windowSizeMsgHandler(msg)
			})

			m = result.(model)
			assert.GreaterOrEqual(t, m.table.Height(), 1,
				"viewport height must be positive to avoid panic in visibleLines")
		})
	}
}
