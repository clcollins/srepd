package tui

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func goldenTestModel(t *testing.T) model {
	t.Helper()
	m := createTestModelWithSelectedIncident()
	m.config.Client = &pd.MockPagerDutyClient{}

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	var ok bool
	m, ok = result.(model)
	require.True(t, ok)

	m.table.SetRows([]table.Row{
		{dot, "P1234567", "Test Alert Firing", "test-service"},
		{dot, "P7654321", "Database CPU High", "prod-db"},
	})
	return m
}

func TestGolden_TableMode(t *testing.T) {
	m := goldenTestModel(t)
	golden.RequireEqual(t, m.View())
}

func TestGolden_ErrorMode(t *testing.T) {
	m := goldenTestModel(t)
	m.err = errors.New("failed to fetch incidents: connection refused")
	golden.RequireEqual(t, m.View())
}

func TestGolden_IncidentMode(t *testing.T) {
	m := goldenTestModel(t)
	m.viewingIncident = true
	m.incidentViewer.SetContent("## Incident P1234567\n\nTest Alert Firing\n\nService: test-service\nUrgency: high")
	golden.RequireEqual(t, m.View())
}

func TestGolden_LogMode(t *testing.T) {
	m := goldenTestModel(t)
	m.viewingLog = true
	m.logViewer.SetContent("2026-07-14T10:00:00Z [INFO] incident fetched\n2026-07-14T10:00:01Z [DEBUG] cache hit for P1234567")
	golden.RequireEqual(t, m.View())
}

func TestGolden_ClusterSelectMode(t *testing.T) {
	m := goldenTestModel(t)
	m.clusterSelectMode = true
	m.clusterSelectPrompt = "Select cluster to log into (Enter=select, Esc=cancel):"
	m.clusterSelectTable = table.New(
		table.WithColumns([]table.Column{
			{Title: "Cluster ID", Width: 40},
			{Title: "Service", Width: 30},
		}),
		table.WithRows([]table.Row{
			{"cluster-abc-123", "test-service"},
			{"cluster-def-456", "prod-db"},
		}),
	)
	golden.RequireEqual(t, m.View())
}

func TestGolden_ConfigModeRequested(t *testing.T) {
	m := goldenTestModel(t)
	m.configModeRequested = true
	golden.RequireEqual(t, m.View())
}

func TestGolden_ConfigMode(t *testing.T) {
	m := goldenTestModel(t)
	m.configMode = true

	var dummyVal string
	m.configForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("PagerDuty Token").
				Value(&dummyVal),
		),
	)
	m.configForm.Init()

	golden.RequireEqual(t, m.View())
}

func TestGolden_BulkSilenceMode(t *testing.T) {
	m := goldenTestModel(t)
	m.bulkSilenceMode = true

	var selected []string
	m.bulkSilenceForm = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select incidents to silence").
				Options(
					huh.NewOption("P1234567 - Test Alert", "P1234567"),
					huh.NewOption("P7654321 - Database CPU", "P7654321"),
				).
				Value(&selected),
		),
	)
	m.bulkSilenceForm.Init()

	golden.RequireEqual(t, m.View())
}

func TestGolden_TeamSelectMode(t *testing.T) {
	m := goldenTestModel(t)
	m.teamSelectMode = true

	var selected []string
	m.teamSelectForm = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select teams").
				Options(
					huh.NewOption("SRE Platform", "TEAM1"),
					huh.NewOption("Observability", "TEAM2"),
				).
				Value(&selected),
		),
	)
	m.teamSelectForm.Init()

	golden.RequireEqual(t, m.View())
}

func TestGolden_MergeMode(t *testing.T) {
	m := goldenTestModel(t)
	m.mergeMode = true
	m.mergeSourceIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P1234567"},
		Title:     "Test Alert Firing",
	}
	m.mergeTable = table.New(
		table.WithColumns(m.table.Columns()),
		table.WithRows([]table.Row{
			{dot, "P7654321", "Database CPU High", "prod-db"},
		}),
	)
	m.mergeTable.Focus()

	golden.RequireEqual(t, m.View())
}

func TestGolden_DocsMode(t *testing.T) {
	m := goldenTestModel(t)
	m.viewingDocs = true
	m.docsViewer.SetContent("# srepd Documentation\n\nWelcome to the docs viewer.")
	golden.RequireEqual(t, m.View())
}

func TestGolden_TourMode(t *testing.T) {
	m := goldenTestModel(t)
	m.tourMode = true
	m.tourStep = 0
	golden.RequireEqual(t, m.View())
}
