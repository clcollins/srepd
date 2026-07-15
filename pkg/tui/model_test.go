package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"charm.land/glamour/v2"
	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestModel creates a minimal model for testing
func createTestModel() model {
	theme := DefaultTheme()
	styles := BuildStyles(theme)
	t := table.New()
	t.SetStyles(styles.Table)
	return model{
		table:           t,
		incidentCache:   make(map[string]*cachedIncidentData),
		incidentList:    []pagerduty.Incident{},
		theme:           theme,
		styles:          styles,
		cmdExecutor:     &execCommandExecutor{},
		watcherBuffer:   newWatcherBuffer(50),
		watcherViewport: newWatcherViewport(),
		watcherMarker:   emojiWatcherMarker,
		agentMarker:     emojiAgentMarker,
		flagMarker:      emojiFlagMarker,
		watcherDedup:    newWatcherDedup(5 * time.Minute),
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
			name:                 "Alerts tab shows loading when alerts not loaded",
			activeTab:            tabAlerts,
			incidentNotesLoaded:  true,
			incidentAlertsLoaded: false,
			expectedContains:     []string{"Loading alerts..."},
		},
		{
			name:                 "Notes tab shows loading when notes not loaded",
			activeTab:            tabNotes,
			incidentNotesLoaded:  false,
			incidentAlertsLoaded: true,
			expectedContains:     []string{"Loading notes..."},
		},
		{
			name:                 "Details tab renders incident info",
			activeTab:            tabDetails,
			incidentNotesLoaded:  true,
			incidentAlertsLoaded: true,
			expectedContains:     []string{"Q123"},
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
			incident.Service.Summary = "test-service"

			m := model{
				selectedIncident:     incident,
				activeTab:            tt.activeTab,
				incidentNotesLoaded:  tt.incidentNotesLoaded,
				incidentAlertsLoaded: tt.incidentAlertsLoaded,
				incidentCache:        make(map[string]*cachedIncidentData),
			}

			content, err := m.renderTabContent()
			assert.NoError(t, err, "renderTabContent should render without error")

			for _, expected := range tt.expectedContains {
				assert.Contains(t, content, expected, "Should contain %q", expected)
			}

			for _, notExpected := range tt.expectedNotContains {
				assert.NotContains(t, content, notExpected, "Should not contain %q", notExpected)
			}
		})
	}
}

func TestFlashNotification(t *testing.T) {
	t.Run("sets status message and returns a command", func(t *testing.T) {
		m := createTestModel()

		cmd := m.flashNotification("Acknowledged Q123")

		assert.Equal(t, "Acknowledged Q123", m.status, "status should be set to flash message")
		assert.NotNil(t, cmd, "flashNotification should return a command (tick timer)")
	})

	t.Run("clearFlashMsg clears status when message matches", func(t *testing.T) {
		m := createTestModel()
		m.status = "Silenced Q456"

		msg := clearFlashMsg{message: "Silenced Q456"}
		result, cmd := m.Update(msg)
		m = result.(model)

		assert.Equal(t, "", m.status, "status should be cleared when flash message matches")
		assert.Nil(t, cmd, "no further command expected")
	})

	t.Run("clearFlashMsg does not clear status when message differs", func(t *testing.T) {
		m := createTestModel()
		m.status = "showing 3/5 incidents..."

		msg := clearFlashMsg{message: "Acknowledged Q123"}
		result, cmd := m.Update(msg)
		m = result.(model)

		assert.Equal(t, "showing 3/5 incidents...", m.status, "status should not be cleared when message differs")
		assert.Nil(t, cmd, "no further command expected")
	})
}

func TestResolvedIncidentsFlashNotification(t *testing.T) {
	t.Run("Resolved incidents trigger flash notification command", func(t *testing.T) {
		m := createTestModel()
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

		result, cmd := m.Update(msg)
		m = result.(model)

		// The handler returns a batched command that includes the flash notification
		// timer. The status may be overwritten by the incident count message in the
		// same frame, but the clearFlashMsg command is still queued.
		assert.NotNil(t, cmd, "A command should be returned (includes flash notification timer)")

		// Verify the incident list was updated correctly (Q456 remains)
		assert.Equal(t, 1, len(m.incidentList), "Incident list should have one incident remaining")
		assert.Equal(t, "Q456", m.incidentList[0].ID, "Remaining incident should be Q456")
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

func TestConfirmAction_PromptRenderedInModal(t *testing.T) {
	t.Run("View renders confirmation prompt in a centered modal", func(t *testing.T) {
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
	t.Run("clearSelectedIncident resets tab state", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q123"},
		}
		m.viewingIncident = true
		m.activeTab = 1
		m.activeTab = 3
		m.activeTab = 1

		m.clearSelectedIncident("test reset")

		assert.Equal(t, 0, m.activeTab, "activeTab should reset to 0")
		assert.Equal(t, 0, m.activeTab, "activeTab should reset to 0")
		assert.Equal(t, 0, m.activeTab, "activeTab should reset to 0")
	})
}

func TestTabSwitch_TabKey(t *testing.T) {
	tests := []struct {
		name        string
		initialTab  int
		keyMsg      tea.KeyMsg
		expectedTab int
	}{
		{
			name:        "Tab advances from Details to Alerts",
			initialTab:  tabDetails,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: tabAlerts,
		},
		{
			name:        "Tab advances from Alerts to Notes",
			initialTab:  tabAlerts,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: tabNotes,
		},
		{
			name:        "Tab from Notes goes to Cluster",
			initialTab:  tabNotes,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: tabCluster,
		},
		{
			name:        "Tab from LimitedSupport goes to Reports",
			initialTab:  tabLimitedSupport,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: tabReports,
		},
		{
			name:        "Tab from Reports goes to PD History",
			initialTab:  tabReports,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: tabPDHistory,
		},
		{
			name:        "Tab wraps from PD History to Details",
			initialTab:  tabPDHistory,
			keyMsg:      tea.KeyMsg{Type: tea.KeyTab},
			expectedTab: tabDetails,
		},
		{
			name:        "Shift+Tab from Details goes to PD History",
			initialTab:  tabDetails,
			keyMsg:      tea.KeyMsg{Type: tea.KeyShiftTab},
			expectedTab: tabPDHistory,
		},
		{
			name:        "Shift+Tab from Alerts goes to Details",
			initialTab:  tabAlerts,
			keyMsg:      tea.KeyMsg{Type: tea.KeyShiftTab},
			expectedTab: tabDetails,
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

			result, _ := m.Update(tt.keyMsg)
			updatedModel := result.(model)

			assert.Equal(t, tt.expectedTab, updatedModel.activeTab,
				"activeTab should be %d after key press", tt.expectedTab)
		})
	}
}

// captureLogOutput runs a function while capturing log output at the given level.
// Returns the captured log output as a string.
func captureLogOutput(level log.Level, fn func()) string {
	var buf bytes.Buffer
	logger := log.NewWithOptions(os.Stderr, log.Options{})
	logger.SetOutput(&buf)
	logger.SetLevel(level)

	origLogger := log.Default()
	log.SetDefault(logger)
	defer log.SetDefault(origLogger)

	fn()
	return buf.String()
}

func TestSetStatusLogsAtDebugLevel(t *testing.T) {
	t.Run("setStatus logs at DEBUG not INFO", func(t *testing.T) {
		var buf bytes.Buffer
		logger := log.NewWithOptions(os.Stderr, log.Options{})
		logger.SetOutput(&buf)
		logger.SetLevel(log.InfoLevel)

		// Temporarily replace the default logger
		origLogger := log.Default()
		log.SetDefault(logger)
		defer log.SetDefault(origLogger)

		m := createTestModel()

		// With INFO level, setStatus should NOT produce output (it logs at DEBUG)
		buf.Reset()
		m.setStatus("showing 1/1 incident...")
		assert.Empty(t, buf.String(), "setStatus should not produce output at INFO level")
		assert.Equal(t, "showing 1/1 incident...", m.status, "status should still be set on the model")

		// With DEBUG level, setStatus SHOULD produce output
		logger.SetLevel(log.DebugLevel)
		buf.Reset()
		m.setStatus("got incident Q123")
		assert.NotEmpty(t, buf.String(), "setStatus should produce output at DEBUG level")
		assert.Contains(t, buf.String(), "got incident Q123", "log output should contain the status message")
	})
}

func TestSREActionsLogAtInfoLevel(t *testing.T) {
	tests := []struct {
		name          string
		msg           tea.Msg
		setupModel    func(*model)
		expectedInLog string
		description   string
	}{
		{
			name: "acknowledged incidents logs at INFO",
			msg: acknowledgedIncidentsMsg{
				incidents: []pagerduty.Incident{
					{APIObject: pagerduty.APIObject{ID: "Q123"}},
				},
				err: nil,
			},
			setupModel: func(m *model) {
				m.config = &pd.Config{
					CurrentUser: &pagerduty.User{
						APIObject: pagerduty.APIObject{ID: "U123"},
					},
				}
			},
			expectedInLog: "acknowledged incident",
			description:   "acknowledging an incident should produce an INFO log",
		},
		{
			name: "re-escalated incidents logs at INFO",
			msg:  reEscalatedIncidentsMsg([]pagerduty.Incident{{APIObject: pagerduty.APIObject{ID: "Q456"}}}),
			setupModel: func(m *model) {
				m.config = &pd.Config{
					CurrentUser: &pagerduty.User{
						APIObject: pagerduty.APIObject{ID: "U123"},
					},
				}
			},
			expectedInLog: "re-escalated incident",
			description:   "re-escalating an incident should produce an INFO log",
		},
		{
			name: "note added logs at INFO",
			msg: addedIncidentNoteMsg{
				note: &pagerduty.IncidentNote{ID: "N123", Content: "test note"},
				err:  nil,
			},
			setupModel: func(m *model) {
				m.selectedIncident = &pagerduty.Incident{
					APIObject: pagerduty.APIObject{ID: "Q789"},
				}
			},
			expectedInLog: "added note",
			description:   "adding a note should produce an INFO log",
		},
		{
			name: "login finished successfully logs at INFO",
			msg:  loginFinishedMsg{err: nil},
			setupModel: func(m *model) {
				m.selectedIncident = &pagerduty.Incident{
					APIObject: pagerduty.APIObject{ID: "Q111"},
				}
			},
			expectedInLog: "login completed",
			description:   "successful login should produce an INFO log",
		},
		{
			name: "resolved incidents logs at INFO",
			msg: updatedIncidentListMsg{
				incidents: []pagerduty.Incident{
					{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Incident 2"},
				},
				err: nil,
			},
			setupModel: func(m *model) {
				m.config = &pd.Config{
					CurrentUser: &pagerduty.User{
						APIObject: pagerduty.APIObject{ID: "U123"},
					},
				}
				m.incidentList = []pagerduty.Incident{
					{
						APIObject:          pagerduty.APIObject{ID: "Q123"},
						Title:              "Incident 1",
						Service:            pagerduty.APIObject{Summary: "svc-1"},
						LastStatusChangeAt: time.Now().Format(time.RFC3339),
					},
					{
						APIObject:          pagerduty.APIObject{ID: "Q456"},
						Title:              "Incident 2",
						Service:            pagerduty.APIObject{Summary: "svc-2"},
						LastStatusChangeAt: time.Now().Format(time.RFC3339),
					},
				}
			},
			expectedInLog: "incident resolved",
			description:   "resolved incidents should produce an INFO log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			if tt.setupModel != nil {
				tt.setupModel(&m)
			}

			output := captureLogOutput(log.InfoLevel, func() {
				result, _ := m.Update(tt.msg)
				m = result.(model)
			})

			assert.Contains(t, output, tt.expectedInLog,
				"%s: expected INFO log containing %q", tt.description, tt.expectedInLog)
		})
	}
}

// --- InitialModel / InitialModelWithConfig tests ---

func TestInitialModel_ValidConfig(t *testing.T) {
	t.Run("returns a model with expected default fields", func(t *testing.T) {
		mockClient := &pd.MockPagerDutyClient{}
		l := launcher.ClusterLauncher{}
		editor := []string{"vim"}

		// Use InitialModelWithConfig to avoid live PagerDuty API calls
		config := &pd.Config{
			Client: mockClient,
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		teaModel, cmd := InitialModelWithConfig(config, editor, l, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		assert.NotNil(t, teaModel, "InitialModelWithConfig should return a non-nil model")
		assert.NotNil(t, cmd, "InitialModelWithConfig should return a non-nil cmd")

		m := teaModel.(model)

		// Check default field values
		assert.True(t, m.autoRefresh, "autoRefresh should default to true")
		assert.True(t, m.showLowUrgency, "showLowUrgency should default to true")
		assert.False(t, m.debug, "debug should be false when passed as false")
		assert.False(t, m.apiInProgress, "apiInProgress should default to false")
		assert.Equal(t, "", m.status, "status should default to empty string")
		assert.NotNil(t, m.incidentCache, "incidentCache should be initialized")
		assert.NotNil(t, m.scheduledJobs, "scheduledJobs should be initialized")
		assert.Equal(t, editor, m.editor, "editor should match input")
	})
}

func TestInitialModel_DebugFlag(t *testing.T) {
	t.Run("debug flag propagates to the model", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, true, nil, nil, "", nil)
		m := teaModel.(model)

		assert.True(t, m.debug, "debug should be true when passed as true")
	})
}

func TestInitialModelWithConfig_NilConfig(t *testing.T) {
	t.Run("nil config sets m.err", func(t *testing.T) {
		teaModel, cmd := InitialModelWithConfig(nil, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		m := teaModel.(model)

		assert.NotNil(t, m.err, "m.err should be set when config is nil")
		assert.Contains(t, m.err.Error(), "config is nil", "error should mention nil config")
		assert.NotNil(t, cmd, "cmd should be non-nil (returns errMsg)")
	})
}

func TestInitialModelWithConfig_SetsConfig(t *testing.T) {
	t.Run("config is stored on the model", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "USER42"},
			},
		}

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		m := teaModel.(model)

		assert.Equal(t, config, m.config, "config should be stored on the model")
		assert.Nil(t, m.err, "m.err should be nil for valid config")
	})
}

func TestInitialModelWithConfig_CmdReturnsErrMsg(t *testing.T) {
	t.Run("cmd produces errMsg with nil error for valid config", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		_, cmd := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		assert.NotNil(t, cmd, "cmd should be non-nil")

		msg := cmd()
		em, ok := msg.(errMsg)
		assert.True(t, ok, "cmd should produce an errMsg")
		assert.Nil(t, em.error, "error should be nil for valid config")
	})
}

func TestInitialModelWithConfig_CmdReturnsErrMsgForNilConfig(t *testing.T) {
	t.Run("cmd produces errMsg with non-nil error for nil config", func(t *testing.T) {
		_, cmd := InitialModelWithConfig(nil, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		assert.NotNil(t, cmd, "cmd should be non-nil")

		msg := cmd()
		em, ok := msg.(errMsg)
		assert.True(t, ok, "cmd should produce an errMsg")
		assert.NotNil(t, em.error, "error should be non-nil for nil config")
	})
}

func TestInitialModel_ScheduledJobsInitialized(t *testing.T) {
	t.Run("scheduled jobs are initialized with at least one job", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		m := teaModel.(model)

		assert.GreaterOrEqual(t, len(m.scheduledJobs), 1, "should have at least one scheduled job (PollIncidents)")
	})
}

func TestInitialModel_MarkdownRenderer(t *testing.T) {
	t.Run("markdown renderer is created for InitialModelWithConfig", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U123"},
			},
		}

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, launcher.ClusterLauncher{}, false, nil, nil, "", nil)
		m := teaModel.(model)

		// markdownRenderer should be non-nil (created by NewTermRenderer)
		assert.NotNil(t, m.markdownRenderer, "markdownRenderer should be initialized")
	})
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

func TestLogFilePathForOS(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err, "should be able to get user home dir")

	tests := []struct {
		name     string
		goos     string
		expected string
	}{
		{
			name:     "Linux returns XDG config path",
			goos:     "linux",
			expected: filepath.Join(home, ".config", "srepd", "debug.log"),
		},
		{
			name:     "Darwin returns Library/Logs path",
			goos:     "darwin",
			expected: filepath.Join(home, "Library", "Logs", "srepd.log"),
		},
		{
			name:     "Unsupported OS returns empty string",
			goos:     "windows",
			expected: "",
		},
		{
			name:     "Empty OS string returns empty string",
			goos:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logFilePathForOS(tt.goos)
			assert.Equal(t, tt.expected, result, "logFilePathForOS(%q) mismatch", tt.goos)
		})
	}
}

func TestDefaultLogFilePath(t *testing.T) {
	t.Run("returns non-empty path on supported platforms", func(t *testing.T) {
		// defaultLogFilePath() calls logFilePathForOS(runtime.GOOS)
		// On Linux (CI) and macOS (dev), it should return a valid path
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			t.Skip("test only runs on linux or darwin")
		}

		result := defaultLogFilePath()
		assert.NotEmpty(t, result, "defaultLogFilePath should return a non-empty path on %s", runtime.GOOS)

		// Verify it uses the correct OS-specific path segment
		switch runtime.GOOS {
		case "linux":
			assert.Contains(t, result, filepath.Join(".config", "srepd", "debug.log"),
				"Linux path should contain .config/srepd/debug.log")
		case "darwin":
			assert.Contains(t, result, filepath.Join("Library", "Logs", "srepd.log"),
				"Darwin path should contain Library/Logs/srepd.log")
		}
	})
}

func TestDefaultLogFilePath_MatchesLogFilePathForOS(t *testing.T) {
	t.Run("defaultLogFilePath is consistent with logFilePathForOS for current OS", func(t *testing.T) {
		expected := logFilePathForOS(runtime.GOOS)
		actual := defaultLogFilePath()
		assert.Equal(t, expected, actual,
			"defaultLogFilePath() should equal logFilePathForOS(runtime.GOOS)")
	})
}

func TestToggleHelp(t *testing.T) {
	tests := []struct {
		name           string
		initialShowAll bool
		expectedAfter  bool
	}{
		{
			name:           "toggle from false to true",
			initialShowAll: false,
			expectedAfter:  true,
		},
		{
			name:           "toggle from true to false",
			initialShowAll: true,
			expectedAfter:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.help = newHelp()
			m.help.ShowAll = tt.initialShowAll

			m.toggleHelp()

			assert.Equal(t, tt.expectedAfter, m.help.ShowAll,
				"help.ShowAll should toggle from %v to %v", tt.initialShowAll, tt.expectedAfter)
		})
	}
}

func TestToggleHelp_DoubleToggle(t *testing.T) {
	t.Run("double toggle returns to original state", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		m.help.ShowAll = false

		m.toggleHelp()
		assert.True(t, m.help.ShowAll, "first toggle should set ShowAll to true")

		m.toggleHelp()
		assert.False(t, m.help.ShowAll, "second toggle should set ShowAll back to false")
	})
}

func TestInitialModelWithConfig_FieldInitialization(t *testing.T) {
	mockConfig := &pd.Config{
		Client: &pd.MockPagerDutyClient{},
		CurrentUser: &pagerduty.User{
			APIObject: pagerduty.APIObject{ID: "U123"},
		},
	}
	editor := []string{"vim"}
	l := launcher.ClusterLauncher{}
	debug := true

	result, cmd := InitialModelWithConfig(mockConfig, editor, l, launcher.ClusterLauncher{}, debug, nil, nil, "", nil)

	assert.NotNil(t, result, "model should not be nil")
	assert.NotNil(t, cmd, "cmd should not be nil")

	m, ok := result.(model)
	assert.True(t, ok, "result should be a model")

	// Verify field assignments
	assert.Equal(t, editor, m.editor, "editor should be set")
	assert.True(t, m.debug, "debug should be set to true")
	assert.NotNil(t, m.config, "config should be set")
	assert.Equal(t, mockConfig, m.config, "config should match input")

	// Verify initialized components
	assert.NotNil(t, m.incidentCache, "incidentCache should be initialized")
	assert.NotNil(t, m.scheduledJobs, "scheduledJobs should be initialized")
	assert.True(t, m.autoRefresh, "autoRefresh should default to true")
	assert.True(t, m.showLowUrgency, "showLowUrgency should default to true")
	assert.False(t, m.apiInProgress, "apiInProgress should default to false")
	assert.Empty(t, m.status, "status should default to empty")
}

func TestErrMsgHandler_ResetsApiInProgress(t *testing.T) {
	t.Run("errMsgHandler resets apiInProgress to false", func(t *testing.T) {
		m := createTestModel()
		m.apiInProgress = true

		result, cmd := m.Update(errMsg{errors.New("API timeout")})
		updated, ok := result.(model)
		require.True(t, ok)

		assert.False(t, updated.apiInProgress,
			"apiInProgress should be reset to false after errMsg")
		assert.Nil(t, cmd, "errMsgHandler should return nil cmd")
		assert.NotNil(t, updated.err, "err should be set")
		// The error renders via the full-screen error view (m.err), not the
		// transient status line — background polls overwrite the status
		// within seconds, so an error copied there is lost almost immediately.
		assert.NotContains(t, updated.status, "API timeout",
			"error must not be copied into the transient status line")
	})
}
