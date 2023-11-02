package pd

import (
	"context"
	"fmt"

	"github.com/PagerDuty/go-pagerduty"
)

const (
	pageLimit     = 100
	defaultOffset = 0
)

var pagerDutyDefaultStatuses = []string{"triggered", "acknowledged"}

type Config struct {
	Client          *pagerduty.Client
	Teams           []*pagerduty.Team
	SilentUser      *pagerduty.User
	DefaultListOpts *pagerduty.ListIncidentsOptions
}

func GetSingleIncident(ctx context.Context, c *Config, id string) (*pagerduty.Incident, error) {
	incident, err := c.Client.GetIncidentWithContext(ctx, id)
	if err != nil {
		return incident, err
	}
	return incident, nil
}

func GetIncidents(ctx context.Context, c *Config, opts pagerduty.ListIncidentsOptions) ([]pagerduty.Incident, error) {
	var i []pagerduty.Incident

	for {
		response, err := c.Client.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			return i, err
		}

		i = append(i, response.Incidents...)

		opts.Offset += pageLimit

		if !response.More {
			break
		}
	}

	return i, nil
}

func (c *Config) PopulateConfig(ctx context.Context, token string, teams []string, user string) error {
	c.Client = pagerduty.NewOAuthClient(token)

	c.DefaultListOpts = &pagerduty.ListIncidentsOptions{
		TeamIDs:  teams,
		Limit:    pageLimit,
		Offset:   defaultOffset,
		Statuses: pagerDutyDefaultStatuses,
	}

	for _, i := range teams {
		team, err := c.Client.GetTeamWithContext(ctx, i)
		if err != nil {
			return fmt.Errorf("failed to find PagerDuty team `%v`: %v", i, err)
		}
		c.Teams = append(c.Teams, team)
	}

	silentuser, err := c.Client.GetUserWithContext(ctx, user, pagerduty.GetUserOptions{})
	if err != nil {
		return fmt.Errorf("failed to find PagerDuty user for silencing alerts `%v`: %v", user, err)
	}
	c.SilentUser = silentuser

	return nil
}
