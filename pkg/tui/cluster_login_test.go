package tui

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// clusterSelectedMsg
// ---------------------------------------------------------------------------

func TestClusterSelectedMsg_NilSelectedIncident(t *testing.T) {
	t.Run("guard: returns status when selectedIncident is nil", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U123"}},
		}
		m.selectedIncident = nil

		result, cmd := m.Update(clusterSelectedMsg("cluster-abc"))
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned when selectedIncident is nil")
		assert.Contains(t, m.status, "unable to login - no selected incident")
	})
}

func TestClusterSelectedMsg_HappyPath(t *testing.T) {
	t.Run("sets status and returns login command", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		result, cmd := m.Update(clusterSelectedMsg("cluster-xyz"))
		m = result.(model)

		assert.NotNil(t, cmd, "should return a login command")
		assert.Contains(t, m.status, "logging into cluster cluster-xyz")
	})
}

func TestClusterSelectedMsg_LogsLoginInitiated(t *testing.T) {
	t.Run("logs login initiated at INFO level", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		output := captureLogOutput(log.InfoLevel, func() {
			result, _ := m.Update(clusterSelectedMsg("cluster-log"))
			m = result.(model)
		})

		assert.Contains(t, output, "login initiated",
			"should log login initiation at INFO level")
	})
}

// ---------------------------------------------------------------------------
// rosaBoundaryClusterSelectedMsg
// ---------------------------------------------------------------------------

func TestRosaBoundaryClusterSelectedMsg_NilSelectedIncident(t *testing.T) {
	t.Run("guard: returns status when selectedIncident is nil", func(t *testing.T) {
		m := createTestModel()
		m.config = &pd.Config{
			Client:      &pd.MockPagerDutyClient{},
			CurrentUser: &pagerduty.User{APIObject: pagerduty.APIObject{ID: "U123"}},
		}
		m.selectedIncident = nil

		result, cmd := m.Update(rosaBoundaryClusterSelectedMsg("cluster-abc"))
		m = result.(model)

		assert.Nil(t, cmd, "no command should be returned when selectedIncident is nil")
		assert.Contains(t, m.status, "unable to login via rosa-boundary - no selected incident")
	})
}

func TestRosaBoundaryClusterSelectedMsg_HappyPath(t *testing.T) {
	t.Run("sets status and returns login command", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		result, cmd := m.Update(rosaBoundaryClusterSelectedMsg("cluster-rb1"))
		m = result.(model)

		assert.NotNil(t, cmd, "should return a login command")
		assert.Contains(t, m.status, "rosa-boundary login to cluster cluster-rb1")
	})
}

func TestRosaBoundaryClusterSelectedMsg_LogsLoginInitiated(t *testing.T) {
	t.Run("logs rosa-boundary login initiated at INFO level", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.config.Client = &pd.MockPagerDutyClient{}

		output := captureLogOutput(log.InfoLevel, func() {
			result, _ := m.Update(rosaBoundaryClusterSelectedMsg("cluster-rb2"))
			m = result.(model)
		})

		assert.Contains(t, output, "rosa-boundary login initiated",
			"should log rosa-boundary login initiation at INFO level")
	})
}

// ---------------------------------------------------------------------------
// loginFinishedMsg
// ---------------------------------------------------------------------------

func TestLoginFinishedMsg_Error(t *testing.T) {
	t.Run("error: sets status and returns errMsg", func(t *testing.T) {
		m := createTestModel()

		loginErr := errors.New("ssh connection refused")
		result, cmd := m.Update(loginFinishedMsg{err: loginErr})
		m = result.(model)

		assert.Contains(t, m.status, "failed to login")
		assert.Contains(t, m.status, "ssh connection refused")
		assert.NotNil(t, cmd, "should return a command wrapping errMsg")

		// Execute the returned command to verify it produces an errMsg
		msg := cmd()
		em, ok := msg.(errMsg)
		assert.True(t, ok, "returned command should produce errMsg")
		assert.Equal(t, loginErr, em.error, "errMsg should contain the original error")
	})
}

func TestLoginFinishedMsg_HappyWithSelectedIncident(t *testing.T) {
	t.Run("success with selectedIncident: logs with incident_id", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P999"},
		}

		output := captureLogOutput(log.InfoLevel, func() {
			result, cmd := m.Update(loginFinishedMsg{err: nil})
			m = result.(model)
			assert.Nil(t, cmd, "no command should be returned on successful login")
		})

		assert.Contains(t, output, "login completed",
			"should log login completed at INFO level")
		assert.Contains(t, output, "P999",
			"should include incident_id in the log")
	})
}

func TestLoginFinishedMsg_HappyWithoutSelectedIncident(t *testing.T) {
	t.Run("success without selectedIncident: logs without incident_id", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = nil

		output := captureLogOutput(log.InfoLevel, func() {
			result, cmd := m.Update(loginFinishedMsg{err: nil})
			m = result.(model)
			assert.Nil(t, cmd, "no command should be returned on successful login")
		})

		assert.Contains(t, output, "login completed",
			"should log login completed at INFO level")
	})
}

// ---------------------------------------------------------------------------
// loginFinishedMsg table-driven summary
// ---------------------------------------------------------------------------

func TestLoginFinishedMsg_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		selectedIncident *pagerduty.Incident
		expectCmd        bool
		expectStatusErr  bool
	}{
		{
			name:             "error returns errMsg command",
			err:              errors.New("timeout"),
			selectedIncident: nil,
			expectCmd:        true,
			expectStatusErr:  true,
		},
		{
			name: "success with incident",
			err:  nil,
			selectedIncident: &pagerduty.Incident{
				APIObject: pagerduty.APIObject{ID: "PINC"},
			},
			expectCmd:       false,
			expectStatusErr: false,
		},
		{
			name:             "success without incident",
			err:              nil,
			selectedIncident: nil,
			expectCmd:        false,
			expectStatusErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.selectedIncident = tt.selectedIncident

			result, cmd := m.Update(loginFinishedMsg{err: tt.err})
			m = result.(model)

			if tt.expectCmd {
				assert.NotNil(t, cmd, "expected a command")
			} else {
				assert.Nil(t, cmd, "expected no command")
			}

			if tt.expectStatusErr {
				assert.Contains(t, m.status, "failed to login")
			}
		})
	}
}
