package tui

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the top-level View() rendering for each focus mode so
// the UI a user actually sees is verifiable in unit tests: the full render
// path runs against an in-memory model with a mocked PD client — no terminal,
// API, or filesystem required.

// sizedTestModel returns a model with a selected incident and a realistic
// window size applied, so View() lays out like a real terminal session.
func sizedTestModel(t *testing.T) model {
	t.Helper()
	m := createTestModelWithSelectedIncident()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, ok := result.(model)
	require.True(t, ok)

	// The window-size handler resets table columns; restore the incident row
	m.table.SetRows([]table.Row{
		{dot, "P1234567", "Test Alert Firing", "test-service"},
	})
	return m
}

func TestView_TableModeRendersIncidents(t *testing.T) {
	m := sizedTestModel(t)

	view := m.View()

	assert.Contains(t, view, "P1234567", "incident ID should be visible in the table")
	assert.Contains(t, view, "Test Alert Firing", "incident title should be visible in the table")
	assert.Contains(t, view, "test-service", "service name should be visible in the table")
}

func TestView_ErrorModeRendersError(t *testing.T) {
	m := sizedTestModel(t)
	m.err = errors.New("something went sideways")

	view := m.View()

	assert.Contains(t, view, "ERROR", "error mode should show the error banner")
	assert.Contains(t, view, "something went sideways", "error text should be rendered")
}

func TestView_IncidentModeRendersTabBar(t *testing.T) {
	m := sizedTestModel(t)
	m.viewingIncident = true
	m.incidentViewer.SetContent("incident body content")

	view := m.View()

	assert.Contains(t, view, "Details", "tab bar should show the Details tab")
	assert.Contains(t, view, "Notes", "tab bar should show the Notes tab")
	assert.Contains(t, view, "incident body content", "viewer content should be rendered")
}

func TestView_LogModeRendersLogContent(t *testing.T) {
	m := sizedTestModel(t)
	m.viewingLog = true
	m.logViewer.SetContent("log line alpha")

	view := m.View()

	assert.Contains(t, view, "log line alpha", "log viewer content should be rendered")
}

func TestView_ClusterSelectModeRendersPrompt(t *testing.T) {
	m := sizedTestModel(t)
	m.clusterSelectMode = true
	m.clusterSelectPrompt = "Select cluster to log into (Enter=select, Esc=cancel):"
	m.clusterSelectTable = table.New(
		table.WithColumns([]table.Column{
			{Title: "Cluster ID", Width: 40},
			{Title: "Service", Width: 30},
		}),
		table.WithRows([]table.Row{{"cluster-abc", "svc-1"}, {"cluster-def", "svc-2"}}),
	)

	view := m.View()

	assert.Contains(t, view, "Select cluster to log into")
	assert.Contains(t, view, "cluster-abc")
	assert.Contains(t, view, "cluster-def")
}

func TestView_EnterOpensIncidentView(t *testing.T) {
	m := sizedTestModel(t)
	m.config.Client = &pd.MockPagerDutyClient{}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := result.(model)
	require.True(t, ok)

	assert.True(t, m.viewingIncident, "Enter on a highlighted row should open the incident view")
	assert.NotNil(t, cmd, "opening an incident should trigger render + fetch commands")
}

func TestView_EscapeReturnsToTable(t *testing.T) {
	m := sizedTestModel(t)
	m.config.Client = &pd.MockPagerDutyClient{}
	m.viewingIncident = true
	m.selectedIncident = &m.incidentList[0]

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m, ok := result.(model)
	require.True(t, ok)

	assert.False(t, m.viewingIncident, "Escape should close the incident view")
	view := m.View()
	assert.Contains(t, view, "P1234567", "table should be visible again after Escape")
}

func TestView_DownNavigationFollowsCursor(t *testing.T) {
	m := sizedTestModel(t)
	m.config.Client = &pd.MockPagerDutyClient{}
	m.incidentList = append(m.incidentList, pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P7654321"},
		Title:     "Second Incident",
		Service:   pagerduty.APIObject{ID: "SVC2", Summary: "svc-2"},
	})
	m.table.SetRows([]table.Row{
		{dot, "P1234567", "Test Alert Firing", "test-service"},
		{dot, "P7654321", "Second Incident", "svc-2"},
	})

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, ok := result.(model)
	require.True(t, ok)

	require.NotNil(t, m.selectedIncident)
	assert.Equal(t, "P7654321", m.selectedIncident.ID, "selection should follow the cursor")
	assert.NotNil(t, cmd, "navigating to an uncached incident should trigger a fetch command")
}

func TestView_ConfirmationRendersModal(t *testing.T) {
	m := sizedTestModel(t)

	m.pendingConfirmation = &confirmActionState{
		prompt: "Silence P1234567? [y/n]",
		action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
	}

	view := m.View()

	assert.Contains(t, view, "Silence P1234567?",
		"modal should contain the confirmation prompt text")
	assert.Contains(t, view, "confirm",
		"modal should contain the hint about confirming")
}

func TestView_ConfirmationNarrowTerminal(t *testing.T) {
	m := createTestModelWithSelectedIncident()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m, ok := result.(model)
	require.True(t, ok)

	m.table.SetRows([]table.Row{
		{dot, "P1234567", "Test Alert Firing", "test-service"},
	})

	m.pendingConfirmation = &confirmActionState{
		prompt: "Silence P1234567? [y/n]",
		action: func() tea.Msg { return silenceSelectedIncidentMsg{} },
	}

	assert.NotPanics(t, func() {
		view := m.View()
		assert.Contains(t, view, "Silence P1234567?",
			"prompt should be visible even on narrow terminal")
	})
}
