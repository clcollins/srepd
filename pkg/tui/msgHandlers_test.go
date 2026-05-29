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

	// A command should be returned
	assert.NotNil(t, cmd,
		"Silence key should return a command when a row is highlighted")
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

	// A command should be returned
	assert.NotNil(t, cmd,
		"UnAck key should return a command when a row is highlighted")
}
