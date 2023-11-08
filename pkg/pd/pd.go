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

type Config struct {
	Client          *pagerduty.Client
	Teams           []*pagerduty.Team
	SilentUser      *pagerduty.User
	IgnoreUsers     []*pagerduty.User
	DefaultListOpts *pagerduty.ListIncidentsOptions
}

// GetTeamsAsStrings returns a []string of team IDs from the Config
// This is used to populate the pagerduty.ListIncidentsOptions.TeamIDs, etc
func (c *Config) GetTeamsAsStrings() []string {
	var s []string
	for _, t := range c.Teams {
		s = append(s, t.ID)
	}
	return s
}

// NewListIncidentAlertsOptsFromDefaults accepts a *Config and returns a pagerduty.ListIncidentAlertsOptions
// with reasonable paging defaults
func NewListIncidentAlertsOptsFromDefaults(c *Config) pagerduty.ListIncidentAlertsOptions {
	return pagerduty.ListIncidentAlertsOptions{
		Limit:    defaultPageLimit,
		Offset:   defaultOffset,
		Statuses: defaultIncidentStatues,
	}
}

func GetAlerts(ctx context.Context, c *Config, id string) ([]pagerduty.IncidentAlert, error) {
	var a []pagerduty.IncidentAlert

	for {
		opts := NewListIncidentAlertsOptsFromDefaults(c)

		response, err := c.Client.ListIncidentAlertsWithContext(ctx, id, opts)
		if err != nil {
			return a, err
		}

		a = append(a, response.Alerts...)

		// increment the offset by the page limit to get the next page
		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return a, nil
}

// GetNotes accepts a context, pdConfig, and incident ID; no options required
// and returns a []pagerduty.IncidentNote
func GetNotes(ctx context.Context, c *Config, id string) ([]pagerduty.IncidentNote, error) {
	var n []pagerduty.IncidentNote
	n, err := c.Client.ListIncidentNotesWithContext(ctx, id)
	if err != nil {
		return n, err
	}

	return n, nil
}

func AddNoteToIncident(ctx context.Context, c *Config, id string, user *pagerduty.User, content string) (*pagerduty.IncidentNote, error) {
	var n *pagerduty.IncidentNote

	note := pagerduty.IncidentNote{
		Content: content,
		User:    user.APIObject,
	}

	n, err := c.Client.CreateIncidentNoteWithContext(ctx, id, note)
	if err != nil {
		return n, err
	}

	return n, nil
}

// GetSingleIncident accepts a context, pdConfig, and incident ID
// and returns a single *pagerduty.Incident
// There are no options for this endpoint
func GetSingleIncident(ctx context.Context, c *Config, id string) (*pagerduty.Incident, error) {
	incident, err := c.Client.GetIncidentWithContext(ctx, id)
	if err != nil {
		return incident, err
	}
	return incident, nil
}

// NewListIncidentOptsFromDefaults accepts a *Config and returns a pagerduty.ListIncidentsOptions
// with reasonable defaults for paging, retrieving only triggered and acknowledged incidents,
// and the team IDs from the config
func NewListIncidentOptsFromDefaults(c *Config) pagerduty.ListIncidentsOptions {
	return pagerduty.ListIncidentsOptions{
		TeamIDs:  c.GetTeamsAsStrings(),
		Limit:    defaultPageLimit,
		Offset:   defaultOffset,
		Statuses: defaultIncidentStatues,
	}
}

// GetIncidents accepts a context, pdConfig, and pagerduty.ListIncidentsOptions
// We can shoot ourselves in the foot here, if we don't pass reasonable defaults in the opts
func GetIncidents(c *Config, opts pagerduty.ListIncidentsOptions) ([]pagerduty.Incident, error) {
	var ctx = context.Background()
	var i []pagerduty.Incident

	for {
		response, err := c.Client.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			return i, err
		}

		i = append(i, response.Incidents...)

		// increment the offset by the page limit to get the next page
		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return i, nil
}

// ReassignIncident accepts a context, pdConfig, valid user email (current user), []pagerduty.Incident, and []pagerduty.User to assign
func ReassignIncident(ctx context.Context, c *Config, email string, incidents []pagerduty.Incident, users []*pagerduty.User) ([]pagerduty.Incident, error) {
	var i []pagerduty.Incident

	a := []pagerduty.Assignee{}
	for _, user := range users {
		a = append(a, pagerduty.Assignee{Assignee: user.APIObject})
	}

	manageOpts := []pagerduty.ManageIncidentsOptions{}
	for _, incident := range incidents {
		manageOpts = append(manageOpts, pagerduty.ManageIncidentsOptions{
			ID:          incident.ID,
			Assignments: a,
		})
	}

	// This loop is likely unnecessary, as the "More" response is probably not used by PagerDuty here
	// but I'm including it in case we need to use it in the future, and raising a panic if we receive
	// a "More" response so we can fix the code
	for {
		response, err := c.Client.ManageIncidentsWithContext(ctx, email, manageOpts)
		if err != nil {
			return i, err
		}

		i = append(i, response.Incidents...)

		// I don't think we'll ever get "More", but if so, we need to panic and raise the issue it so we can fix the code
		if response.More {
			panic("(reassignIncident): received more than one page of incidents, but there is no paging option")
		}

		// This should still work to break out of the loop
		if !response.More {
			break
		}
	}

	return i, nil
}

func AcknowledgeIncident(ctx context.Context, c *Config, email string, incidents []pagerduty.Incident) ([]pagerduty.Incident, error) {
	var i []pagerduty.Incident

	manageOpts := []pagerduty.ManageIncidentsOptions{}
	for _, incident := range incidents {
		manageOpts = append(manageOpts, pagerduty.ManageIncidentsOptions{
			ID:     incident.ID,
			Status: "acknowledged",
		})
	}

	for {
		response, err := c.Client.ManageIncidentsWithContext(ctx, email, manageOpts)
		if err != nil {
			return i, err
		}

		i = append(i, response.Incidents...)

		// I don't think we'll ever get "More", but if so, we need to panic and raise the issue it so we can fix the code
		if response.More {
			panic("(acknowledgeIncident): received more than one page of incidents, but there is no paging option")
		}

		// This should still work to break out of the loop
		if !response.More {
			break
		}

	}

	return i, nil
}

// PopulateConfig generates a pagerduty client, and populates the teams and silent user in the *Config struct
func NewConfig(token string, teams []string, user string, ignoreusers []string) (*Config, error) {
	ctx := context.Background()
	c := &Config{}
	c.Client = pagerduty.NewOAuthClient(token)

	for _, i := range teams {
		team, err := c.Client.GetTeamWithContext(ctx, i)
		if err != nil {
			return c, fmt.Errorf("failed to find PagerDuty team `%v`: %v", i, err)
		}
		c.Teams = append(c.Teams, team)
	}

	silentuser, err := c.Client.GetUserWithContext(ctx, user, pagerduty.GetUserOptions{})
	if err != nil {
		return c, fmt.Errorf("failed to find PagerDuty user for silencing alerts `%v`: %v", user, err)
	}
	c.SilentUser = silentuser

	for _, i := range ignoreusers {
		ignoreuser, err := c.Client.GetUserWithContext(ctx, i, pagerduty.GetUserOptions{})
		if err != nil {
			return c, fmt.Errorf("failed to find PagerDuty user to ignore `%v`: %v", i, err)
		}
		c.IgnoreUsers = append(c.IgnoreUsers, ignoreuser)
	}

	return c, nil
}
