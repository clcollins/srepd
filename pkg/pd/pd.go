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
	Client     *pagerduty.Client
	Teams      []*pagerduty.Team
	SilentUser *pagerduty.User
	ListOpts   *pagerduty.ListIncidentsOptions
}

func GetIncidents(ctx context.Context, c *Config) ([]pagerduty.Incident, error) {
	var i []pagerduty.Incident

	opts := *c.ListOpts
	opts.Statuses = pagerDutyDefaultStatuses

	for {
		response, err := c.Client.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			return i, err
		}

		for _, p := range response.Incidents {
			i = append(i, p)
		}

		opts.Offset += pageLimit

		if response.More != true {
			break
		}

	}

	return i, nil
}

func (c *Config) PopulateConfig(ctx context.Context, token string, teams []string, user string) error {
	c.Client = pagerduty.NewOAuthClient(token)

	c.ListOpts = &pagerduty.ListIncidentsOptions{
		TeamIDs: teams,
		Limit:   pageLimit,
		Offset:  defaultOffset,
	}

	// TODO: Capture this and store the current user info in the ctx
	_, err := c.Client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})
	if err != nil {
		return err
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
