package tui

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

// --- noAcknowledgeMsg ---

func TestNoAcknowledgeMsg_IsNoOp(t *testing.T) {
	t.Run("returns model unchanged and nil cmd", func(t *testing.T) {
		m := createTestModel()
		m.status = "original status"

		result, cmd := m.Update(noAcknowledgeMsg{})
		updated := result.(model)

		assert.Equal(t, "original status", updated.status, "status should not change")
		assert.Nil(t, cmd, "should return nil cmd")
	})
}

// --- acknowledgeIncidentsMsg ---

func TestAcknowledgeIncidentsMsg_WithIncidents(t *testing.T) {
	t.Run("uses provided incidents and sets apiInProgress", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q100"}},
			{APIObject: pagerduty.APIObject{ID: "Q200"}},
		}

		result, cmd := m.Update(acknowledgeIncidentsMsg{incidents: incidents})
		updated := result.(model)

		assert.True(t, updated.apiInProgress, "apiInProgress should be true")
		assert.NotNil(t, cmd, "should return a sequence command")
	})
}

func TestAcknowledgeIncidentsMsg_FallbackToSelected(t *testing.T) {
	t.Run("uses selectedIncident when no incidents provided", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		result, cmd := m.Update(acknowledgeIncidentsMsg{incidents: nil})
		updated := result.(model)

		assert.True(t, updated.apiInProgress, "apiInProgress should be true")
		assert.NotNil(t, cmd, "should return a command")
	})
}

func TestAcknowledgeIncidentsMsg_NoIncidentNoSelected(t *testing.T) {
	t.Run("sets error status when no incidents and no selectedIncident", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}
		m.selectedIncident = nil

		result, cmd := m.Update(acknowledgeIncidentsMsg{incidents: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "failed acknowledging")
		assert.Nil(t, cmd, "should return nil cmd on guard failure")
		assert.False(t, updated.apiInProgress, "apiInProgress should remain false")
	})
}

// --- unAcknowledgeIncidentsMsg ---

func TestUnAcknowledgeIncidentsMsg_WithIncidents(t *testing.T) {
	t.Run("groups by escalation policy and sets apiInProgress", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		incidents := []pagerduty.Incident{
			{
				APIObject:        pagerduty.APIObject{ID: "Q100"},
				EscalationPolicy: pagerduty.APIObject{ID: "POL_A"},
			},
			{
				APIObject:        pagerduty.APIObject{ID: "Q200"},
				EscalationPolicy: pagerduty.APIObject{ID: "POL_B"},
			},
		}

		result, cmd := m.Update(unAcknowledgeIncidentsMsg{incidents: incidents})
		updated := result.(model)

		assert.True(t, updated.apiInProgress, "apiInProgress should be true")
		assert.NotNil(t, cmd, "should return a sequence command")
	})
}

func TestUnAcknowledgeIncidentsMsg_FallbackToSelected(t *testing.T) {
	t.Run("uses selectedIncident when no incidents provided", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}
		// Ensure selectedIncident has an escalation policy
		m.selectedIncident.EscalationPolicy = pagerduty.APIObject{ID: "POL123"}

		result, cmd := m.Update(unAcknowledgeIncidentsMsg{incidents: nil})
		updated := result.(model)

		assert.True(t, updated.apiInProgress, "apiInProgress should be true")
		assert.NotNil(t, cmd, "should return a command")
	})
}

func TestUnAcknowledgeIncidentsMsg_NoIncidentNoSelected(t *testing.T) {
	t.Run("sets error status when no incidents and no selectedIncident", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}
		m.selectedIncident = nil

		result, cmd := m.Update(unAcknowledgeIncidentsMsg{incidents: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "failed re-escalating")
		assert.Nil(t, cmd, "should return nil cmd on guard failure")
	})
}

func TestUnAcknowledgeIncidentsMsg_UsesReescalateLevel(t *testing.T) {
	t.Run("uses configured reescalateLevel when nonzero", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}
		m.reescalateLevel = 3

		incidents := []pagerduty.Incident{
			{
				APIObject:        pagerduty.APIObject{ID: "Q100"},
				EscalationPolicy: pagerduty.APIObject{ID: "POL_A"},
			},
		}

		result, cmd := m.Update(unAcknowledgeIncidentsMsg{incidents: incidents})
		updated := result.(model)

		assert.True(t, updated.apiInProgress, "apiInProgress should be true")
		assert.NotNil(t, cmd, "should return commands")
	})
}

func TestUnAcknowledgeIncidentsMsg_NoEscalationPolicy(t *testing.T) {
	t.Run("incidents without escalation policy produce fallback cmd", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		// Incident with empty escalation policy ID
		incidents := []pagerduty.Incident{
			{
				APIObject:        pagerduty.APIObject{ID: "Q100"},
				EscalationPolicy: pagerduty.APIObject{ID: ""},
			},
		}

		result, cmd := m.Update(unAcknowledgeIncidentsMsg{incidents: incidents})
		_ = result.(model)

		// When no policy groups are created (empty EP ID), the cmds slice is empty,
		// so we fall through to the updateIncidentListMsg return path
		assert.NotNil(t, cmd, "should return updateIncidentListMsg fallback cmd")
	})
}

// --- acknowledgedIncidentsMsg ---

func TestAcknowledgedIncidentsMsg_Success(t *testing.T) {
	t.Run("clears apiInProgress and returns flash and refresh", func(t *testing.T) {
		m := createTestModel()
		m.apiInProgress = true
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}

		msg := acknowledgedIncidentsMsg{
			incidents: []pagerduty.Incident{
				{APIObject: pagerduty.APIObject{ID: "Q123"}},
			},
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.False(t, updated.apiInProgress, "apiInProgress should be false")
		assert.NotNil(t, cmd, "should return batch with flash and refresh")
	})
}

func TestAcknowledgedIncidentsMsg_Error(t *testing.T) {
	t.Run("clears apiInProgress and returns errMsg", func(t *testing.T) {
		m := createTestModel()
		m.apiInProgress = true
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}

		msg := acknowledgedIncidentsMsg{
			err: errors.New("acknowledge failed"),
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.False(t, updated.apiInProgress, "apiInProgress should be false")
		assert.NotNil(t, cmd, "should return errMsg cmd")

		// Execute the cmd to verify it produces an errMsg
		cmdResult := cmd()
		errResult, ok := cmdResult.(errMsg)
		assert.True(t, ok, "should produce errMsg")
		assert.Error(t, errResult.error, "should contain the error")
	})
}

// --- reassignIncidentsMsg ---

func TestReassignIncidentsMsg_Success(t *testing.T) {
	t.Run("returns sequence with reassign and clear commands", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}},
		}
		users := []*pagerduty.User{
			{APIObject: pagerduty.APIObject{ID: "U456"}, Email: "user@example.com"},
		}

		result, cmd := m.Update(reassignIncidentsMsg{incidents: incidents, users: users})
		_ = result.(model)

		assert.NotNil(t, cmd, "should return sequence command")
	})
}

func TestReassignIncidentsMsg_NilIncidents(t *testing.T) {
	t.Run("sets error status when incidents are nil", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		result, cmd := m.Update(reassignIncidentsMsg{incidents: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "failed reassigning")
		assert.Nil(t, cmd, "should return nil cmd on guard failure")
	})
}

// --- reassignedIncidentsMsg ---

func TestReassignedIncidentsMsg_SetsStatusAndRefreshes(t *testing.T) {
	t.Run("logs IDs, sets status, and returns updateIncidentListMsg", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}

		msg := reassignedIncidentsMsg([]pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q100"}},
			{APIObject: pagerduty.APIObject{ID: "Q200"}},
		})

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.status, "reassigned incidents")
		assert.Contains(t, updated.status, "Q100")
		assert.Contains(t, updated.status, "Q200")
		assert.NotNil(t, cmd, "should return updateIncidentListMsg cmd")
	})
}

// --- reEscalateIncidentsMsg ---

func TestReEscalateIncidentsMsg_Success(t *testing.T) {
	t.Run("returns sequence with reEscalate and clear commands", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		incidents := []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q123"}},
		}
		policy := &pagerduty.EscalationPolicy{
			APIObject: pagerduty.APIObject{ID: "POL1"},
		}

		result, cmd := m.Update(reEscalateIncidentsMsg{
			incidents: incidents,
			policy:    policy,
			level:     2,
		})
		_ = result.(model)

		assert.NotNil(t, cmd, "should return sequence command")
	})
}

func TestReEscalateIncidentsMsg_NilIncidents(t *testing.T) {
	t.Run("sets error status when incidents are nil", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		result, cmd := m.Update(reEscalateIncidentsMsg{incidents: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "failed re-escalating")
		assert.Nil(t, cmd, "should return nil cmd on guard failure")
	})
}

// --- reEscalatedIncidentsMsg ---

func TestReEscalatedIncidentsMsg_Success(t *testing.T) {
	t.Run("clears apiInProgress and returns flash and refresh", func(t *testing.T) {
		m := createTestModel()
		m.apiInProgress = true
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q789", HTMLURL: "https://example.com/incident/Q789"},
			Title:     "Test Alert",
		}

		msg := reEscalatedIncidentsMsg([]pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q789"}},
		})

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.False(t, updated.apiInProgress, "apiInProgress should be false")
		assert.NotNil(t, cmd, "should return batch with flash and refresh")
	})
}

func TestReEscalatedIncidentsMsg_NoSelectedIncident(t *testing.T) {
	t.Run("handles nil selectedIncident gracefully in log closures", func(t *testing.T) {
		m := createTestModel()
		m.apiInProgress = true
		m.config = &pd.Config{
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}
		m.selectedIncident = nil

		msg := reEscalatedIncidentsMsg([]pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "Q789"}},
		})

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.False(t, updated.apiInProgress, "apiInProgress should be false")
		assert.NotNil(t, cmd, "should still return commands")
	})
}
