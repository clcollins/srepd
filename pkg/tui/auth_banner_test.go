package tui

import (
	"fmt"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/stretchr/testify/assert"
)

func TestAuthBannerTickMsg_CyclesPhase(t *testing.T) {
	tests := []struct {
		name          string
		startPhase    int
		expectedPhase int
	}{
		{"0 to 1", 0, 1},
		{"1 to 2", 1, 2},
		{"4 to 5", 4, 5},
		{"5 wraps to 0", 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createTestModel()
			m.ocmAuthPending = true
			m.authBannerPhase = tt.startPhase

			result, cmd := m.Update(authBannerTickMsg{})
			updated := result.(model)

			assert.Equal(t, tt.expectedPhase, updated.authBannerPhase)
			assert.NotNil(t, cmd, "should schedule next tick")
		})
	}
}

func TestAuthBannerTickMsg_StopsWhenAuthDone(t *testing.T) {
	t.Run("stops tick loop and resets phase when auth is complete", func(t *testing.T) {
		m := createTestModel()
		m.ocmAuthPending = false
		m.authBannerPhase = 3

		result, cmd := m.Update(authBannerTickMsg{})
		updated := result.(model)

		assert.Equal(t, 0, updated.authBannerPhase, "phase should reset to 0")
		assert.Nil(t, cmd, "should not schedule next tick")
	})
}

func TestOCMClientReadyMsg_ClearsBannerPhase(t *testing.T) {
	t.Run("clears authBannerPhase on successful auth", func(t *testing.T) {
		m := createTestModel()
		m.ocmAuthPending = true
		m.authBannerPhase = 4
		m.incidentClusterMap = make(map[string][]string)
		m.clusterEnrichInFlight = make(map[string]bool)
		m.clusterEnrichFailed = make(map[string]int)
		m.clusterCache = make(map[string]*ocm.ClusterInfo)
		mock := createMockOCMClient()

		result, _ := m.Update(OCMClientReadyMsg{Client: mock, Err: nil})
		updated := result.(model)

		assert.Equal(t, 0, updated.authBannerPhase)
		assert.False(t, updated.ocmAuthPending)
	})

	t.Run("clears authBannerPhase on error", func(t *testing.T) {
		m := createTestModel()
		m.ocmAuthPending = true
		m.authBannerPhase = 3

		result, _ := m.Update(OCMClientReadyMsg{Err: fmt.Errorf("auth failed")})
		updated := result.(model)

		assert.Equal(t, 0, updated.authBannerPhase)
		assert.False(t, updated.ocmAuthPending)
	})
}

func TestRenderBottomStatus_AuthBanner(t *testing.T) {
	t.Run("shows auth banner when ocmAuthPending is true", func(t *testing.T) {
		m := sizedAuthTestModel(t)
		m.ocmAuthPending = true
		m.authBannerPhase = 0

		output := m.renderBottomStatus()

		assert.Contains(t, output, "Please complete OCM browser auth")
	})
}

func TestRenderBottomStatus_NoAuthBanner(t *testing.T) {
	t.Run("does not show auth banner when ocmAuthPending is false", func(t *testing.T) {
		m := sizedAuthTestModel(t)
		m.ocmAuthPending = false

		output := m.renderBottomStatus()

		assert.NotContains(t, output, "Please complete OCM browser auth")
	})
}

func TestLoginMsg_BlockedDuringAuth(t *testing.T) {
	t.Run("loginMsg blocked while OCM auth is pending", func(t *testing.T) {
		m := createTestModel()
		m.ocmAuthPending = true
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P123"},
		}

		result, cmd := m.Update(loginMsg("login"))
		updated := result.(model)

		assert.Contains(t, updated.status, "Login blocked")
		assert.NotNil(t, cmd, "should return flash notification command")
	})
}

func TestRosaBoundaryLoginMsg_BlockedDuringAuth(t *testing.T) {
	t.Run("rosaBoundaryLoginMsg blocked while OCM auth is pending", func(t *testing.T) {
		m := createTestModel()
		m.ocmAuthPending = true
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P123"},
		}

		result, cmd := m.Update(rosaBoundaryLoginMsg("login"))
		updated := result.(model)

		assert.Contains(t, updated.status, "Login blocked")
		assert.NotNil(t, cmd, "should return flash notification command")
	})
}

func TestTableMode_LoginKeyBlockedDuringAuth(t *testing.T) {
	t.Run("login key in table mode blocked while OCM auth is pending", func(t *testing.T) {
		m := createTestModelWithSelectedIncident()
		m.ocmAuthPending = true

		result, cmd := m.Update(loginKeyMsg())
		updated := result.(model)

		assert.Contains(t, updated.status, "Login blocked")
		assert.NotNil(t, cmd, "should return flash notification command")
	})
}

func TestIncidentViewMode_LoginKeyBlockedDuringAuth(t *testing.T) {
	t.Run("login key in incident view blocked while OCM auth is pending", func(t *testing.T) {
		m := createTestModel()
		m.ocmAuthPending = true
		m.viewingIncident = true
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P123"},
		}
		m.incidentAlertsLoaded = true

		result, cmd := m.Update(loginKeyMsg())
		updated := result.(model)

		assert.Contains(t, updated.status, "Login blocked")
		assert.NotNil(t, cmd, "should return flash notification command")
	})
}

func TestChordRosaBoundaryLogin_BlockedDuringAuth(t *testing.T) {
	t.Run("rosa-boundary chord blocked while OCM auth is pending", func(t *testing.T) {
		m := createTestModel()
		m.ocmAuthPending = true
		m.rosaBoundaryLauncher = launcher.ClusterLauncher{Enabled: true}
		m.viewingIncident = true
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "P123"},
		}
		m.incidentAlertsLoaded = true

		result, cmd := chordRosaBoundaryLogin(m)
		updated := result.(model)

		assert.Contains(t, updated.status, "Login blocked")
		assert.NotNil(t, cmd, "should return flash notification command")
	})
}

func sizedAuthTestModel(t *testing.T) model {
	t.Helper()
	m := createTestModel()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = result.(model)
	return m
}
