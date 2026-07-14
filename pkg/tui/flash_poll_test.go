package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- clearFlashMsg handler tests ---
// Note: basic matching/non-matching tests exist in model_test.go under
// TestFlashNotification. These cover additional edge cases.

func TestClearFlashMsg_ExactMatch(t *testing.T) {
	t.Run("clears status when message matches exactly", func(t *testing.T) {
		m := createTestModel()
		m.status = "Acknowledged Q123"

		result, cmd := m.Update(clearFlashMsg{message: "Acknowledged Q123"})
		updated := result.(model)

		assert.Equal(t, "", updated.status, "status should be cleared when flash message matches")
		assert.Nil(t, cmd, "no further command expected")
	})
}

func TestClearFlashMsg_DifferentMessage(t *testing.T) {
	t.Run("does not clear status when message differs", func(t *testing.T) {
		m := createTestModel()
		m.status = "showing 3/5 incidents..."

		result, cmd := m.Update(clearFlashMsg{message: "Acknowledged Q123"})
		updated := result.(model)

		assert.Equal(t, "showing 3/5 incidents...", updated.status,
			"status should NOT be cleared when flash message differs from current status")
		assert.Nil(t, cmd)
	})
}

func TestClearFlashMsg_EmptyStatus(t *testing.T) {
	t.Run("no-op when status is already empty", func(t *testing.T) {
		m := createTestModel()
		m.status = ""

		result, cmd := m.Update(clearFlashMsg{message: "some old message"})
		updated := result.(model)

		assert.Equal(t, "", updated.status, "status should remain empty")
		assert.Nil(t, cmd)
	})
}

func TestClearFlashMsg_EmptyMessage(t *testing.T) {
	t.Run("clears empty status when message is empty", func(t *testing.T) {
		m := createTestModel()
		m.status = ""

		result, cmd := m.Update(clearFlashMsg{message: ""})
		updated := result.(model)

		assert.Equal(t, "", updated.status)
		assert.Nil(t, cmd)
	})
}

// --- PollIncidentsMsg handler tests ---

func TestPollIncidentsMsg_AutoRefreshDisabled(t *testing.T) {
	t.Run("no-op when autoRefresh is false", func(t *testing.T) {
		m := createTestModel()
		m.autoRefresh = false
		m.apiInProgress = false

		result, cmd := m.Update(PollIncidentsMsg{})
		updated := result.(model)

		assert.False(t, updated.apiInProgress, "apiInProgress should remain false")
		assert.Nil(t, cmd, "should return nil cmd when autoRefresh is disabled")
	})
}

func TestPollIncidentsMsg_AutoRefreshEnabled(t *testing.T) {
	t.Run("sets apiInProgress and returns batch when autoRefresh is true", func(t *testing.T) {
		m := createTestModel()
		m.autoRefresh = true
		m.apiInProgress = false
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
			Teams: []*pagerduty.Team{
				{APIObject: pagerduty.APIObject{ID: "T1"}, Name: "Test Team"},
			},
		}

		result, cmd := m.Update(PollIncidentsMsg{})
		updated := result.(model)

		assert.True(t, updated.apiInProgress, "apiInProgress should be set to true")
		assert.NotNil(t, cmd, "should return batch with spinner tick and updateIncidentList")
	})
}

func TestPollIncidentsMsg_AlreadyInProgress(t *testing.T) {
	t.Run("still dispatches when apiInProgress was already true", func(t *testing.T) {
		m := createTestModel()
		m.autoRefresh = true
		m.apiInProgress = true
		m.config = &pd.Config{
			Client: &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{
				APIObject: pagerduty.APIObject{ID: "U1"},
			},
		}

		result, cmd := m.Update(PollIncidentsMsg{})
		updated := result.(model)

		// The handler does not guard on apiInProgress, it unconditionally sets it
		assert.True(t, updated.apiInProgress)
		assert.NotNil(t, cmd)
	})
}

// --- getIncidentMsg handler tests ---

func TestGetIncidentMsg_EmptyString(t *testing.T) {
	t.Run("returns setStatusMsg with no incident selected", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(getIncidentMsg(""))
		updated := result.(model)

		// The handler returns a cmd that produces setStatusMsg
		require.NotNil(t, cmd, "should return a cmd")
		msg := cmd()
		statusMsg, ok := msg.(setStatusMsg)
		assert.True(t, ok, "cmd should produce a setStatusMsg")
		assert.Equal(t, "no incident selected", statusMsg.string)

		// apiInProgress should not be set for empty string
		assert.False(t, updated.apiInProgress)
	})
}

func TestGetIncidentMsg_HappyPath(t *testing.T) {
	t.Run("sets status and apiInProgress, returns batch of 4 commands", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}

		result, cmd := m.Update(getIncidentMsg("P1234567"))
		updated := result.(model)

		assert.True(t, updated.apiInProgress, "apiInProgress should be set to true")
		assert.Contains(t, updated.status, "getting details for incident P1234567")
		assert.NotNil(t, cmd, "should return batch of commands (spinner + get + alerts + notes)")
	})
}

func TestGetIncidentMsg_SetsStatusWithID(t *testing.T) {
	t.Run("status message includes the incident ID", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U1"}},
		}

		result, _ := m.Update(getIncidentMsg("QABC123"))
		updated := result.(model)

		assert.Contains(t, updated.status, "QABC123",
			"status should include the incident ID being fetched")
	})
}
