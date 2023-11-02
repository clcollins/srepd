package tui

import (
	"context"
	"log"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/clcollins/srepd/pkg/pd"
)

type getCurrentUserMsg string
type gotCurrentUserMsg *pagerduty.User

func getCurrentUser(ctx context.Context, pdConfig *pd.Config) tea.Cmd {
	if debug {
		log.Printf("getCurrentUser")
	}
	return func() tea.Msg {
		u, err := pdConfig.Client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})
		if err != nil {
			return errMsg{err}
		}
		return gotCurrentUserMsg(u)
	}
}

type getSilentUserMsg string
type gotSilentUserMsg *pagerduty.User

func getUser(ctx context.Context, pdConfig *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		u, err := pdConfig.Client.GetUserWithContext(ctx, id, pagerduty.GetUserOptions{})
		if err != nil {
			return errMsg{err}
		}
		return gotSilentUserMsg(u)
	}
}

func AssignedToUser(i pagerduty.Incident, id string) bool {
	for _, a := range i.Assignments {
		if a.Assignee.ID == id {
			return true
		}
	}
	return false
}

type getIncidentsMsg string
type gotIncidentsMsg []pagerduty.Incident

func getIncidents(ctx context.Context, pdConfig *pd.Config) tea.Cmd {
	if debug {
		log.Printf("getIncidents")
	}
	return func() tea.Msg {
		opts := pagerduty.ListIncidentsOptions{
			TeamIDs:  pdConfig.DefaultListOpts.TeamIDs,
			Limit:    pdConfig.DefaultListOpts.Limit,
			Offset:   pdConfig.DefaultListOpts.Offset,
			Statuses: pdConfig.DefaultListOpts.Statuses,
		}

		i, err := pd.GetIncidents(ctx, pdConfig, opts)
		if err != nil {
			return errMsg{err}
		}
		return gotIncidentsMsg(i)
	}
}

type getSingleIncidentMsg string
type gotSingleIncidentMsg *pagerduty.Incident

func getSingleIncident(ctx context.Context, pdConfig *pd.Config, id string) tea.Cmd {
	return func() tea.Msg {
		i, err := pd.GetSingleIncident(ctx, pdConfig, id)
		if err != nil {
			return errMsg{err}
		}
		return gotSingleIncidentMsg(i)

	}
}
