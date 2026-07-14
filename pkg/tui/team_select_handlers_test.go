package tui

import (
	"errors"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fetchedTeamsMsg handler tests ---

func TestFetchedTeamsMsg_Error(t *testing.T) {
	t.Run("sets status with error message", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(fetchedTeamsMsg{
			teams: nil,
			err:   errors.New("API rate limit exceeded"),
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "could not fetch teams")
		assert.Contains(t, updated.status, "API rate limit exceeded")
		assert.False(t, updated.teamSelectMode, "should not enter team select mode on error")
		assert.Nil(t, cmd, "should return nil cmd on error")
	})
}

func TestFetchedTeamsMsg_EmptyTeams(t *testing.T) {
	t.Run("sets status when no teams found", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(fetchedTeamsMsg{
			teams: []pagerduty.Team{},
			err:   nil,
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "no teams found")
		assert.False(t, updated.teamSelectMode, "should not enter team select mode with empty teams")
		assert.Nil(t, cmd, "should return nil cmd when no teams")
	})
}

func TestFetchedTeamsMsg_HappyPath(t *testing.T) {
	t.Run("builds team select form and sets teamSelectMode", func(t *testing.T) {
		m := createTestModel()
		// Need layout height for form
		windowSize = tea.WindowSizeMsg{Width: 120, Height: 60}

		teams := []pagerduty.Team{
			{APIObject: pagerduty.APIObject{ID: "TEAM1"}, Name: "Team Alpha"},
			{APIObject: pagerduty.APIObject{ID: "TEAM2"}, Name: "Team Bravo"},
			{APIObject: pagerduty.APIObject{ID: "TEAM3"}, Name: "Team Charlie"},
		}

		result, cmd := m.Update(fetchedTeamsMsg{teams: teams, err: nil})
		updated := result.(model)

		assert.True(t, updated.teamSelectMode, "should enter team select mode")
		assert.NotNil(t, updated.teamSelectForm, "should create team select form")
		assert.NotNil(t, cmd, "should return form Init cmd")

		// Verify team names map is populated
		assert.Equal(t, "Team Alpha", updated.teamSelectNames["TEAM1"])
		assert.Equal(t, "Team Bravo", updated.teamSelectNames["TEAM2"])
		assert.Equal(t, "Team Charlie", updated.teamSelectNames["TEAM3"])

		// Verify teamSelectIDs is reset (will be filled by form)
		assert.Empty(t, updated.teamSelectIDs, "teamSelectIDs should be empty before form submission")
	})
}

func TestFetchedTeamsMsg_SingleTeam(t *testing.T) {
	t.Run("works with a single team", func(t *testing.T) {
		m := createTestModel()
		windowSize = tea.WindowSizeMsg{Width: 120, Height: 60}

		teams := []pagerduty.Team{
			{APIObject: pagerduty.APIObject{ID: "TEAM1"}, Name: "Solo Team"},
		}

		result, cmd := m.Update(fetchedTeamsMsg{teams: teams, err: nil})
		updated := result.(model)

		assert.True(t, updated.teamSelectMode)
		assert.NotNil(t, updated.teamSelectForm)
		assert.NotNil(t, cmd)
		assert.Len(t, updated.teamSelectNames, 1)
	})
}

// --- teamsSelectedMsg handler tests ---

func TestTeamsSelectedMsg_EmptyIDs(t *testing.T) {
	t.Run("sets status when no teams selected", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(teamsSelectedMsg{
			ids:   []string{},
			names: map[string]string{},
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "no teams selected")
		assert.Nil(t, cmd, "should return nil cmd when no teams selected")
	})
}

func TestTeamsSelectedMsg_NilIDs(t *testing.T) {
	t.Run("sets status when ids is nil", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(teamsSelectedMsg{
			ids:   nil,
			names: nil,
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "no teams selected")
		assert.Nil(t, cmd, "should return nil cmd when ids is nil")
	})
}

func TestTeamsSelectedMsg_HappyPath(t *testing.T) {
	t.Run("sets status with count and returns writeTeamsToConfigCmd", func(t *testing.T) {
		m := createTestModel()

		ids := []string{"TEAM1", "TEAM2"}
		names := map[string]string{
			"TEAM1": "Team Alpha",
			"TEAM2": "Team Bravo",
		}

		result, cmd := m.Update(teamsSelectedMsg{ids: ids, names: names})
		updated := result.(model)

		assert.Contains(t, updated.status, "selected 2 team(s)")
		assert.Contains(t, updated.status, "updating config")
		assert.NotNil(t, cmd, "should return writeTeamsToConfigCmd")
	})
}

func TestTeamsSelectedMsg_SingleTeam(t *testing.T) {
	t.Run("sets correct count for single team", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(teamsSelectedMsg{
			ids:   []string{"TEAM1"},
			names: map[string]string{"TEAM1": "Team Alpha"},
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "selected 1 team(s)")
		assert.NotNil(t, cmd)
	})
}

// --- teamsConfigUpdatedMsg handler tests ---

func TestTeamsConfigUpdatedMsg_Error(t *testing.T) {
	t.Run("sets status with error on write failure", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(teamsConfigUpdatedMsg{
			err: errors.New("permission denied"),
		})
		updated := result.(model)

		assert.Contains(t, updated.status, "config write failed")
		assert.Contains(t, updated.status, "permission denied")
		// Always returns a cmd that emits updateIncidentListMsg
		require.NotNil(t, cmd, "should always return a cmd")

		msg := cmd()
		_, ok := msg.(updateIncidentListMsg)
		assert.True(t, ok, "cmd should emit updateIncidentListMsg")
	})
}

func TestTeamsConfigUpdatedMsg_HappyPath(t *testing.T) {
	t.Run("sets status to teams saved and returns updateIncidentListMsg cmd", func(t *testing.T) {
		m := createTestModel()

		result, cmd := m.Update(teamsConfigUpdatedMsg{err: nil})
		updated := result.(model)

		assert.Contains(t, updated.status, "teams saved to config")
		require.NotNil(t, cmd, "should return cmd that emits updateIncidentListMsg")

		msg := cmd()
		_, ok := msg.(updateIncidentListMsg)
		assert.True(t, ok, "cmd should emit updateIncidentListMsg")
	})
}
