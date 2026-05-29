package tui

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
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

func TestClusterSelect_DigitSelectsCluster(t *testing.T) {
	m := createTestModel()
	m.clusterSelectMode = true
	m.clusterSelectOptions = []string{"cluster-abc", "cluster-def", "cluster-ghi"}

	// Press '2' to select the second cluster
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	result, cmd := m.Update(keyMsg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.clusterSelectMode,
		"Cluster select mode should be cleared after selection")
	assert.Nil(t, updatedModel.clusterSelectOptions,
		"Cluster select options should be cleared after selection")

	// The command should produce a clusterSelectedMsg with the chosen cluster
	assert.NotNil(t, cmd, "A command should be returned after selection")
	msg := cmd()
	assert.Equal(t, clusterSelectedMsg("cluster-def"), msg,
		"Should select the second cluster")
}

func TestClusterSelect_EscCancels(t *testing.T) {
	m := createTestModel()
	m.clusterSelectMode = true
	m.clusterSelectOptions = []string{"cluster-abc", "cluster-def"}

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

func TestClusterSelect_OutOfRangeIgnored(t *testing.T) {
	m := createTestModel()
	m.clusterSelectMode = true
	m.clusterSelectOptions = []string{"cluster-abc"}

	// Press '5' which is out of range (only 1 option)
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}}
	result, cmd := m.Update(keyMsg)
	updatedModel := result.(model)

	assert.True(t, updatedModel.clusterSelectMode,
		"Cluster select mode should remain active for out-of-range digit")
	assert.NotNil(t, updatedModel.clusterSelectOptions,
		"Cluster select options should remain for out-of-range digit")
	assert.Nil(t, cmd, "No command for out-of-range digit")
}

func TestClusterSelect_OtherKeysIgnored(t *testing.T) {
	m := createTestModel()
	m.clusterSelectMode = true
	m.clusterSelectOptions = []string{"cluster-abc", "cluster-def"}

	// Press 'a' which is not a valid key in cluster selection mode
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	result, cmd := m.Update(keyMsg)
	updatedModel := result.(model)

	assert.True(t, updatedModel.clusterSelectMode,
		"Cluster select mode should remain active for non-digit keys")
	assert.Nil(t, cmd, "No command for non-digit keys")
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

// viewLogKeyMsg returns a tea.KeyMsg that matches the ViewLog key binding (ctrl+d).
func viewLogKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlD}
}

func TestViewLogKey_OpensLogViewer(t *testing.T) {
	// Scenario: Pressing ctrl+d in table mode should trigger a readLogFile command
	// which, when its message arrives, sets viewingLog=true.

	m := createTestModel()
	m.table = newTableWithStyles()
	m.table.Focus()
	m.logFilePath = "/tmp/test-srepd-debug.log"

	// Press ctrl+d
	result, cmd := m.Update(viewLogKeyMsg())
	updatedModel := result.(model)

	// The model should NOT yet be viewingLog (that happens on logFileContentMsg)
	// But a command should be returned (readLogFile)
	assert.NotNil(t, cmd, "ctrl+d should return a command to read the log file")
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
		actionLogTable: newActionLogTable(),
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
	// Scenario: ctrl+d should be registered in the keymap as ViewLog.

	// Check that the binding exists and matches ctrl+d
	keys := defaultKeyMap.ViewLog.Keys()
	assert.Contains(t, keys, "ctrl+d", "ViewLog binding should include ctrl+d")
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
