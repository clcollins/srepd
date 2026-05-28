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

func TestTableMode_NoteKeyWithNoSelectedIncident(t *testing.T) {
	// Scenario: The table has rows with incidents highlighted, but
	// selectedIncident is nil because the user has not explicitly navigated
	// (no up/down key press to trigger syncSelectedIncidentToHighlightedRow).
	// Pressing Note key should NOT return "no incident selected" - it should
	// sync the highlighted row and proceed to the note action.

	incidents := []pagerduty.Incident{
		{
			APIObject: pagerduty.APIObject{ID: "Q111", HTMLURL: "https://example.pagerduty.com/incidents/Q111"},
			Title:     "Test Alert",
			Service:   pagerduty.APIObject{ID: "SVC1", Summary: "test-service"},
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
			APIObject: pagerduty.APIObject{ID: "Q222", HTMLURL: "https://example.pagerduty.com/incidents/Q222"},
			Title:     "Another Alert",
			Service:   pagerduty.APIObject{ID: "SVC2", Summary: "another-service"},
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
