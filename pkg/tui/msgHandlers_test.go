package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
)

// createTestModelWithTableRows creates a model with a focused table containing
// the provided incidents as rows, and populates the incidentList to match.
func createTestModelWithTableRows(incidents []pagerduty.Incident) model {
	m := createTestModel()
	m.incidentList = incidents

	cols := []table.Column{
		{Title: "Status", Width: 2},
		{Title: "ID", Width: 10},
		{Title: "Summary", Width: 30},
		{Title: "Service", Width: 20},
	}

	var rows []table.Row
	for _, inc := range incidents {
		rows = append(rows, table.Row{".", inc.ID, inc.Title, inc.Service.Summary})
	}

	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	return m
}

// noteKeyMsg returns a tea.KeyMsg that matches the Note key binding ('n').
func noteKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
}

// openKeyMsg returns a tea.KeyMsg that matches the Open key binding ('o').
func openKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}
}

// ackKeyMsg returns a tea.KeyMsg that matches the Ack key binding ('a').
func ackKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
}

// silenceKeyMsg returns a tea.KeyMsg that matches the Silence key binding (ctrl+s).
func silenceKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlS}
}

// unAckKeyMsg returns a tea.KeyMsg that matches the UnAck key binding (ctrl+e).
func unAckKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlE}
}

func TestTableMode_NoteKeyWithNoSelectedIncident(t *testing.T) {
	// Scenario: The table has rows with incidents highlighted, but
	// selectedIncident is nil because the user has not explicitly navigated
	// (no up/down key press to trigger syncSelectedIncidentToHighlightedRow).
	// Pressing Note key should NOT return "no incident selected" - it should
	// sync the highlighted row and proceed to the note action.

	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111", HTMLURL: "https://example.pagerduty.com/incidents/Q111"},
			Title:              "Test Alert",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	m := createTestModelWithTableRows(incidents)
	// Explicitly ensure selectedIncident is nil to simulate the bug condition
	m.selectedIncident = nil

	// Press the Note key
	result, cmd := m.Update(noteKeyMsg())
	updatedModel := result.(model)

	// The status should NOT be "no incident selected" - that was the bug
	assert.NotEqual(t, "no incident selected", updatedModel.status,
		"Note key should not report 'no incident selected' when a row is highlighted")

	// A command should be returned (parseTemplateForNote or equivalent)
	assert.NotNil(t, cmd,
		"Note key should return a command when a row is highlighted")
}

func TestTableMode_NoteKeyWithSelectedIncident(t *testing.T) {
	// Scenario: selectedIncident is already set (user navigated explicitly).
	// Pressing Note key should proceed to parseTemplateForNoteMsg.

	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q222", HTMLURL: "https://example.pagerduty.com/incidents/Q222"},
			Title:              "Another Alert",
			Service:            pagerduty.APIObject{ID: "SVC2", Summary: "another-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	m := createTestModelWithTableRows(incidents)
	// Set selectedIncident as if user had navigated
	m.selectedIncident = &incidents[0]

	// Press the Note key
	result, cmd := m.Update(noteKeyMsg())
	updatedModel := result.(model)

	// Should not set an error status
	assert.NotEqual(t, "no incident selected", updatedModel.status,
		"Note key should not report 'no incident selected' when selectedIncident is set")

	// A command should be returned (parseTemplateForNote)
	assert.NotNil(t, cmd,
		"Note key should return a command when selectedIncident is set")
}

func TestTableMode_NoteKeyWithNoRows(t *testing.T) {
	// Scenario: Table has no rows at all. Pressing Note key should gracefully
	// handle this (status message about no incident, no panic).

	m := createTestModel()
	m.table = table.New(table.WithFocused(true))

	result, cmd := m.Update(noteKeyMsg())
	updatedModel := result.(model)

	// Should set some status about no incident being available
	assert.Contains(t, updatedModel.status, "no incident",
		"Note key with empty table should report no incident")

	// No command should be returned
	assert.Nil(t, cmd,
		"Note key with empty table should not return a command")
}

func TestTableMode_OpenKeyWithNoSelectedIncident(t *testing.T) {
	// Scenario: The table has rows with incidents highlighted, but
	// selectedIncident is nil because the user has not explicitly navigated
	// (no up/down key press to trigger syncSelectedIncidentToHighlightedRow).
	// Pressing Open key should NOT return "no incident selected" - it should
	// sync the highlighted row and proceed to the open action.

	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111", HTMLURL: "https://example.pagerduty.com/incidents/Q111"},
			Title:              "Test Alert",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	m := createTestModelWithTableRows(incidents)
	// Explicitly ensure selectedIncident is nil to simulate the bug condition
	m.selectedIncident = nil

	// Press the Open key
	result, cmd := m.Update(openKeyMsg())
	updatedModel := result.(model)

	// The status should NOT be "no incident selected" - that was the bug
	assert.NotEqual(t, "no incident selected", updatedModel.status,
		"Open key should not report 'no incident selected' when a row is highlighted")

	// A command should be returned (openBrowserCmd or equivalent)
	assert.NotNil(t, cmd,
		"Open key should return a command when a row is highlighted")
}

func TestTableMode_OpenKeyWithNoRows(t *testing.T) {
	// Scenario: Table has no rows at all. Pressing Open key should gracefully
	// handle this (status message about no incident, no panic).

	m := createTestModel()
	m.table = table.New(table.WithFocused(true))

	result, cmd := m.Update(openKeyMsg())
	updatedModel := result.(model)

	// Should set some status about no incident being available
	assert.Contains(t, updatedModel.status, "no incident",
		"Open key with empty table should report no incident")

	// No command should be returned
	assert.Nil(t, cmd,
		"Open key with empty table should not return a command")
}

func TestTableMode_AckKeyWithNoSelectedIncident(t *testing.T) {
	// Scenario: The table has rows with incidents highlighted, but
	// selectedIncident is nil. Pressing Ack key should sync the highlighted
	// row and proceed. The message handler will resolve the incident.

	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111", HTMLURL: "https://example.pagerduty.com/incidents/Q111"},
			Title:              "Test Alert",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	m := createTestModelWithTableRows(incidents)
	m.selectedIncident = nil

	// Press the Ack key
	result, cmd := m.Update(ackKeyMsg())
	updatedModel := result.(model)

	// After pressing Ack, selectedIncident should have been synced
	assert.NotNil(t, updatedModel.selectedIncident,
		"Ack key should sync selectedIncident to highlighted row")
	assert.Equal(t, "Q111", updatedModel.selectedIncident.ID,
		"Synced selectedIncident should match highlighted row")

	// A command should be returned
	assert.NotNil(t, cmd,
		"Ack key should return a command when a row is highlighted")
}

func TestTableMode_SilenceKeyWithNoSelectedIncident(t *testing.T) {
	// Scenario: The table has rows with incidents highlighted, but
	// selectedIncident is nil. Pressing Silence key should sync the highlighted
	// row and proceed.

	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111", HTMLURL: "https://example.pagerduty.com/incidents/Q111"},
			Title:              "Test Alert",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	m := createTestModelWithTableRows(incidents)
	m.selectedIncident = nil

	// Press the Silence key
	result, cmd := m.Update(silenceKeyMsg())
	updatedModel := result.(model)

	// After pressing Silence, selectedIncident should have been synced
	assert.NotNil(t, updatedModel.selectedIncident,
		"Silence key should sync selectedIncident to highlighted row")
	assert.Equal(t, "Q111", updatedModel.selectedIncident.ID,
		"Synced selectedIncident should match highlighted row")

	// Silence uses confirmation prompt, so pendingConfirmation should be set
	assert.NotNil(t, updatedModel.pendingConfirmation,
		"Silence key should set pendingConfirmation")
	assert.Nil(t, cmd,
		"Silence key should not return a command before confirmation")
}

// enterKeyMsg returns a tea.KeyMsg that matches the Enter key binding.
func enterKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

// sopKeyMsg returns a tea.KeyMsg that matches the SOP key binding ('s').
func sopKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
}

// loginKeyMsg returns a tea.KeyMsg that matches the Login key binding ('l').
func loginKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
}

// refreshKeyMsg returns a tea.KeyMsg that matches the Refresh key binding ('r').
func refreshKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
}

func TestTableMode_EnterKeyWithNoRows(t *testing.T) {
	// Scenario: Table has no rows at all. Pressing Enter key should gracefully
	// report "no incident highlighted" instead of opening an empty incident viewer.

	m := createTestModel()
	m.table = table.New(table.WithFocused(true))

	result, cmd := m.Update(enterKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident",
		"Enter key with empty table should report no incident")
	assert.False(t, updatedModel.viewingIncident,
		"Should not open incident viewer when no row is highlighted")
	assert.Nil(t, cmd,
		"Enter key with empty table should not return a command")
}

func TestTableMode_SOPKeyWithNoRows(t *testing.T) {
	// Scenario: Table has no rows at all. Pressing SOP key should gracefully
	// handle this (status message about no incident, no panic).

	m := createTestModel()
	m.table = table.New(table.WithFocused(true))

	result, cmd := m.Update(sopKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident",
		"SOP key with empty table should report no incident")
	assert.Nil(t, cmd,
		"SOP key with empty table should not return a command")
}

func TestTableMode_SOPKeyWithNoSelectedIncident(t *testing.T) {
	// Scenario: The table has rows with incidents highlighted, but
	// selectedIncident is nil. Pressing SOP key should sync the highlighted
	// row and proceed (sync happens inside the handler).

	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111", HTMLURL: "https://example.pagerduty.com/incidents/Q111"},
			Title:              "Test Alert",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	m := createTestModelWithTableRows(incidents)
	m.selectedIncident = nil

	// Press the SOP key
	result, _ := m.Update(sopKeyMsg())
	updatedModel := result.(model)

	// The status should NOT be "no incident highlighted" - there IS a row highlighted
	assert.NotEqual(t, "no incident highlighted", updatedModel.status,
		"SOP key should not report 'no incident highlighted' when a row is highlighted")
}

func TestTableMode_LoginKeyWithNoRows(t *testing.T) {
	// Scenario: Table has no rows at all. Pressing Login key should gracefully
	// handle this via doIfIncidentSelected.

	m := createTestModel()
	m.table = table.New(table.WithFocused(true))

	result, cmd := m.Update(loginKeyMsg())
	updatedModel := result.(model)

	// doIfIncidentSelected checks SelectedRow() and returns a setStatusMsg
	// The cmd when executed should produce a "no incident selected" status
	assert.NotNil(t, cmd, "Login key should return a status command even with no rows")
	msg := cmd()
	statusMsg, ok := msg.(setStatusMsg)
	assert.True(t, ok, "Should return a setStatusMsg")
	assert.Contains(t, statusMsg.string, "no incident",
		"Should indicate no incident selected")

	_ = updatedModel
}

// --- Incident view mode: actions with nil selectedIncident ---

func TestIncidentViewMode_AckWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(ackKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"Ack in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestIncidentViewMode_UnAckWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(unAckKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"UnAck in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestIncidentViewMode_SilenceWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(silenceKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"Silence in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestIncidentViewMode_NoteWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(noteKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"Note in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestIncidentViewMode_LoginWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(loginKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"Login in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestIncidentViewMode_OpenWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(openKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"Open in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestIncidentViewMode_SOPWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(sopKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"SOP in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestIncidentViewMode_RefreshWithNilSelectedIncident(t *testing.T) {
	m := createTestModel()
	m.viewingIncident = true
	m.selectedIncident = nil

	result, cmd := m.Update(refreshKeyMsg())
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "no incident selected",
		"Refresh in incident view with nil selectedIncident should report no incident selected")
	assert.Nil(t, cmd,
		"No command should be returned when selectedIncident is nil")
}

func TestClusterSelect_EnterSelectsCluster(t *testing.T) {
	m := createTestModel()
	m.clusterSelectMode = true
	m.clusterSelectOptions = []string{"cluster-abc", "cluster-def", "cluster-ghi"}
	cols := []table.Column{{Title: "Cluster ID", Width: 40}}
	rows := []table.Row{{"cluster-abc"}, {"cluster-def"}, {"cluster-ghi"}}
	m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(keyMsg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.clusterSelectMode,
		"Cluster select mode should be cleared after selection")
	assert.Nil(t, updatedModel.clusterSelectOptions,
		"Cluster select options should be cleared after selection")
	assert.NotNil(t, cmd, "A command should be returned after selection")
	msg := cmd()
	assert.Equal(t, clusterSelectedMsg("cluster-abc"), msg,
		"Should select the highlighted cluster")
}

func TestClusterSelect_EscCancels(t *testing.T) {
	m := createTestModel()
	m.clusterSelectMode = true
	m.clusterSelectOptions = []string{"cluster-abc", "cluster-def"}
	cols := []table.Column{{Title: "Cluster ID", Width: 40}}
	rows := []table.Row{{"cluster-abc"}, {"cluster-def"}}
	m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))

	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	result, cmd := m.Update(keyMsg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.clusterSelectMode,
		"Cluster select mode should be cleared on Escape")
	assert.Nil(t, updatedModel.clusterSelectOptions,
		"Cluster select options should be cleared on Escape")
	assert.Contains(t, updatedModel.status, "cancelled",
		"Status should indicate cancellation")
	assert.Nil(t, cmd, "No command should be returned on cancel")
}

func TestClusterSelect_ClearedOnViewTransition(t *testing.T) {
	m := createTestModel()
	m.clusterSelectMode = true
	m.clusterSelectOptions = []string{"cluster-abc", "cluster-def"}

	m.clearSelectedIncident("test")

	assert.False(t, m.clusterSelectMode,
		"Cluster select mode should be cleared on view transition")
	assert.Nil(t, m.clusterSelectOptions,
		"Cluster select options should be cleared on view transition")
}

// viewLogKeyMsg returns a tea.KeyMsg that matches the ViewLog key binding (ctrl+l).
func viewLogKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlL}
}

func TestViewLogKey_OpensLogViewer(t *testing.T) {
	// Scenario: Pressing ctrl+l in table mode should trigger a readLogFile command
	// which, when its message arrives, sets viewingLog=true.

	m := createTestModel()
	m.table = newTableWithStyles()
	m.table.Focus()
	m.logFilePath = "/tmp/test-srepd-debug.log"

	// Press ctrl+l
	result, cmd := m.Update(viewLogKeyMsg())
	updatedModel := result.(model)

	// The model should NOT yet be viewingLog (that happens on logFileContentMsg)
	// But a command should be returned (readLogFile)
	assert.NotNil(t, cmd, "ctrl+l should return a command to read the log file")
	assert.False(t, updatedModel.viewingLog, "viewingLog should not be set until content arrives")
}

func TestViewLogKey_EscapeCloses(t *testing.T) {
	// Scenario: When viewingLog is true, pressing Escape should dismiss the log viewer.

	m := createTestModel()
	m.viewingLog = true
	m.logViewer = newLogViewer()

	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	result, cmd := m.Update(escMsg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.viewingLog, "Escape should dismiss the log viewer")
	assert.Nil(t, cmd, "No command should be returned after dismissing log viewer")
}

func TestLogFileContentMsg_SetsViewport(t *testing.T) {
	// Scenario: When logFileContentMsg arrives, it should set viewport content
	// and set viewingLog=true.

	m := createTestModel()
	m.logViewer = newLogViewer()

	content := "2025-01-01 DEBUG test log entry\n2025-01-02 INFO another entry"
	msg := logFileContentMsg(content)

	result, cmd := m.Update(msg)
	updatedModel := result.(model)

	assert.True(t, updatedModel.viewingLog, "viewingLog should be true after content arrives")
	assert.Nil(t, cmd, "No further command should be returned")
}

func TestLogFileContentMsg_FileNotFound(t *testing.T) {
	// Scenario: When logFileContentMsg arrives with a "file not found" message,
	// it should still display in the viewport.

	m := createTestModel()
	m.logViewer = newLogViewer()

	content := "No log file found at /tmp/nonexistent.log"
	msg := logFileContentMsg(content)

	result, _ := m.Update(msg)
	updatedModel := result.(model)

	assert.True(t, updatedModel.viewingLog, "viewingLog should be true even for error messages")
}

func TestLogViewer_WindowResize(t *testing.T) {
	// Scenario: WindowSizeMsg should set logViewer dimensions.

	m := model{
		table:          newTableWithStyles(),
		incidentViewer: newIncidentViewer(),
		logViewer:      newLogViewer(),
		help:           newHelp(),
		incidentCache:  make(map[string]*cachedIncidentData),
	}

	msg := tea.WindowSizeMsg{Width: 120, Height: 50}
	result, _ := m.windowSizeMsgHandler(msg)
	updatedModel := result.(model)

	// logViewer should have the same dimensions as incidentViewer
	assert.Equal(t, updatedModel.incidentViewer.Width, updatedModel.logViewer.Width,
		"logViewer width should match incidentViewer width")
	assert.Equal(t, updatedModel.incidentViewer.Height, updatedModel.logViewer.Height,
		"logViewer height should match incidentViewer height")
}

func TestViewLogKey_InKeymap(t *testing.T) {
	// Scenario: ctrl+l should be registered in the keymap as ViewLog.

	// Check that the binding exists and matches ctrl+l
	keys := defaultKeyMap.ViewLog.Keys()
	assert.Contains(t, keys, "ctrl+l", "ViewLog binding should include ctrl+l")
}

func TestSetStatusMsgHandler(t *testing.T) {
	tests := []struct {
		name           string
		msg            setStatusMsg
		expectedStatus string
	}{
		{
			name:           "sets status from message",
			msg:            setStatusMsg{"loading incidents..."},
			expectedStatus: "loading incidents...",
		},
		{
			name:           "sets empty status",
			msg:            setStatusMsg{""},
			expectedStatus: "",
		},
		{
			name:           "sets error-like status",
			msg:            setStatusMsg{"no incident selected"},
			expectedStatus: "no incident selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.status = "previous status"

			result, cmd := m.setStatusMsgHandler(tt.msg)
			updatedModel := result.(model)

			assert.Equal(t, tt.expectedStatus, updatedModel.status,
				"status should be set to message value")
			assert.Nil(t, cmd, "setStatusMsgHandler should not return a command")
		})
	}
}

func TestErrMsgHandler(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "sets error from error message",
			err:  fmt.Errorf("test error occurred"),
		},
		{
			name: "handles wrapped error",
			err:  fmt.Errorf("outer: %w", fmt.Errorf("inner error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.status = "previous status"
			m.err = nil

			msg := errMsg{tt.err}
			result, cmd := m.errMsgHandler(msg)
			updatedModel := result.(model)

			// The error is displayed via the full-screen error view (m.err),
			// NOT the transient status line — background polls overwrite the
			// status within seconds, so an error copied there is lost almost
			// immediately.
			assert.Equal(t, "previous status", updatedModel.status,
				"status must not be overwritten with the error text")
			assert.NotNil(t, updatedModel.err, "err should be set on model")
			assert.Equal(t, msg, updatedModel.err, "err should match the errMsg")
			assert.Nil(t, cmd, "errMsgHandler should not return a command")
		})
	}
}

func TestTableMode_UnAckKeyWithNoSelectedIncident(t *testing.T) {
	// Scenario: The table has rows with incidents highlighted, but
	// selectedIncident is nil. Pressing UnAck key should sync the highlighted
	// row and proceed.

	incidents := []pagerduty.Incident{
		{
			APIObject:          pagerduty.APIObject{ID: "Q111", HTMLURL: "https://example.pagerduty.com/incidents/Q111"},
			Title:              "Test Alert",
			Service:            pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
			LastStatusChangeAt: time.Now().Format(time.RFC3339),
		},
	}

	m := createTestModelWithTableRows(incidents)
	m.selectedIncident = nil

	// Press the UnAck key
	result, cmd := m.Update(unAckKeyMsg())
	updatedModel := result.(model)

	// After pressing UnAck, selectedIncident should have been synced
	assert.NotNil(t, updatedModel.selectedIncident,
		"UnAck key should sync selectedIncident to highlighted row")
	assert.Equal(t, "Q111", updatedModel.selectedIncident.ID,
		"Synced selectedIncident should match highlighted row")

	// Re-escalate uses confirmation prompt, so pendingConfirmation should be set
	assert.NotNil(t, updatedModel.pendingConfirmation,
		"UnAck key should set pendingConfirmation")
	assert.Nil(t, cmd,
		"UnAck key should not return a command before confirmation")
}

// --- switchInputFocusMode gap tests ---

func TestInputMode_QuitKey(t *testing.T) {
	// Cover the Quit key path in switchInputFocusMode (lines 535-537)
	// The quit key is normally caught by keyMsgHandler before reaching
	// switchInputFocusMode, so we call switchInputFocusMode directly.
	t.Run("ctrl+c in switchInputFocusMode quits the application", func(t *testing.T) {
		m := createTestModel()
		m.input = newTextInput()
		m.input.Focus()

		// Call switchInputFocusMode directly to exercise its quit handler
		quitMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
		_, cmd := switchInputFocusMode(m, quitMsg)

		// The command should be tea.Quit
		assert.NotNil(t, cmd, "ctrl+c in switchInputFocusMode should return a command")
		msg := cmd()
		_, isQuit := msg.(tea.QuitMsg)
		assert.True(t, isQuit, "ctrl+c in switchInputFocusMode should produce a QuitMsg")
	})
}

func TestInputMode_NonKeyMsgReturnsUnchanged(t *testing.T) {
	// Cover the fallthrough return at line 570 in switchInputFocusMode
	t.Run("non-KeyMsg in input mode returns model unchanged", func(t *testing.T) {
		m := createTestModel()
		m.input = newTextInput()
		m.input.Focus()
		m.status = "original status"

		// Send a non-KeyMsg message while input is focused
		// We use the keyMsgHandler path through Update which will delegate to switchInputFocusMode
		// but switchInputFocusMode only handles tea.KeyMsg, so a non-KeyMsg should fall through
		// We need to call switchInputFocusMode directly with a non-KeyMsg
		result, cmd := switchInputFocusMode(m, tea.WindowSizeMsg{Width: 80, Height: 24})
		updatedModel := result.(model)

		assert.Equal(t, "original status", updatedModel.status, "status should be unchanged")
		assert.Nil(t, cmd, "no command should be returned for non-KeyMsg")
	})
}

// --- switchErrorFocusMode gap tests ---

func TestErrorMode_EscapeClearsError(t *testing.T) {
	// Cover the Escape key path in switchErrorFocusMode (lines 794-795)
	t.Run("escape key in error mode clears the error", func(t *testing.T) {
		m := createTestModel()
		m.err = fmt.Errorf("test error")

		// Press Escape
		escMsg := tea.KeyMsg{Type: tea.KeyEscape}
		result, cmd := m.Update(escMsg)
		updatedModel := result.(model)

		assert.Nil(t, updatedModel.err, "error should be cleared after pressing Escape in error mode")
		assert.Nil(t, cmd, "no command should be returned")
	})
}

func TestErrorMode_HelpTogglesFullHelp(t *testing.T) {
	t.Run("h key in error mode toggles full help", func(t *testing.T) {
		m := createTestModel()
		m.err = fmt.Errorf("test error")
		assert.False(t, m.help.ShowAll, "help should start compact")

		helpMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
		result, _ := m.Update(helpMsg)
		updatedModel := result.(model)

		assert.True(t, updatedModel.help.ShowAll, "h should expand help in error mode")
		assert.NotNil(t, updatedModel.err, "toggling help must not clear the error")

		result, _ = updatedModel.Update(helpMsg)
		updatedModel = result.(model)
		assert.False(t, updatedModel.help.ShowAll, "h should collapse help again")
	})
}

func TestRecalculateTableHeight_CompactHelp(t *testing.T) {
	t.Run("24-line terminal with compact help shows usable table height", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		m.help.ShowAll = false
		m.help.Width = 80
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 24}

		m.recomputeLayout()

		assert.GreaterOrEqual(t, m.table.Height(), 10,
			"compact help on 24-line terminal should leave room for data rows")
	})
}

func TestRecalculateTableHeight_ExpandedHelp(t *testing.T) {
	t.Run("24-line terminal with expanded help shows smaller table", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		m.help.ShowAll = true
		m.help.Width = 80
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 24}

		m.recomputeLayout()

		assert.GreaterOrEqual(t, m.table.Height(), 4,
			"expanded help on 24-line terminal should still show some rows")
	})
}

func TestRecalculateTableHeight_CompactTallerThanExpanded(t *testing.T) {
	t.Run("compact help produces taller table than expanded help", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		m.help.Width = 80
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 24}

		m.help.ShowAll = false
		m.recomputeLayout()
		compactHeight := m.table.Height()

		m.help.ShowAll = true
		m.recomputeLayout()
		expandedHeight := m.table.Height()

		assert.Greater(t, compactHeight, expandedHeight,
			"compact help should yield taller table than expanded help")
	})
}

func TestToggleHelp_RecalculatesTableHeight(t *testing.T) {
	t.Run("toggling help changes table height", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		m.help.ShowAll = false
		m.help.Width = 80
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 24}

		m.recomputeLayout()
		compactHeight := m.table.Height()

		m.toggleHelp()
		expandedHeight := m.table.Height()

		assert.Greater(t, compactHeight, expandedHeight,
			"toggling to expanded help should shrink the table")
	})
}

func TestRecalculateTableHeight_VerySmallTerminal(t *testing.T) {
	t.Run("very small terminal keeps viewport height positive", func(t *testing.T) {
		m := createTestModel()
		m.help = newHelp()
		m.help.Width = 80
		windowSize = tea.WindowSizeMsg{Width: 80, Height: 10}

		m.recomputeLayout()

		assert.GreaterOrEqual(t, m.table.Height(), 1,
			"viewport height must remain positive even on very small terminals")
	})
}

func TestErrorMode_NonEscapeKeysDoNotClearError(t *testing.T) {
	t.Run("non-Escape keys do not clear the error", func(t *testing.T) {
		m := createTestModel()
		m.err = fmt.Errorf("test error")

		// Press a non-Escape key (like 'a')
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		result, cmd := m.Update(keyMsg)
		updatedModel := result.(model)

		assert.NotNil(t, updatedModel.err, "error should NOT be cleared by non-Escape key")
		assert.Nil(t, cmd, "no command should be returned")
	})
}

func TestBulkSilenceMode_AbortExitsMode(t *testing.T) {
	t.Run("aborted form exits bulk silence mode", func(t *testing.T) {
		m := createTestModel()
		m.bulkSilenceMode = true

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("test").
					Options(huh.NewOption("test", "Q123")).
					Value(&m.bulkSilenceIDs),
			),
		)
		m.bulkSilenceForm = form
		m.bulkSilenceForm.Init()
		m.bulkSilenceForm.State = huh.StateAborted

		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
		result, _ := switchBulkSilenceFocusMode(m, keyMsg)
		updated := result.(model)

		assert.False(t, updated.bulkSilenceMode, "should exit bulk silence mode")
		assert.Contains(t, updated.status, "cancelled")
	})
}

func TestBulkSilenceMode_ConfirmationPrompt(t *testing.T) {
	t.Run("completing form with selections shows confirmation prompt", func(t *testing.T) {
		m := createTestModel()
		m.bulkSilenceMode = true
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Test 1"},
			{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Test 2"},
		}
		m.bulkSilenceIDs = []string{"Q123", "Q456"}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("test").
					Options(
						huh.NewOption("test1", "Q123"),
						huh.NewOption("test2", "Q456"),
					).
					Value(&m.bulkSilenceIDs),
			),
		)
		m.bulkSilenceForm = form
		m.bulkSilenceForm.Init()

		// Simulate form completion by setting state
		m.bulkSilenceForm.State = huh.StateCompleted

		// Send any key to trigger the handler
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		result, _ := switchBulkSilenceFocusMode(m, keyMsg)
		updated := result.(model)

		assert.False(t, updated.bulkSilenceMode, "should exit bulk silence mode")
		assert.NotNil(t, updated.pendingConfirmation, "should show confirmation prompt")
		assert.Contains(t, updated.pendingConfirmation.prompt, "2 incident(s)")
	})
}

// createInputFocusedModel creates a test model with a focused text input
// containing the given initial value. Used for testing that global key
// bindings are bypassed during input mode.
func createInputFocusedModel(initialValue string) model {
	m := createTestModel()
	m.input = newTextInput()
	m.input.Focus()
	if initialValue != "" {
		m.input.SetValue(initialValue)
		m.input.SetCursor(len(initialValue))
	}
	return m
}

func TestInputMode_TypingU_DoesNotToggleUrgency(t *testing.T) {
	m := createInputFocusedModel("")
	m.showLowUrgency = false

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
	result, _ := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.showLowUrgency, "pressing 'u' in input mode must not toggle urgency")
	assert.True(t, updated.input.Focused(), "input must remain focused")
	assert.Contains(t, updated.input.Value(), "u", "input value must contain the typed 'u'")
}

func TestInputMode_TypingI_DoesNotReFocusInput(t *testing.T) {
	m := createInputFocusedModel("test")

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	result, _ := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.True(t, updated.input.Focused(), "input must remain focused")
	assert.Contains(t, updated.input.Value(), "i", "input value must contain the typed 'i'")
}

func TestInputMode_TypingColon_DoesNotReFocusInput(t *testing.T) {
	m := createInputFocusedModel("test")

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}}
	result, _ := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.True(t, updated.input.Focused(), "input must remain focused")
	assert.Contains(t, updated.input.Value(), ":", "input value must contain the typed ':'")
}

func TestInputMode_CtrlR_DoesNotToggleAutoRefresh(t *testing.T) {
	m := createInputFocusedModel("test")
	m.autoRefresh = false

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlR}
	result, _ := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.autoRefresh, "ctrl+r in input mode must not toggle autoRefresh")
	assert.True(t, updated.input.Focused(), "input must remain focused")
}

func TestInputMode_CtrlA_DoesNotToggleAutoAck(t *testing.T) {
	m := createInputFocusedModel("test")
	m.autoAcknowledge = false

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlA}
	result, _ := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.autoAcknowledge, "ctrl+a in input mode must not toggle autoAcknowledge")
	assert.True(t, updated.input.Focused(), "input must remain focused")
}

func TestInputMode_CtrlC_StillQuits(t *testing.T) {
	m := createInputFocusedModel("test")

	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.keyMsgHandler(keyMsg)

	assert.NotNil(t, cmd, "ctrl+c must still produce a command")
	msg := cmd()
	_, isQuit := msg.(tea.QuitMsg)
	assert.True(t, isQuit, "ctrl+c in input mode must still quit")
}

func TestInputMode_Escape_StillExitsInput(t *testing.T) {
	m := createInputFocusedModel("test")

	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.input.Focused(), "Escape must blur the input")
	assert.Empty(t, updated.input.Value(), "Escape must reset the input value")
}

func TestInputMode_Enter_StillDispatchesPrompt(t *testing.T) {
	m := createInputFocusedModel(":agent investigate this alert")

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.input.Focused(), "input should blur after agent dispatch")
	assert.Empty(t, updated.input.Value(), "Enter must reset the input value")
	assert.NotNil(t, cmd, "Enter must dispatch a command")

	msg := cmd()
	promptMsg, ok := msg.(claudePromptMsg)
	assert.True(t, ok, "dispatched message must be claudePromptMsg")
	assert.Equal(t, "investigate this alert", promptMsg.prompt)
}
