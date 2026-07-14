package tui

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// openBrowserMsg
// ---------------------------------------------------------------------------

func TestOpenBrowserMsg_NilSelectedIncident(t *testing.T) {
	t.Run("guard: returns status when selectedIncident is nil", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil

		result, cmd := m.Update(openBrowserMsg("open"))
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned when selectedIncident is nil")
		assert.Equal(t, "no incident selected", m.status)
	})
}

func TestOpenBrowserMsg_HappyPath(t *testing.T) {
	t.Run("returns batch command with flash and browser open", func(t *testing.T) {
		// On Linux/macOS, defaultBrowserOpenCommand is set to a non-empty value,
		// so the happy path should produce a batch command.
		if defaultBrowserOpenCommand == "" {
			t.Skip("defaultBrowserOpenCommand is empty on this OS")
		}

		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}
		m.selectedIncident.HTMLURL = "https://pagerduty.example.com/incidents/P1234567"

		result, cmd := m.Update(openBrowserMsg("open"))
		m = result.(model)

		assert.NotNil(t, cmd, "should return a batch command")
	})
}

// ---------------------------------------------------------------------------
// openSOPMsg
// ---------------------------------------------------------------------------

func TestOpenSOPMsg_NilSelectedIncident(t *testing.T) {
	t.Run("guard: returns status when selectedIncident is nil", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil

		result, cmd := m.Update(openSOPMsg("sop"))
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned when selectedIncident is nil")
		assert.Equal(t, "no incident selected", m.status)
	})
}

func TestOpenSOPMsg_NoSOPLinkFound(t *testing.T) {
	t.Run("no SOP link in alerts sets status", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}
		// No alerts set, so no SOP link will be found
		m.selectedIncidentAlerts = nil

		result, cmd := m.Update(openSOPMsg("sop"))
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned when no SOP link exists")
		assert.Equal(t, "no SOP link found", m.status)
	})
}

func TestOpenSOPMsg_HappyPath(t *testing.T) {
	t.Run("returns batch command when SOP link is found in alerts", func(t *testing.T) {
		if defaultBrowserOpenCommand == "" {
			t.Skip("defaultBrowserOpenCommand is empty on this OS")
		}

		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		// Set up alerts with a SOP link in the detail fields
		m.selectedIncidentAlerts = []pagerduty.IncidentAlert{
			{
				APIObject: pagerduty.APIObject{ID: "A1"},
				Body: map[string]interface{}{
					"details": map[string]interface{}{
						"link": "https://sop.example.com/runbook",
					},
				},
			},
		}

		result, cmd := m.Update(openSOPMsg("sop"))
		m = result.(model)

		assert.NotNil(t, cmd, "should return a batch command when SOP link exists")
	})
}

// ---------------------------------------------------------------------------
// browserFinishedMsg
// ---------------------------------------------------------------------------

func TestBrowserFinishedMsg_Error(t *testing.T) {
	t.Run("error: sets status and returns errMsg", func(t *testing.T) {
		m := createTestModel()

		browserErr := errors.New("browser not found")
		result, cmd := m.Update(browserFinishedMsg{err: browserErr})
		m = result.(model)

		assert.Contains(t, m.status, "failed to open browser")
		assert.Contains(t, m.status, "browser not found")
		assert.NotNil(t, cmd, "should return a command wrapping errMsg")

		msg := cmd()
		em, ok := msg.(errMsg)
		assert.True(t, ok, "returned command should produce errMsg")
		assert.Equal(t, browserErr, em.error)
	})
}

func TestBrowserFinishedMsg_HappyWithSelectedIncident(t *testing.T) {
	t.Run("success with selectedIncident: sets status with incident ID", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "PBROWSER"},
		}

		result, cmd := m.Update(browserFinishedMsg{err: nil})
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned on success")
		assert.Contains(t, m.status, "PBROWSER")
		assert.Contains(t, m.status, "opened incident")
		assert.Contains(t, m.status, "check browser window")
	})
}

func TestBrowserFinishedMsg_HappyWithoutSelectedIncident(t *testing.T) {
	t.Run("success without selectedIncident: sets generic status", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil

		result, cmd := m.Update(browserFinishedMsg{err: nil})
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned on success")
		assert.Contains(t, m.status, "opened incident in browser")
		assert.Contains(t, m.status, "check browser window")
	})
}

func TestBrowserFinishedMsg_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		selectedIncident *pagerduty.Incident
		expectCmd        bool
		expectStatusSub  string
	}{
		{
			name:             "error sets failure status",
			err:              errors.New("exec failed"),
			selectedIncident: nil,
			expectCmd:        true,
			expectStatusSub:  "failed to open browser",
		},
		{
			name: "success with incident includes ID",
			err:  nil,
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "P555"},
			},
			expectCmd:       false,
			expectStatusSub: "P555",
		},
		{
			name:             "success without incident shows generic message",
			err:              nil,
			selectedIncident: nil,
			expectCmd:        false,
			expectStatusSub:  "opened incident in browser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.selectedIncident = tt.selectedIncident

			result, cmd := m.Update(browserFinishedMsg{err: tt.err})
			m = result.(model)

			if tt.expectCmd {
				assert.NotNil(t, cmd, "expected a command")
			} else {
				assert.Nil(t, cmd, "expected no command")
			}
			assert.Contains(t, m.status, tt.expectStatusSub)
		})
	}
}

// ---------------------------------------------------------------------------
// waitForSelectedIncidentThenDoMsg
// ---------------------------------------------------------------------------

func TestWaitForSelectedIncidentThenDoMsg_NilAction(t *testing.T) {
	t.Run("guard: nil action sets status", func(t *testing.T) {
		m := createTestModel()

		msg := waitForSelectedIncidentThenDoMsg{
			action: nil,
			msg:    openBrowserMsg("test"),
		}

		result, cmd := m.Update(msg)
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned for nil action")
		assert.Contains(t, m.status, "no action included")
	})
}

func TestWaitForSelectedIncidentThenDoMsg_NilMsg(t *testing.T) {
	t.Run("guard: nil msg sets status", func(t *testing.T) {
		m := createTestModel()

		msg := waitForSelectedIncidentThenDoMsg{
			action: func() tea.Msg { return nil },
			msg:    nil,
		}

		result, cmd := m.Update(msg)
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned for nil msg")
		assert.Contains(t, m.status, "no data included")
	})
}

func TestWaitForSelectedIncidentThenDoMsg_Cancelled(t *testing.T) {
	t.Run("guard: cancelled when no incident selected and not viewing and no table row", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil
		m.viewingIncident = false
		// Table with no rows means SelectedRow() returns nil

		msg := waitForSelectedIncidentThenDoMsg{
			action: func() tea.Msg { return openBrowserMsg("test") },
			msg:    openBrowserMsg("test"),
		}

		result, cmd := m.Update(msg)
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned when action is cancelled")
		assert.Contains(t, m.status, "action cancelled")
	})
}

func TestWaitForSelectedIncidentThenDoMsg_Requeues(t *testing.T) {
	t.Run("requeues when selectedIncident nil but still viewing", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil
		m.viewingIncident = true // still viewing, so don't cancel

		dummyAction := func() tea.Msg { return openBrowserMsg("deferred") }
		msg := waitForSelectedIncidentThenDoMsg{
			action: dummyAction,
			msg:    openBrowserMsg("deferred"),
		}

		result, cmd := m.Update(msg)
		m = result.(model)

		assert.NotNil(t, cmd, "should return a requeue command")
		assert.Contains(t, m.status, "waiting for incident info")
	})
}

func TestWaitForSelectedIncidentThenDoMsg_RequeuedWithTableRow(t *testing.T) {
	t.Run("requeues when selectedIncident nil but table row is highlighted", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		// Force selectedIncident to nil while keeping the table row
		m.selectedIncident = nil
		m.viewingIncident = false

		dummyAction := func() tea.Msg { return openBrowserMsg("deferred") }
		msg := waitForSelectedIncidentThenDoMsg{
			action: dummyAction,
			msg:    openBrowserMsg("deferred"),
		}

		result, cmd := m.Update(msg)
		m = result.(model)

		assert.NotNil(t, cmd, "should return a requeue command when table row is highlighted")
		assert.Contains(t, m.status, "waiting for incident info")
	})
}

func TestWaitForSelectedIncidentThenDoMsg_HappyPath(t *testing.T) {
	t.Run("executes action when selectedIncident is available", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P777"},
		}

		executed := false
		dummyAction := func() tea.Msg {
			executed = true
			return openBrowserMsg("done")
		}

		msg := waitForSelectedIncidentThenDoMsg{
			action: dummyAction,
			msg:    openBrowserMsg("test"),
		}

		result, cmd := m.Update(msg)
		_ = result.(model)

		assert.NotNil(t, cmd, "should return the action command")

		// Execute the returned command to verify it is the action
		cmd()
		assert.True(t, executed, "the action should have been executed")
	})
}

func TestWaitForSelectedIncidentThenDoMsg_TableDriven(t *testing.T) {
	dummyAction := func() tea.Msg { return openBrowserMsg("x") }
	dummyMsg := openBrowserMsg("x")

	tests := []struct {
		name             string
		action           tea.Cmd
		msg              tea.Msg
		selectedIncident *pagerduty.Incident
		viewingIncident  bool
		hasTableRow      bool
		expectCmd        bool
		expectStatusSub  string
	}{
		{
			name:            "nil action",
			action:          nil,
			msg:             dummyMsg,
			expectCmd:       false,
			expectStatusSub: "no action included",
		},
		{
			name:            "nil msg",
			action:          dummyAction,
			msg:             nil,
			expectCmd:       false,
			expectStatusSub: "no data included",
		},
		{
			name:            "cancelled: no incident, not viewing, no row",
			action:          dummyAction,
			msg:             dummyMsg,
			viewingIncident: false,
			hasTableRow:     false,
			expectCmd:       false,
			expectStatusSub: "action cancelled",
		},
		{
			name:            "requeues: no incident, viewing",
			action:          dummyAction,
			msg:             dummyMsg,
			viewingIncident: true,
			hasTableRow:     false,
			expectCmd:       true,
			expectStatusSub: "waiting for incident info",
		},
		{
			name:   "happy: incident available",
			action: dummyAction,
			msg:    dummyMsg,
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "P999"},
			},
			expectCmd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m model
			if tt.hasTableRow {
				m = createTestModelWithSelectedIncident()
				m.selectedIncident = tt.selectedIncident
			} else {
				m = createTestModel()
				m.selectedIncident = tt.selectedIncident
			}
			m.viewingIncident = tt.viewingIncident

			msg := waitForSelectedIncidentThenDoMsg{
				action: tt.action,
				msg:    tt.msg,
			}

			result, cmd := m.Update(msg)
			m = result.(model)

			if tt.expectCmd {
				assert.NotNil(t, cmd, "expected a command")
			} else {
				assert.Nil(t, cmd, "expected no command")
			}

			if tt.expectStatusSub != "" {
				assert.Contains(t, m.status, tt.expectStatusSub)
			}
		})
	}
}
