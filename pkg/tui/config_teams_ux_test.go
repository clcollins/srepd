package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
)

// The team step should greet the validated user by name so the wizard feels
// responsive ("the token worked, and it's really me").
func TestTeamGreeting(t *testing.T) {
	assert.Equal(t,
		"Hi Chris — found 2 teams on your PagerDuty profile. Select the team(s) whose incidents you want to monitor; most users need exactly one.",
		teamGreeting("Chris", 2))
	assert.Equal(t,
		"Hi Chris — found 1 team on your PagerDuty profile. Select the team(s) whose incidents you want to monitor; most users need exactly one.",
		teamGreeting("Chris", 1))
	// No user name (token not validated yet / lookup failed): fall back to
	// neutral guidance.
	assert.Equal(t,
		"Select the team(s) whose incidents you want to monitor; most users need exactly one.",
		teamGreeting("", 0))
}

// fetchTeamOptions must surface the validated user's name for the greeting.
func TestFetchTeamOptions_ReturnsUserName(t *testing.T) {
	opts, teams, userName := fetchTeamOptions(mockFactoryOK(), "u+goodtoken", "", nil)
	assert.Len(t, opts, 2)
	assert.Len(t, teams, 2)
	assert.Equal(t, "Mock User", userName)
}

func team(id string) pagerduty.Team {
	return pagerduty.Team{APIObject: pagerduty.APIObject{ID: id}}
}

// teamPreselection decides which options render preselected:
// existing-config teams always win; a brand-new user on exactly one team has
// it preselected (the common SRE case answers itself); multiple teams with
// no existing config preselect nothing — a deliberate choice, guided by the
// "most users need exactly one" copy.
func TestTeamPreselection(t *testing.T) {
	existing := map[string]bool{"TEAM_002": true}

	sel := teamPreselection(existing, []pagerduty.Team{team("TEAM_001"), team("TEAM_002")})
	assert.False(t, sel["TEAM_001"])
	assert.True(t, sel["TEAM_002"], "existing config selection wins")

	sel = teamPreselection(map[string]bool{}, []pagerduty.Team{team("TEAM_001")})
	assert.True(t, sel["TEAM_001"], "a single team must be preselected for new users")

	sel = teamPreselection(nil, []pagerduty.Team{team("TEAM_001")})
	assert.True(t, sel["TEAM_001"], "nil existing set behaves like empty")

	sel = teamPreselection(map[string]bool{}, []pagerduty.Team{team("TEAM_001"), team("TEAM_002")})
	assert.False(t, sel["TEAM_001"])
	assert.False(t, sel["TEAM_002"])
}
