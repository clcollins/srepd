package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

func TestMergeIncidents_Success(t *testing.T) {
	t.Run("mergeIncidents returns mergedIncidentMsg on success", func(t *testing.T) {
		mockClient := &pd.MockPagerDutyClient{}
		config := &pd.Config{
			Client:      mockClient,
			CurrentUser: &pagerduty.User{Email: "test@example.com"},
		}

		cmd := mergeIncidents(config, "SOURCE_001", "TARGET_001")
		msg := cmd()

		merged, ok := msg.(mergedIncidentMsg)
		assert.True(t, ok, "should return mergedIncidentMsg")
		assert.Equal(t, "SOURCE_001", merged.sourceID)
		assert.Equal(t, "TARGET_001", merged.targetID)
		assert.NoError(t, merged.err)
	})
}

func TestMergeIncidents_Error(t *testing.T) {
	t.Run("mergeIncidents returns error for invalid target", func(t *testing.T) {
		mockClient := &pd.MockPagerDutyClient{}
		config := &pd.Config{
			Client:      mockClient,
			CurrentUser: &pagerduty.User{Email: "test@example.com"},
		}

		cmd := mergeIncidents(config, "SOURCE_001", "err")
		msg := cmd()

		merged, ok := msg.(mergedIncidentMsg)
		assert.True(t, ok, "should return mergedIncidentMsg")
		assert.Error(t, merged.err)
	})
}

func TestMergeMode_EnterFromMainView(t *testing.T) {
	t.Run("ctrl+m enters merge mode with source incident", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q123"},
			Title:     "Source Incident",
		}
		m.incidentList = []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Source"},
			{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Target"},
		}
		m.config = &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "USER1"},
				Email:     "test@example.com",
			},
		}
		cols := []table.Column{
			{Title: "", Width: 1},
			{Title: "ID", Width: 16},
			{Title: "Title", Width: 40},
			{Title: "Service", Width: 30},
		}
		rows := []table.Row{
			{"•", "Q123", "Source", "svc"},
			{"•", "Q456", "Target", "svc"},
		}
		m.table = table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))

		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}
		result, _ := m.Update(keyMsg)
		updated := result.(model)

		assert.True(t, updated.mergeMode, "should enter merge mode")
		assert.Equal(t, "Q123", updated.mergeSourceIncident.ID, "source incident should be set")
	})
}

func TestMergeMode_EscapeCancels(t *testing.T) {
	t.Run("escape exits merge mode", func(t *testing.T) {
		m := createTestModel()
		m.mergeMode = true
		m.mergeSourceIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q123"},
		}

		keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
		result, _ := m.Update(keyMsg)
		updated := result.(model)

		assert.False(t, updated.mergeMode, "should exit merge mode")
		assert.Nil(t, updated.mergeSourceIncident, "source should be cleared")
	})
}

func TestMergeMode_ExcludesSourceFromList(t *testing.T) {
	t.Run("merge target list excludes source incident", func(t *testing.T) {
		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}, Title: "Source"},
			{APIObject: pagerduty.APIObject{ID: "Q456"}, Title: "Target 1"},
			{APIObject: pagerduty.APIObject{ID: "Q789"}, Title: "Target 2"},
		}

		filtered := filterMergeCandidates(incidents, "Q123")

		assert.Len(t, filtered, 2)
		for _, inc := range filtered {
			assert.NotEqual(t, "Q123", inc.ID, "source should be excluded")
		}
	})
}

func TestMergedIncidentMsg_HandledInModel(t *testing.T) {
	t.Run("mergedIncidentMsg produces flash notification", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "USER1"}},
		}

		msg := mergedIncidentMsg{
			sourceID: "Q123",
			targetID: "Q456",
			err:      nil,
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.status, "Merged Q123 into Q456")
		assert.NotNil(t, cmd, "should return commands for flash and refresh")
	})
}
