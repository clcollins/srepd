package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestMapClusterServices(t *testing.T) {
	tests := []struct {
		name     string
		alerts   []pagerduty.IncidentAlert
		expected map[string]string
	}{
		{
			name: "maps cluster IDs to service names",
			alerts: []pagerduty.IncidentAlert{
				{
					Service: pagerduty.APIObject{Summary: "osd-cluster1-hive-cluster"},
					Body: map[string]interface{}{
						"details": map[string]interface{}{
							"cluster_id": "aaaa-1111",
						},
					},
				},
				{
					Service: pagerduty.APIObject{Summary: "prod-deadmanssnitch"},
					Body: map[string]interface{}{
						"details": map[string]interface{}{
							"cluster_id": "bbbb-2222",
						},
					},
				},
			},
			expected: map[string]string{
				"aaaa-1111": "osd-cluster1-hive-cluster",
				"bbbb-2222": "prod-deadmanssnitch",
			},
		},
		{
			name: "first service wins for duplicate cluster IDs",
			alerts: []pagerduty.IncidentAlert{
				{
					Service: pagerduty.APIObject{Summary: "first-service"},
					Body: map[string]interface{}{
						"details": map[string]interface{}{
							"cluster_id": "aaaa-1111",
						},
					},
				},
				{
					Service: pagerduty.APIObject{Summary: "second-service"},
					Body: map[string]interface{}{
						"details": map[string]interface{}{
							"cluster_id": "aaaa-1111",
						},
					},
				},
			},
			expected: map[string]string{
				"aaaa-1111": "first-service",
			},
		},
		{
			name:     "empty alerts returns empty map",
			alerts:   []pagerduty.IncidentAlert{},
			expected: map[string]string{},
		},
		{
			name: "skips alerts without cluster_id",
			alerts: []pagerduty.IncidentAlert{
				{
					Service: pagerduty.APIObject{Summary: "some-service"},
					Body:    map[string]interface{}{},
				},
			},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapClusterServices(tt.alerts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSwitchClusterSelectFocusMode_Enter(t *testing.T) {
	t.Run("Enter selects highlighted cluster", func(t *testing.T) {
		m := createTestModel()
		m.clusterSelectMode = true
		m.clusterSelectOptions = []string{"aaaa-1111", "bbbb-2222"}
		cols := []table.Column{
			{Title: "Cluster ID", Width: 40},
			{Title: "Service", Width: 50},
		}
		rows := []table.Row{
			{"aaaa-1111", "osd-cluster1"},
			{"bbbb-2222", "prod-dms"},
		}
		m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))

		result, cmd := switchClusterSelectFocusMode(m, tea.KeyMsg{Type: tea.KeyEnter})
		updated := result.(model)

		assert.False(t, updated.clusterSelectMode)
		assert.Nil(t, updated.clusterSelectOptions)
		assert.NotNil(t, cmd)
		msg := cmd()
		assert.Equal(t, clusterSelectedMsg("aaaa-1111"), msg)
	})
}

func TestSwitchClusterSelectFocusMode_Escape(t *testing.T) {
	t.Run("Escape cancels cluster selection", func(t *testing.T) {
		m := createTestModel()
		m.clusterSelectMode = true
		m.clusterSelectOptions = []string{"aaaa-1111"}
		cols := []table.Column{{Title: "Cluster ID", Width: 40}}
		rows := []table.Row{{"aaaa-1111"}}
		m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))

		result, cmd := switchClusterSelectFocusMode(m, tea.KeyMsg{Type: tea.KeyEsc})
		updated := result.(model)

		assert.False(t, updated.clusterSelectMode)
		assert.Nil(t, updated.clusterSelectOptions)
		assert.Contains(t, updated.status, "cancelled")
		assert.Nil(t, cmd)
	})
}

func TestSwitchClusterSelectFocusMode_Quit(t *testing.T) {
	t.Run("Quit key exits app from cluster select", func(t *testing.T) {
		m := createTestModel()
		m.clusterSelectMode = true
		cols := []table.Column{{Title: "Cluster ID", Width: 40}}
		m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithFocused(true))

		_, cmd := switchClusterSelectFocusMode(m, tea.KeyMsg{Type: tea.KeyCtrlC})

		assert.NotNil(t, cmd)
	})
}

func TestSwitchClusterSelectFocusMode_Navigation(t *testing.T) {
	t.Run("other keys pass through to table", func(t *testing.T) {
		m := createTestModel()
		m.clusterSelectMode = true
		cols := []table.Column{{Title: "Cluster ID", Width: 40}}
		rows := []table.Row{{"aaaa-1111"}, {"bbbb-2222"}}
		m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))

		result, _ := switchClusterSelectFocusMode(m, tea.KeyMsg{Type: tea.KeyDown})
		updated := result.(model)

		assert.True(t, updated.clusterSelectMode, "should stay in cluster select mode")
	})
}

func TestSwitchClusterSelectFocusMode_EmptyTable(t *testing.T) {
	t.Run("Enter with empty table shows no cluster selected", func(t *testing.T) {
		m := createTestModel()
		m.clusterSelectMode = true
		cols := []table.Column{{Title: "Cluster ID", Width: 40}}
		m.clusterSelectTable = table.New(table.WithColumns(cols), table.WithFocused(true))

		result, cmd := switchClusterSelectFocusMode(m, tea.KeyMsg{Type: tea.KeyEnter})
		updated := result.(model)

		assert.Contains(t, updated.status, "no cluster selected")
		assert.Nil(t, cmd)
	})
}
