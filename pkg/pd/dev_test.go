package pd

import (
	"context"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFixturesDir = "../../testdata/fixtures"

func TestLoadFixtures(t *testing.T) {
	t.Run("loads all fixture files successfully", func(t *testing.T) {
		fixtures, err := LoadFixtures(testFixturesDir)
		require.NoError(t, err)

		assert.NotNil(t, fixtures.Incidents)
		assert.NotNil(t, fixtures.Alerts)
		assert.NotNil(t, fixtures.Notes)
		assert.NotNil(t, fixtures.Config)

		// Verify incident count matches fixture data (12 scenarios)
		assert.Equal(t, 20, len(fixtures.Incidents))

		// Verify config has user, teams, team members, and escalation policies
		assert.NotEmpty(t, fixtures.Config.User.ID)
		assert.NotEmpty(t, fixtures.Config.Teams)
		assert.NotEmpty(t, fixtures.Config.TeamMembers)
		assert.NotEmpty(t, fixtures.Config.EscalationPolicies)
	})

	t.Run("returns error for nonexistent directory", func(t *testing.T) {
		_, err := LoadFixtures("/nonexistent/path")
		assert.Error(t, err)
	})
}

func TestDevClient_ListIncidents(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns all fixture incidents", func(t *testing.T) {
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
		require.NoError(t, err)

		assert.Equal(t, 20, len(resp.Incidents))
		assert.False(t, resp.More)
	})

	t.Run("filters by triggered status", func(t *testing.T) {
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{
			Statuses: []string{"triggered"},
		})
		require.NoError(t, err)

		for _, inc := range resp.Incidents {
			assert.Equal(t, "triggered", inc.Status)
		}
	})

	t.Run("filters by acknowledged status", func(t *testing.T) {
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{
			Statuses: []string{"acknowledged"},
		})
		require.NoError(t, err)

		for _, inc := range resp.Incidents {
			assert.Equal(t, "acknowledged", inc.Status)
		}
		// Fixture PDEV_INC_010 is pre-acknowledged
		assert.GreaterOrEqual(t, len(resp.Incidents), 1)
	})

	t.Run("returns both triggered and acknowledged by default", func(t *testing.T) {
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{
			Statuses: []string{"triggered", "acknowledged"},
		})
		require.NoError(t, err)

		assert.Equal(t, 12, len(resp.Incidents))
	})
}

func TestDevClient_GetIncident(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns specific incident by ID", func(t *testing.T) {
		incident, err := client.GetIncidentWithContext(ctx, "PDEV_INC_001")
		require.NoError(t, err)

		assert.Equal(t, "PDEV_INC_001", incident.ID)
		assert.Equal(t, "ClusterOperatorDown CRITICAL (1)", incident.Title)
		assert.Equal(t, "triggered", incident.Status)
	})

	t.Run("returns error for nonexistent incident", func(t *testing.T) {
		_, err := client.GetIncidentWithContext(ctx, "NONEXISTENT")
		assert.Error(t, err)
	})
}

func TestDevClient_AcknowledgeUpdatesState(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("acknowledge changes status and adds acknowledgement", func(t *testing.T) {
		// Verify initial state is triggered
		incident, err := client.GetIncidentWithContext(ctx, "PDEV_INC_001")
		require.NoError(t, err)
		assert.Equal(t, "triggered", incident.Status)
		assert.Empty(t, incident.Acknowledgements)

		// Acknowledge the incident
		resp, err := client.ManageIncidentsWithContext(ctx, "dev@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     "PDEV_INC_001",
				Status: "acknowledged",
				Assignments: []pagerduty.Assignee{
					{
						Assignee: pagerduty.APIObject{
							ID:   "PDEV_USER_001",
							Type: "user_reference",
						},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Incidents)

		// Verify state changed
		updated, err := client.GetIncidentWithContext(ctx, "PDEV_INC_001")
		require.NoError(t, err)
		assert.Equal(t, "acknowledged", updated.Status)
		assert.NotEmpty(t, updated.Acknowledgements)
		assert.Equal(t, "PDEV_USER_001", updated.Acknowledgements[0].Acknowledger.ID)
	})
}

func TestDevClient_AddNoteAppendsToList(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("adds note to incident", func(t *testing.T) {
		// Check initial notes for an incident with no notes
		notes, err := client.ListIncidentNotesWithContext(ctx, "PDEV_INC_001")
		require.NoError(t, err)
		initialCount := len(notes)

		// Add a note
		note, err := client.CreateIncidentNoteWithContext(ctx, "PDEV_INC_001", pagerduty.IncidentNote{
			Content: "Test note from dev client",
			User: pagerduty.APIObject{
				ID:   "PDEV_USER_001",
				Type: "user_reference",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Test note from dev client", note.Content)
		assert.NotEmpty(t, note.ID)

		// Verify note was appended
		updatedNotes, err := client.ListIncidentNotesWithContext(ctx, "PDEV_INC_001")
		require.NoError(t, err)
		assert.Equal(t, initialCount+1, len(updatedNotes))
	})
}

func TestDevClient_ListIncidentsAfterAck(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("acknowledged incident persists in list", func(t *testing.T) {
		// Acknowledge incident
		_, err := client.ManageIncidentsWithContext(ctx, "dev@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     "PDEV_INC_003",
				Status: "acknowledged",
				Assignments: []pagerduty.Assignee{
					{
						Assignee: pagerduty.APIObject{
							ID:   "PDEV_USER_001",
							Type: "user_reference",
						},
					},
				},
			},
		})
		require.NoError(t, err)

		// List incidents with both statuses
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{
			Statuses: []string{"triggered", "acknowledged"},
		})
		require.NoError(t, err)

		// Find our acknowledged incident
		found := false
		for _, inc := range resp.Incidents {
			if inc.ID == "PDEV_INC_003" {
				found = true
				assert.Equal(t, "acknowledged", inc.Status)
			}
		}
		assert.True(t, found, "acknowledged incident should still appear in list")
	})
}

func TestDevClient_SilenceUpdatesPolicy(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("re-escalate changes escalation policy", func(t *testing.T) {
		// Verify initial policy
		incident, err := client.GetIncidentWithContext(ctx, "PDEV_INC_005")
		require.NoError(t, err)
		initialPolicyID := incident.EscalationPolicy.ID

		// Re-escalate to silent policy
		_, err = client.ManageIncidentsWithContext(ctx, "dev@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:               "PDEV_INC_005",
				EscalationPolicy: &pagerduty.APIReference{ID: "PDEV_POLICY_SILENT", Type: "escalation_policy"},
				EscalationLevel:  1,
			},
		})
		require.NoError(t, err)

		// Verify policy changed
		updated, err := client.GetIncidentWithContext(ctx, "PDEV_INC_005")
		require.NoError(t, err)
		assert.NotEqual(t, initialPolicyID, updated.EscalationPolicy.ID)
		assert.Equal(t, "PDEV_POLICY_SILENT", updated.EscalationPolicy.ID)
	})
}

func TestDevClient_ListIncidentAlerts(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns alerts for incident with single alert", func(t *testing.T) {
		resp, err := client.ListIncidentAlertsWithContext(ctx, "PDEV_INC_001", pagerduty.ListIncidentAlertsOptions{})
		require.NoError(t, err)
		assert.Equal(t, 1, len(resp.Alerts))
		assert.Equal(t, "PDEV_ALERT_001", resp.Alerts[0].ID)
	})

	t.Run("returns alerts for incident with multiple alerts", func(t *testing.T) {
		resp, err := client.ListIncidentAlertsWithContext(ctx, "PDEV_INC_002", pagerduty.ListIncidentAlertsOptions{})
		require.NoError(t, err)
		assert.Equal(t, 3, len(resp.Alerts))
	})

	t.Run("returns empty alerts for CEE escalation", func(t *testing.T) {
		resp, err := client.ListIncidentAlertsWithContext(ctx, "PDEV_INC_007", pagerduty.ListIncidentAlertsOptions{})
		require.NoError(t, err)
		assert.Equal(t, 0, len(resp.Alerts))
	})

	t.Run("returns 25 alerts for large payload incident", func(t *testing.T) {
		resp, err := client.ListIncidentAlertsWithContext(ctx, "PDEV_INC_008", pagerduty.ListIncidentAlertsOptions{})
		require.NoError(t, err)
		assert.Equal(t, 25, len(resp.Alerts))
	})
}

func TestDevClient_ListIncidentNotes(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns notes for incident with notes", func(t *testing.T) {
		notes, err := client.ListIncidentNotesWithContext(ctx, "PDEV_INC_012")
		require.NoError(t, err)
		assert.Equal(t, 3, len(notes))
	})

	t.Run("returns empty notes for incident without notes", func(t *testing.T) {
		notes, err := client.ListIncidentNotesWithContext(ctx, "PDEV_INC_002")
		require.NoError(t, err)
		assert.Equal(t, 0, len(notes))
	})
}

func TestDevClient_GetCurrentUser(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns fixture user", func(t *testing.T) {
		user, err := client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})
		require.NoError(t, err)
		assert.Equal(t, "PDEV_USER_001", user.ID)
		assert.Equal(t, "dev@example.com", user.Email)
		assert.Equal(t, "Dev User", user.Name)
	})
}

func TestDevClient_GetTeam(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns fixture team", func(t *testing.T) {
		team, err := client.GetTeamWithContext(ctx, "PDEV_TEAM_001")
		require.NoError(t, err)
		assert.Equal(t, "PDEV_TEAM_001", team.ID)
		assert.Equal(t, "Dev Platform SRE", team.Name)
	})
}

func TestDevClient_GetEscalationPolicy(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns fixture escalation policy", func(t *testing.T) {
		policy, err := client.GetEscalationPolicyWithContext(ctx, "PDEV_POLICY_DEFAULT", &pagerduty.GetEscalationPolicyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "PDEV_POLICY_DEFAULT", policy.ID)
		assert.Equal(t, "SREP Default Escalation", policy.Name)
	})

	t.Run("returns error for unknown policy", func(t *testing.T) {
		_, err := client.GetEscalationPolicyWithContext(ctx, "NONEXISTENT", &pagerduty.GetEscalationPolicyOptions{})
		assert.Error(t, err)
	})
}

func TestDevClient_ListMembers(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns team members", func(t *testing.T) {
		resp, err := client.ListMembersWithContext(ctx, "PDEV_TEAM_001", pagerduty.ListTeamMembersOptions{})
		require.NoError(t, err)
		assert.Equal(t, 3, len(resp.Members))
	})
}

func TestDevClient_GetUser(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns user by ID", func(t *testing.T) {
		user, err := client.GetUserWithContext(ctx, "PDEV_USER_002", pagerduty.GetUserOptions{})
		require.NoError(t, err)
		assert.Equal(t, "PDEV_USER_002", user.ID)
		assert.Equal(t, "Alice Engineer", user.Name)
	})

	t.Run("returns error for unknown user", func(t *testing.T) {
		_, err := client.GetUserWithContext(ctx, "NONEXISTENT", pagerduty.GetUserOptions{})
		assert.Error(t, err)
	})
}

func TestDevClient_ListOnCalls(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns on-call entries", func(t *testing.T) {
		resp, err := client.ListOnCallsWithContext(ctx, pagerduty.ListOnCallOptions{})
		require.NoError(t, err)
		assert.NotNil(t, resp)
		// Dev client returns a current on-call entry for the fixture user
		assert.GreaterOrEqual(t, len(resp.OnCalls), 1)
	})
}

func TestDevClient_ListIncidents_StableOrder(t *testing.T) {
	client := newTestDevClient(t)
	ctx := context.Background()

	t.Run("returns incidents sorted by CreatedAt descending", func(t *testing.T) {
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
		require.NoError(t, err)
		require.True(t, len(resp.Incidents) > 1, "need at least 2 incidents to verify order")

		// Verify descending CreatedAt order (newest first, matching PD API)
		for i := 1; i < len(resp.Incidents); i++ {
			assert.True(t, resp.Incidents[i-1].CreatedAt >= resp.Incidents[i].CreatedAt,
				"incident %d (%s) should be >= incident %d (%s) by CreatedAt",
				i-1, resp.Incidents[i-1].CreatedAt, i, resp.Incidents[i].CreatedAt)
		}
	})

	t.Run("order is stable across repeated calls", func(t *testing.T) {
		var firstOrder []string
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
		require.NoError(t, err)
		for _, inc := range resp.Incidents {
			firstOrder = append(firstOrder, inc.ID)
		}

		// Call multiple times and verify the order never changes
		for attempt := 0; attempt < 20; attempt++ {
			resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
			require.NoError(t, err)
			for i, inc := range resp.Incidents {
				assert.Equal(t, firstOrder[i], inc.ID, "order changed on attempt %d at index %d", attempt, i)
			}
		}
	})

	t.Run("order is preserved after acknowledging an incident", func(t *testing.T) {
		// Capture order before mutation
		resp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
		require.NoError(t, err)
		var orderBefore []string
		for _, inc := range resp.Incidents {
			orderBefore = append(orderBefore, inc.ID)
		}

		// Acknowledge an incident in the middle of the list
		_, err = client.ManageIncidentsWithContext(ctx, "dev@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     "PDEV_INC_005",
				Status: "acknowledged",
				Assignments: []pagerduty.Assignee{
					{
						Assignee: pagerduty.APIObject{
							ID:   "PDEV_USER_001",
							Type: "user_reference",
						},
					},
				},
			},
		})
		require.NoError(t, err)

		// Verify order is unchanged
		resp, err = client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
		require.NoError(t, err)
		for i, inc := range resp.Incidents {
			assert.Equal(t, orderBefore[i], inc.ID, "order changed after ack at index %d", i)
		}
	})

	t.Run("filtered results preserve relative order", func(t *testing.T) {
		// First acknowledge a specific incident so we have both statuses
		_, err := client.ManageIncidentsWithContext(ctx, "dev@example.com", []pagerduty.ManageIncidentsOptions{
			{
				ID:     "PDEV_INC_002",
				Status: "acknowledged",
				Assignments: []pagerduty.Assignee{
					{
						Assignee: pagerduty.APIObject{
							ID:   "PDEV_USER_001",
							Type: "user_reference",
						},
					},
				},
			},
		})
		require.NoError(t, err)

		// Get full list to know the expected relative order
		fullResp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{})
		require.NoError(t, err)

		// Get triggered-only list
		triggeredResp, err := client.ListIncidentsWithContext(ctx, pagerduty.ListIncidentsOptions{
			Statuses: []string{"triggered"},
		})
		require.NoError(t, err)

		// Verify triggered incidents appear in the same relative order as the full list
		triggeredIdx := 0
		for _, inc := range fullResp.Incidents {
			if triggeredIdx >= len(triggeredResp.Incidents) {
				break
			}
			if inc.Status == "triggered" {
				assert.Equal(t, triggeredResp.Incidents[triggeredIdx].ID, inc.ID,
					"triggered incident at filtered index %d has wrong relative order", triggeredIdx)
				triggeredIdx++
			}
		}
		assert.Equal(t, len(triggeredResp.Incidents), triggeredIdx, "not all triggered incidents were matched")
	})
}

func TestDevClient_ImplementsInterface(t *testing.T) {
	// Compile-time check that DevPagerDutyClient implements PagerDutyClientInterface
	var _ PagerDutyClientInterface = (*DevPagerDutyClient)(nil)
}

func TestNewDevConfig_Success(t *testing.T) {
	config, err := NewDevConfig(testFixturesDir)
	require.NoError(t, err)

	assert.NotNil(t, config.Client, "Client should be set")
	assert.NotNil(t, config.CurrentUser, "CurrentUser should be set")
	assert.Equal(t, "PDEV_USER_001", config.CurrentUser.ID)
	assert.NotEmpty(t, config.Teams, "Teams should not be empty")
	assert.NotEmpty(t, config.TeamsMemberIDs, "TeamsMemberIDs should not be empty")
	assert.NotEmpty(t, config.EscalationPolicies, "EscalationPolicies should not be empty")
}

func TestNewDevConfig_NonexistentDir(t *testing.T) {
	_, err := NewDevConfig("/nonexistent/path/to/fixtures")
	assert.Error(t, err, "should return error for nonexistent fixtures directory")
	assert.Contains(t, err.Error(), "NewDevConfig")
}

// newTestDevClient creates a DevPagerDutyClient loaded with test fixtures
func newTestDevClient(t *testing.T) *DevPagerDutyClient {
	t.Helper()
	fixtures, err := LoadFixtures(testFixturesDir)
	require.NoError(t, err)

	client, err := NewDevPagerDutyClient(fixtures)
	require.NoError(t, err)

	return client
}
