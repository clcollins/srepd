package pd

import (
	"context"
	"fmt"
	"strings"

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
	GetEscalationPolicyWithContext(ctx context.Context, id string, opts *pagerduty.GetEscalationPolicyOptions) (*pagerduty.EscalationPolicy, error)
	GetIncidentWithContext(ctx context.Context, id string) (*pagerduty.Incident, error)
	GetTeamWithContext(ctx context.Context, id string) (*pagerduty.Team, error)
	ListMembersWithContext(ctx context.Context, id string, opts pagerduty.ListTeamMembersOptions) (*pagerduty.ListTeamMembersResponse, error)
	GetUserWithContext(ctx context.Context, id string, opts pagerduty.GetUserOptions) (*pagerduty.User, error)
	ListIncidentAlertsWithContext(ctx context.Context, id string, opts pagerduty.ListIncidentAlertsOptions) (*pagerduty.ListAlertsResponse, error)
	ListIncidentsWithContext(ctx context.Context, opts pagerduty.ListIncidentsOptions) (*pagerduty.ListIncidentsResponse, error)
	ListIncidentNotesWithContext(ctx context.Context, id string) ([]pagerduty.IncidentNote, error)
	ListOnCallsWithContext(ctx context.Context, opts pagerduty.ListOnCallOptions) (*pagerduty.ListOnCallsResponse, error)
	ManageIncidentsWithContext(ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) (*pagerduty.ListIncidentsResponse, error)
}

// PagerDutyClient implements PagerDutyClientInterface and is used by the pd package to make calls to PagerDuty
// This allows for mocking calls that would usually use the pagerduty.Client struct
type PagerDutyClient interface {
	PagerDutyClientInterface
}

// Config is a struct that holds the PagerDuty client used for all the PagerDuty calls, and the config info for
// teams, and ignored users
type Config struct {
	Client      PagerDutyClient
	CurrentUser *pagerduty.User

	// List of the users in the Teams
	TeamsMemberIDs     []string
	Teams              []*pagerduty.Team
	EscalationPolicies map[string]*pagerduty.EscalationPolicy

	IgnoredUsers []*pagerduty.User
}

func NewConfig(token string, teams []string, escalation_policies map[string]string, ignoredUsers []string) (*Config, error) {
	var c Config
	var err error

	c.Client = newClient(token)

	c.CurrentUser, err = c.Client.GetCurrentUserWithContext(context.Background(), pagerduty.GetCurrentUserOptions{})
	if err != nil {
		return &c, fmt.Errorf("pd.NewConfig(): failed to retrieve PagerDuty user: %v", err)
	}

	c.Teams, err = GetTeams(c.Client, teams)
	if err != nil {
		return &c, fmt.Errorf("pd.NewConfig(): failed to get team(s) `%v`: %v", teams, err)
	}

	c.TeamsMemberIDs, err = GetTeamMemberIDs(c.Client, c.Teams, pagerduty.ListTeamMembersOptions{Limit: defaultPageLimit, Offset: defaultOffset})
	if err != nil {
		return &c, fmt.Errorf("pd.NewConfig(): failed to get users(s) from teams: %v", err)
	}

	_, ok := escalation_policies["default"]
	if !ok {
		return &c, fmt.Errorf("pd.NewConfig(): escalation_policies map must contain a `default` key")
	}
	_, ok = escalation_policies["silent_default"]
	if !ok {
		return &c, fmt.Errorf("pd.NewConfig(): escalation_policies map must contain a `silent_default` key")
	}

	c.EscalationPolicies = make(map[string]*pagerduty.EscalationPolicy)

	for key, value := range escalation_policies {
		c.EscalationPolicies[strings.ToUpper(key)], err = GetEscalationPolicy(c.Client, value, pagerduty.GetEscalationPolicyOptions{})
		if err != nil {
			return &c, fmt.Errorf("pd.NewConfig(): failed to get escalation policy: (%s: %s) %v", key, value, err)
		}
	}

	for _, i := range ignoredUsers {
		user, err := GetUser(c.Client, i, pagerduty.GetUserOptions{})
		if err != nil {
			return &c, fmt.Errorf("pd.NewConfig(): failed to get user for ignore list `%v`: %v", i, err)
		}
		c.IgnoredUsers = append(c.IgnoredUsers, user)
	}

	return &c, nil
}

func newClient(token string) PagerDutyClient {
	return pagerduty.NewClient(token)
}

func NewListIncidentOptsFromDefaults() pagerduty.ListIncidentsOptions {
	return pagerduty.ListIncidentsOptions{
		Limit:    defaultPageLimit,
		Offset:   defaultOffset,
		Statuses: defaultIncidentStatues,
	}

}

func AcknowledgeIncident(client PagerDutyClient, incidents []pagerduty.Incident, user *pagerduty.User) ([]pagerduty.Incident, error) {
	var ctx = context.Background()
	var email string

	opts := []pagerduty.ManageIncidentsOptions{}

	for _, incident := range incidents {
		if user == nil {
			email = ""
			opts = append(opts, pagerduty.ManageIncidentsOptions{
				ID:              incident.ID,
				EscalationLevel: 1,
			})
		} else {
			email = user.Email
			opts = append(opts, pagerduty.ManageIncidentsOptions{
				ID:     incident.ID,
				Status: "acknowledged",
				Assignments: []pagerduty.Assignee{
					{
						Assignee: user.APIObject,
					},
				},
			})
		}
	}

	return loopManageIncidents(client, ctx, email, opts)
}

func GetAlerts(client PagerDutyClient, id string, opts pagerduty.ListIncidentAlertsOptions) ([]pagerduty.IncidentAlert, error) {
	var ctx = context.Background()
	var a []pagerduty.IncidentAlert

	for {
		response, err := client.ListIncidentAlertsWithContext(ctx, id, opts)
		if err != nil {
			return a, fmt.Errorf("pd.GetAlerts(): failed to get alerts for incident `%v`: %v", id, err)
		}

		a = append(a, response.Alerts...)

		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return a, nil
}

func GetEscalationPolicy(client PagerDutyClient, id string, opts pagerduty.GetEscalationPolicyOptions) (*pagerduty.EscalationPolicy, error) {
	var ctx = context.Background()
	var p *pagerduty.EscalationPolicy

	p, err := client.GetEscalationPolicyWithContext(ctx, id, &opts)
	if err != nil {
		return p, fmt.Errorf("pd.GetEscalationPolicy(): failed to get escalation policy: %v", err)
	}

	return p, nil
}

func GetIncident(client PagerDutyClient, id string) (*pagerduty.Incident, error) {
	var ctx = context.Background()
	var i *pagerduty.Incident

	i, err := client.GetIncidentWithContext(ctx, id)
	if err != nil {
		return i, fmt.Errorf("pd.GetIncident(): failed to get incident `%v`: %v", id, err)
	}

	return i, nil
}

func GetIncidents(client PagerDutyClient, opts pagerduty.ListIncidentsOptions) ([]pagerduty.Incident, error) {
	var ctx = context.Background()
	var i []pagerduty.Incident

	for {
		response, err := client.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			return i, fmt.Errorf("pd.GetIncidents(): failed to get incidents: %v", err)
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
		return n, fmt.Errorf("pd.GetNotes(): failed to get incident notes `%v`: %v", id, err)
	}

	return n, nil
}

func GetTeams(client PagerDutyClient, teams []string) ([]*pagerduty.Team, error) {
	var ctx = context.Background()
	var t []*pagerduty.Team

	for _, i := range teams {
		team, err := client.GetTeamWithContext(ctx, i)
		if err != nil {
			return t, fmt.Errorf("pd.GetTeams(): failed to find PagerDuty team `%v`: %v", i, err)
		}
		t = append(t, team)
	}

	return t, nil
}

func GetTeamMemberIDs(client PagerDutyClient, teams []*pagerduty.Team, opts pagerduty.ListTeamMembersOptions) ([]string, error) {
	var ctx = context.Background()
	var u []string

	for _, team := range teams {
		for {
			response, err := client.ListMembersWithContext(ctx, team.ID, opts)
			if err != nil {
				return u, fmt.Errorf("pd.GetUsers(): failed to retrieve users for PagerDuty team `%v`: %v", team.ID, err)
			}

			for _, member := range response.Members {
				u = append(u, member.User.ID)
			}

			opts.Offset += opts.Limit

			if !response.More {
				break
			}
		}
	}

	return u, nil
}

func GetUser(client PagerDutyClient, id string, opts pagerduty.GetUserOptions) (*pagerduty.User, error) {
	var ctx = context.Background()
	var u *pagerduty.User

	u, err := client.GetUserWithContext(ctx, id, opts)
	if err != nil {
		return u, fmt.Errorf("pd.GetUser(): failed to find PagerDuty user `%v`: %v", id, err)
	}

	return u, nil
}

func GetUserOnCalls(client PagerDutyClient, id string, opts pagerduty.ListOnCallOptions) ([]pagerduty.OnCall, error) {
	var ctx = context.Background()
	var o []pagerduty.OnCall

	for {
		response, err := client.ListOnCallsWithContext(ctx, opts)
		if err != nil {
			return o, fmt.Errorf("pd.GetUserOnCalls(): failed to get on-call entries for user `%v`: %v", id, err)
		}

		o = response.OnCalls

		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return o, nil
}

func loopManageIncidents(client PagerDutyClient, ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) (incidentList []pagerduty.Incident, err error) {
	for {
		response, err := client.ManageIncidentsWithContext(ctx, email, opts)
		if err != nil {
			return incidentList, err
		}

		incidentList = append(incidentList, response.Incidents...)

		// ManageIncidentsWithContext should never return a "More" response, but since it's in the ListIncidentsResponse API, check for it
		if response.More {
			panic("pd.loopManageIncidents(): PagerDuty response indicated more data available")
		}

		if !response.More {
			break
		}
	}

	return incidentList, err
}

// ReassignIncidents reassigns a list of incidents to a list of users
func ReassignIncidents(client PagerDutyClient, incidents []pagerduty.Incident, user *pagerduty.User, users []*pagerduty.User) ([]pagerduty.Incident, error) {
	var ctx = context.Background()

	a := []pagerduty.Assignee{}
	for _, user := range users {
		a = append(a, pagerduty.Assignee{Assignee: user.APIObject})
	}

	opts := []pagerduty.ManageIncidentsOptions{}

	for _, incident := range incidents {
		if incident.ID == "" {
			return nil, fmt.Errorf("pd.ReassignIncidents(): incident is nil")
		}
		opts = append(opts, pagerduty.ManageIncidentsOptions{
			ID:          incident.ID,
			Assignments: a,
		})
	}

	return loopManageIncidents(client, ctx, user.Email, opts)
}

// ReEscalateIncidents re-escalates a list of incidents to an escalation policy at a specific level
func ReEscalateIncidents(client PagerDutyClient, incidents []pagerduty.Incident, policy *pagerduty.EscalationPolicy, level uint) ([]pagerduty.Incident, error) {
	var ctx = context.Background()

	opts := []pagerduty.ManageIncidentsOptions{}

	for _, incident := range incidents {
		if incident.ID == "" {
			return nil, fmt.Errorf("pd.Re-escalateIncident(): incident is nil")
		}

		opts = append(opts, pagerduty.ManageIncidentsOptions{
			ID:               incident.ID,
			EscalationPolicy: &pagerduty.APIReference{ID: policy.ID, Type: "escalation_policy"},
			EscalationLevel:  level,
		})
	}

	return loopManageIncidents(client, ctx, "", opts)
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
