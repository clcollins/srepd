package pd

import (
	"context"
	"fmt"

	"github.com/PagerDuty/go-pagerduty"
)

const (
	defaultPageLimit = 100
	defaultOffset    = 0
)

var defaultIncidentStatues = []string{"triggered", "acknowledged"}

// PagerDutyClientInterface is an interface that defines the methods used by the pd package and makes it easier to mock
// calls to PagerDuty in tests
type PagerDutyClientInterface interface {
	CreateIncidentNoteWithContext(ctx context.Context, id string, note pagerduty.IncidentNote) (*pagerduty.IncidentNote, error)
	GetCurrentUserWithContext(ctx context.Context, opts pagerduty.GetCurrentUserOptions) (*pagerduty.User, error)
	GetIncidentWithContext(ctx context.Context, id string) (*pagerduty.Incident, error)
	GetTeamWithContext(ctx context.Context, id string) (*pagerduty.Team, error)
	GetUserWithContext(ctx context.Context, id string, opts pagerduty.GetUserOptions) (*pagerduty.User, error)
	ListIncidentAlertsWithContext(ctx context.Context, id string, opts pagerduty.ListIncidentAlertsOptions) (*pagerduty.ListAlertsResponse, error)
	ListIncidentsWithContext(ctx context.Context, opts pagerduty.ListIncidentsOptions) (*pagerduty.ListIncidentsResponse, error)
	ListIncidentNotesWithContext(ctx context.Context, id string) ([]pagerduty.IncidentNote, error)
	ManageIncidentsWithContext(ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) (*pagerduty.ListIncidentsResponse, error)
}

// PagerDutyClient implements PagerDutyClientInterface and is used by the pd package to make calls to PagerDuty
// This allows for mocking calls that would usually use the pagerduty.Client struct
type PagerDutyClient interface {
	PagerDutyClientInterface
}

// Config is a struct that holds the PagerDuty client used for all the PagerDuty calls, and the config info for
// teams, silent user, and ignored users
type Config struct {
	Client       PagerDutyClient
	Teams        []*pagerduty.Team
	SilentUser   *pagerduty.User
	IgnoredUsers []*pagerduty.User
}

func NewConfig(token string, teams []string, silentUser string, ignoredUsers []string) (*Config, error) {
	var c Config
	var err error

	c.Client = newClient(token)

	c.Teams, err = GetTeams(c.Client, teams)
	if err != nil {
		return &c, fmt.Errorf("NewConfig(): failed to get team(s) `%v`: %v", teams, err)
	}

	c.SilentUser, err = GetUser(c.Client, silentUser, pagerduty.GetUserOptions{})
	if err != nil {
		return &c, fmt.Errorf("NewConfig(): failed to get silent user: %v", err)
	}

	for _, i := range ignoredUsers {
		user, err := GetUser(c.Client, i, pagerduty.GetUserOptions{})
		if err != nil {
			return &c, fmt.Errorf("NewConfig(): failed to get user for ignore list `%v`: %v", i, err)
		}
		c.IgnoredUsers = append(c.IgnoredUsers, user)
	}

	return &c, nil
}

func newClient(token string) PagerDutyClient {
	return pagerduty.NewOAuthClient(token)
}

func NewListIncidentOptsFromDefaults() pagerduty.ListIncidentsOptions {
	return pagerduty.ListIncidentsOptions{
		Limit:    defaultPageLimit,
		Offset:   defaultOffset,
		Statuses: defaultIncidentStatues,
	}

}

func AcknowledgeIncident(client PagerDutyClient, incidents []*pagerduty.Incident, user *pagerduty.User) ([]pagerduty.Incident, error) {
	var ctx = context.Background()
	var i []pagerduty.Incident

	opts := []pagerduty.ManageIncidentsOptions{}

	for _, incident := range incidents {
		opts = append(opts, pagerduty.ManageIncidentsOptions{
			ID:     incident.ID,
			Status: "acknowledged",
			Assignments: []pagerduty.Assignee{{
				Assignee: user.APIObject,
			}},
		})
	}

	for {
		response, err := client.ManageIncidentsWithContext(ctx, user.Email, opts)
		if err != nil {
			return i, fmt.Errorf("AcknowledgeIncident(): failed to acknowledge incident(s) `%v`: %v", incidents, err)
		}

		i = append(i, response.Incidents...)

		if response.More {
			panic("AcknowledgeIncident(): PagerDuty response indicated more data available")
		}

		if !response.More {
			break
		}

	}

	return i, nil
}

func GetAlerts(client PagerDutyClient, id string, opts pagerduty.ListIncidentAlertsOptions) ([]pagerduty.IncidentAlert, error) {
	var ctx = context.Background()
	var a []pagerduty.IncidentAlert

	for {
		response, err := client.ListIncidentAlertsWithContext(ctx, id, opts)
		if err != nil {
			return a, fmt.Errorf("GetAlerts(): failed to get alerts for incident `%v`: %v", id, err)
		}

		a = append(a, response.Alerts...)

		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return a, nil
}

func GetIncident(client PagerDutyClient, id string) (*pagerduty.Incident, error) {
	var ctx = context.Background()
	var i *pagerduty.Incident

	i, err := client.GetIncidentWithContext(ctx, id)
	if err != nil {
		return i, fmt.Errorf("GetIncident(): failed to get incident `%v`: %v", id, err)
	}

	return i, nil
}

func GetIncidents(client PagerDutyClient, opts pagerduty.ListIncidentsOptions) ([]pagerduty.Incident, error) {
	var ctx = context.Background()
	var i []pagerduty.Incident

	for {
		response, err := client.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			return i, fmt.Errorf("GetIncidents(): failed to get incidents : %v", err)
		}

		i = append(i, response.Incidents...)

		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return i, nil
}

func GetNotes(client PagerDutyClient, id string) ([]pagerduty.IncidentNote, error) {
	var ctx = context.Background()
	var n []pagerduty.IncidentNote

	n, err := client.ListIncidentNotesWithContext(ctx, id)
	if err != nil {
		return n, fmt.Errorf("GetNotes(): failed to get incident notes `%v`: %v", id, err)
	}

	return n, nil
}

func GetTeams(client PagerDutyClient, teams []string) ([]*pagerduty.Team, error) {
	var ctx = context.Background()
	var t []*pagerduty.Team

	for _, i := range teams {
		team, err := client.GetTeamWithContext(ctx, i)
		if err != nil {
			return t, fmt.Errorf("GetTeams(): failed to find PagerDuty team `%v`: %v", i, err)
		}
		t = append(t, team)
	}

	return t, nil
}

func GetUser(client PagerDutyClient, id string, opts pagerduty.GetUserOptions) (*pagerduty.User, error) {
	var ctx = context.Background()
	var u *pagerduty.User

	u, err := client.GetUserWithContext(ctx, id, opts)
	if err != nil {
		return u, fmt.Errorf("GetUser(): failed to find PagerDuty user `%v`: %v", id, err)
	}

	return u, nil
}

func ReassignIncidents(client PagerDutyClient, incidents []*pagerduty.Incident, user *pagerduty.User, users []*pagerduty.User) ([]pagerduty.Incident, error) {
	var ctx = context.Background()
	var i []pagerduty.Incident

	a := []pagerduty.Assignee{}
	for _, user := range users {
		a = append(a, pagerduty.Assignee{Assignee: user.APIObject})
	}

	opts := []pagerduty.ManageIncidentsOptions{}

	for _, incident := range incidents {
		opts = append(opts, pagerduty.ManageIncidentsOptions{
			ID:          incident.ID,
			Assignments: a,
		})
	}

	// This loop is likely unnecessary, as the "More" response is probably not used by PagerDuty here
	// but I'm including it in case we need to use it in the future, and raising a panic if we receive
	// a "More" response so we can fix the code

	for {
		response, err := client.ManageIncidentsWithContext(ctx, user.Email, opts)
		if err != nil {
			return i, err
		}

		if response.More {
			// If we ever do get a "More" response, we we need to handle it, so panic to call attention to the problem
			panic("ReassignIncidents(): PagerDuty response indicated more data available")
		}

		i = append(i, response.Incidents...)

		if !response.More {
			break
		}
	}

	return i, nil
}

func PostNote(client PagerDutyClient, id string, user *pagerduty.User, content string) (*pagerduty.IncidentNote, error) {
	var ctx = context.Background()
	var n *pagerduty.IncidentNote

	note := pagerduty.IncidentNote{
		Content: content,
		User:    user.APIObject,
	}

	n, err := client.CreateIncidentNoteWithContext(ctx, id, note)
	if err != nil {
		return n, err
	}

	return n, nil
}
