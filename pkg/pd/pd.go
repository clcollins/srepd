package pd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/log"
)

const (
	defaultPageLimit  = 100
	defaultOffset     = 0
	defaultAPITimeout = 30 * time.Second

	PolicyClassReal   = "REAL"
	PolicyClassSilent = "SILENT"
)

var defaultIncidentStatuses = []string{"triggered", "acknowledged"}

// contextWithTimeout returns a context with the default API timeout and its cancel func.
func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultAPITimeout)
}

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
	ListEscalationPoliciesWithContext(ctx context.Context, opts pagerduty.ListEscalationPoliciesOptions) (*pagerduty.ListEscalationPoliciesResponse, error)
	ListOnCallsWithContext(ctx context.Context, opts pagerduty.ListOnCallOptions) (*pagerduty.ListOnCallsResponse, error)
	ManageIncidentsWithContext(ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) (*pagerduty.ListIncidentsResponse, error)
	MergeIncidentsWithContext(ctx context.Context, from string, id string, o []pagerduty.MergeIncidentsOptions) (*pagerduty.Incident, error)
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
	TeamMembersByTeam  map[string][]string // team ID → member IDs
	Teams              []*pagerduty.Team
	EscalationPolicies map[string]*pagerduty.EscalationPolicy

	IgnoredUsers []*pagerduty.User
}

// NewConfig creates a new Config, initializing a real PagerDuty client from the token.
func NewConfig(token string, teams []string, escalation_policies map[string]string, ignoredUsers []string, defaultSilentPolicy string, customSilentPolicies map[string]string) (*Config, error) {
	return NewConfigWithClient(NewClient(token), teams, escalation_policies, ignoredUsers, defaultSilentPolicy, customSilentPolicies)
}

// NewConfigWithClient creates a Config using a pre-existing client.
// This enables testing with MockPagerDutyClient.
func NewConfigWithClient(client PagerDutyClient, teams []string, escalation_policies map[string]string, ignoredUsers []string, defaultSilentPolicy string, customSilentPolicies map[string]string) (*Config, error) {
	var c Config
	var err error

	c.Client = client

	ctx, cancel := contextWithTimeout()
	defer cancel()

	c.CurrentUser, err = c.Client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})
	if err != nil {
		return &c, fmt.Errorf("pd.NewConfig(): failed to retrieve PagerDuty user: %v", err)
	}

	c.Teams, err = GetTeams(c.Client, teams)
	if err != nil {
		return &c, fmt.Errorf("pd.NewConfig(): failed to get team(s) `%v`: %v", teams, err)
	}

	c.TeamsMemberIDs, c.TeamMembersByTeam, err = GetTeamMemberIDs(c.Client, c.Teams, pagerduty.ListTeamMembersOptions{Limit: defaultPageLimit, Offset: defaultOffset})
	if err != nil {
		return &c, fmt.Errorf("pd.NewConfig(): failed to get users(s) from teams: %v", err)
	}

	c.EscalationPolicies = make(map[string]*pagerduty.EscalationPolicy)

	if len(escalation_policies) > 0 {
		// Backward-compatible path: old service_escalation_policies config
		log.Info("pd.NewConfig(): using deprecated service_escalation_policies — migrate to default_silent_escalation_policy")
		_, ok := escalation_policies["default"]
		if !ok {
			return &c, fmt.Errorf("pd.NewConfig(): escalation_policies map must contain a `default` key")
		}
		_, ok = escalation_policies["silent_default"]
		if !ok {
			return &c, fmt.Errorf("pd.NewConfig(): escalation_policies map must contain a `silent_default` key")
		}

		for key, value := range escalation_policies {
			c.EscalationPolicies[strings.ToUpper(key)], err = GetEscalationPolicy(c.Client, value, pagerduty.GetEscalationPolicyOptions{})
			if err != nil {
				return &c, fmt.Errorf("pd.NewConfig(): failed to get escalation policy: (%s: %s) %v", key, value, err)
			}
		}
	} else if defaultSilentPolicy != "" {
		// New path: default_silent_escalation_policy
		policy, err := GetEscalationPolicy(c.Client, defaultSilentPolicy, pagerduty.GetEscalationPolicyOptions{})
		if err != nil {
			return &c, fmt.Errorf("pd.NewConfig(): failed to get default silent escalation policy `%v`: %w", defaultSilentPolicy, err)
		}
		c.EscalationPolicies["SILENT_DEFAULT"] = policy
		log.Info("pd.NewConfig(): loaded default silent policy", "id", defaultSilentPolicy, "name", policy.Name)

		for svcID, policyID := range customSilentPolicies {
			p, err := GetEscalationPolicy(c.Client, policyID, pagerduty.GetEscalationPolicyOptions{})
			if err != nil {
				return &c, fmt.Errorf("pd.NewConfig(): failed to get custom silent policy for service %s: %w", svcID, err)
			}
			c.EscalationPolicies[strings.ToUpper(svcID)] = p
			log.Info("pd.NewConfig(): loaded custom silent policy override", "service", svcID, "policy", policyID)
		}
	} else {
		log.Warn("pd.NewConfig(): no silent escalation policy configured — silencing will be disabled")
	}

	if len(ignoredUsers) > 0 {
		log.Info("pd.NewConfig(): using manual ignoredusers (deprecated — remove ignoredusers from config to use auto-discovery)")
		for _, i := range ignoredUsers {
			user, err := GetUser(c.Client, i, pagerduty.GetUserOptions{})
			if err != nil {
				return &c, fmt.Errorf("pd.NewConfig(): failed to get user for ignore list `%v`: %v", i, err)
			}
			c.IgnoredUsers = append(c.IgnoredUsers, user)
		}
	} else {
		silentUserIDs := ExtractSilentPolicyUsers(c.EscalationPolicies)
		for _, id := range silentUserIDs {
			user, err := GetUser(c.Client, id, pagerduty.GetUserOptions{})
			if err != nil {
				log.Warn("pd.NewConfig(): failed to get user from silent policy, skipping", "user_id", id, "error", err)
				continue
			}
			c.IgnoredUsers = append(c.IgnoredUsers, user)
		}
		if len(c.IgnoredUsers) > 0 {
			var ids []string
			for _, u := range c.IgnoredUsers {
				ids = append(ids, u.ID)
			}
			log.Info("pd.NewConfig(): auto-discovered ignored users from silent policies", "count", len(c.IgnoredUsers), "user_ids", ids)
		}
	}

	return &c, nil
}

func NewClient(token string) PagerDutyClient {
	return NewRateLimitedClient(pagerduty.NewClient(token))
}

func NewListIncidentOptsFromDefaults() pagerduty.ListIncidentsOptions {
	return pagerduty.ListIncidentsOptions{
		Limit:    defaultPageLimit,
		Offset:   defaultOffset,
		Statuses: defaultIncidentStatuses,
	}

}

func AcknowledgeIncident(client PagerDutyClient, incidents []pagerduty.Incident, user *pagerduty.User, currentUser *pagerduty.User) ([]pagerduty.Incident, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var email string

	opts := []pagerduty.ManageIncidentsOptions{}

	// Use currentUser's email for API call authentication
	if currentUser != nil {
		email = currentUser.Email
	}

	for _, incident := range incidents {
		if user == nil {
			// Un-acknowledge: set escalation level without assignment
			opts = append(opts, pagerduty.ManageIncidentsOptions{
				ID:              incident.ID,
				EscalationLevel: 1,
			})
		} else {
			// Acknowledge: assign to specific user
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
	ctx, cancel := contextWithTimeout()
	defer cancel()
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
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var p *pagerduty.EscalationPolicy

	p, err := client.GetEscalationPolicyWithContext(ctx, id, &opts)
	if err != nil {
		return p, fmt.Errorf("pd.GetEscalationPolicy(): failed to get escalation policy: %v", err)
	}

	return p, nil
}

func GetIncident(client PagerDutyClient, id string) (*pagerduty.Incident, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var i *pagerduty.Incident

	i, err := client.GetIncidentWithContext(ctx, id)
	if err != nil {
		return i, fmt.Errorf("pd.GetIncident(): failed to get incident `%v`: %v", id, err)
	}

	return i, nil
}

func GetIncidents(client PagerDutyClient, opts pagerduty.ListIncidentsOptions) ([]pagerduty.Incident, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var i []pagerduty.Incident

	for {
		response, err := client.ListIncidentsWithContext(ctx, opts)
		if err != nil {
			if strings.Contains(err.Error(), "414") {
				return i, fmt.Errorf("pd.GetIncidents(): too many team members (%d) for PagerDuty API query — try selecting fewer teams: %w", len(opts.UserIDs), err)
			}
			return i, fmt.Errorf("pd.GetIncidents(): failed to get incidents: %w", err)
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
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var n []pagerduty.IncidentNote

	n, err := client.ListIncidentNotesWithContext(ctx, id)
	if err != nil {
		return n, fmt.Errorf("pd.GetNotes(): failed to get incident notes `%v`: %v", id, err)
	}

	return n, nil
}

func GetTeams(client PagerDutyClient, teams []string) ([]*pagerduty.Team, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
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

func GetTeamMemberIDs(client PagerDutyClient, teams []*pagerduty.Team, opts pagerduty.ListTeamMembersOptions) ([]string, map[string][]string, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var allIDs []string
	byTeam := make(map[string][]string)

	for _, team := range teams {
		opts.Offset = defaultOffset
		for {
			response, err := client.ListMembersWithContext(ctx, team.ID, opts)
			if err != nil {
				return allIDs, byTeam, fmt.Errorf("pd.GetUsers(): failed to retrieve users for PagerDuty team `%v`: %v", team.ID, err)
			}

			for _, member := range response.Members {
				allIDs = append(allIDs, member.User.ID)
				byTeam[team.ID] = append(byTeam[team.ID], member.User.ID)
			}

			opts.Offset += opts.Limit

			if !response.More {
				break
			}
		}
	}

	return allIDs, byTeam, nil
}

func GetUser(client PagerDutyClient, id string, opts pagerduty.GetUserOptions) (*pagerduty.User, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var u *pagerduty.User

	u, err := client.GetUserWithContext(ctx, id, opts)
	if err != nil {
		return u, fmt.Errorf("pd.GetUser(): failed to find PagerDuty user `%v`: %v", id, err)
	}

	return u, nil
}

func GetCurrentUserTeams(client PagerDutyClient) ([]pagerduty.Team, error) {
	user, err := GetCurrentUserWithTeams(client)
	if err != nil {
		return nil, fmt.Errorf("pd.GetCurrentUserTeams(): failed to get current user: %w", err)
	}
	return user.Teams, nil
}

// GetCurrentUserWithTeams returns the currently-authenticated user with
// their teams included — one API call serving both identity (greeting,
// validation feedback) and team discovery.
func GetCurrentUserWithTeams(client PagerDutyClient) (*pagerduty.User, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()

	user, err := client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{
		Includes: []string{"teams"},
	})
	if err != nil {
		return nil, fmt.Errorf("pd.GetCurrentUserWithTeams(): failed to get current user: %w", err)
	}
	return user, nil
}

// GetCurrentUser returns the currently-authenticated PagerDuty user, applying the
// default API timeout. It exists so callers do not open-code
// GetCurrentUserWithContext(context.Background(), ...), which has no timeout.
func GetCurrentUser(client PagerDutyClient) (*pagerduty.User, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()

	user, err := client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})
	if err != nil {
		return nil, fmt.Errorf("pd.GetCurrentUser(): failed to get current user: %w", err)
	}

	return user, nil
}

func GetUserOnCalls(client PagerDutyClient, id string, opts pagerduty.ListOnCallOptions) ([]pagerduty.OnCall, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var o []pagerduty.OnCall

	for {
		response, err := client.ListOnCallsWithContext(ctx, opts)
		if err != nil {
			return o, fmt.Errorf("pd.GetUserOnCalls(): failed to get on-call entries for user `%v`: %v", id, err)
		}

		o = append(o, response.OnCalls...)

		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return o, nil
}

func loopManageIncidents(client PagerDutyClient, ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) ([]pagerduty.Incident, error) {
	log.Debug("pd.loopManageIncidents", "email", email, "opts_count", len(opts))
	response, err := client.ManageIncidentsWithContext(ctx, email, opts)
	if err != nil {
		log.Error("pd.loopManageIncidents", "error", err, "email", email)
		return nil, err
	}

	// ManageIncidentsWithContext should never return a "More" response, but since it's in the ListIncidentsResponse API, check for it
	if response.More {
		return nil, fmt.Errorf("pd.loopManageIncidents(): unexpected pagination response from ManageIncidents API")
	}

	return response.Incidents, nil
}

// ReassignIncidents reassigns a list of incidents to a list of users
func ReassignIncidents(client PagerDutyClient, incidents []pagerduty.Incident, user *pagerduty.User, users []*pagerduty.User) ([]pagerduty.Incident, error) {
	if user == nil {
		return nil, fmt.Errorf("pd.ReassignIncidents(): from-user is nil")
	}

	ctx, cancel := contextWithTimeout()
	defer cancel()

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
func ReEscalateIncidents(client PagerDutyClient, incidents []pagerduty.Incident, user *pagerduty.User, policy *pagerduty.EscalationPolicy, level uint) ([]pagerduty.Incident, error) {
	if user == nil {
		return nil, fmt.Errorf("pd.ReEscalateIncidents(): user is nil")
	}
	if policy == nil {
		return nil, fmt.Errorf("pd.ReEscalateIncidents(): policy is nil")
	}

	ctx, cancel := contextWithTimeout()
	defer cancel()

	opts := []pagerduty.ManageIncidentsOptions{}

	for _, incident := range incidents {
		if incident.ID == "" {
			return nil, fmt.Errorf("pd.Re-escalateIncident(): incident is nil")
		}

		opt := pagerduty.ManageIncidentsOptions{
			ID:              incident.ID,
			EscalationLevel: level,
		}

		// PagerDuty restarts escalation at level 1 whenever escalation_policy is
		// set on an incident. So we only send escalation_policy when we are
		// *moving* the incident to a DIFFERENT policy (e.g. silencing to a silent
		// policy). For an in-place re-escalation — where the target policy is the
		// incident's current policy — we omit escalation_policy so the requested
		// EscalationLevel actually takes effect instead of being reset to level 1.
		if policy.ID != incident.EscalationPolicy.ID {
			opt.EscalationPolicy = &pagerduty.APIReference{ID: policy.ID, Type: "escalation_policy"}
		}

		opts = append(opts, opt)
	}

	return loopManageIncidents(client, ctx, user.Email, opts)
}

func UpdateIncidentTitle(client PagerDutyClient, incidentID string, newTitle string, currentUser *pagerduty.User) ([]pagerduty.Incident, error) {
	if currentUser == nil {
		return nil, fmt.Errorf("pd.UpdateIncidentTitle(): user is nil")
	}
	if incidentID == "" {
		return nil, fmt.Errorf("pd.UpdateIncidentTitle(): incident ID is empty")
	}
	if newTitle == "" {
		return nil, fmt.Errorf("pd.UpdateIncidentTitle(): title is empty")
	}

	ctx, cancel := contextWithTimeout()
	defer cancel()

	opts := []pagerduty.ManageIncidentsOptions{
		{
			ID:    incidentID,
			Type:  "incident",
			Title: newTitle,
		},
	}

	return loopManageIncidents(client, ctx, currentUser.Email, opts)
}

func PostNote(client PagerDutyClient, id string, user *pagerduty.User, content string) (*pagerduty.IncidentNote, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var n *pagerduty.IncidentNote

	note := pagerduty.IncidentNote{
		Content: content,
		User:    user.APIObject,
	}

	n, err := client.CreateIncidentNoteWithContext(ctx, id, note)
	if err != nil {
		return n, fmt.Errorf("pd.PostNote(): failed to create note for incident %v: %w", id, err)
	}

	return n, nil
}

// ClassifyEscalationPolicy returns PolicyClassReal if any target in any
// escalation rule is a schedule_reference (meaning incidents can reach
// on-call humans). Returns PolicyClassSilent if all targets are
// user_reference only, or if the policy has no rules.
func ClassifyEscalationPolicy(policy *pagerduty.EscalationPolicy) string {
	if policy == nil {
		return PolicyClassSilent
	}
	for _, rule := range policy.EscalationRules {
		for _, target := range rule.Targets {
			if target.Type == "schedule_reference" {
				return PolicyClassReal
			}
		}
	}
	return PolicyClassSilent
}

// ExtractSilentPolicyUsers collects the deduplicated, sorted user IDs
// from all SILENT escalation policies. These are the bot/placeholder
// users whose assigned incidents should be filtered from the view.
func ExtractSilentPolicyUsers(policies map[string]*pagerduty.EscalationPolicy) []string {
	seen := make(map[string]bool)
	var userIDs []string

	for _, policy := range policies {
		if ClassifyEscalationPolicy(policy) != PolicyClassSilent {
			continue
		}
		for _, rule := range policy.EscalationRules {
			for _, target := range rule.Targets {
				if target.Type == "user_reference" && !seen[target.ID] {
					seen[target.ID] = true
					userIDs = append(userIDs, target.ID)
				}
			}
		}
	}

	if len(userIDs) == 0 {
		return nil
	}
	sort.Strings(userIDs)
	return userIDs
}

// GetTeamEscalationPolicies fetches all escalation policies associated with the given team IDs.
func GetTeamEscalationPolicies(client PagerDutyClient, teamIDs []string) ([]pagerduty.EscalationPolicy, error) {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	var policies []pagerduty.EscalationPolicy

	if len(teamIDs) == 0 {
		return policies, nil
	}

	opts := pagerduty.ListEscalationPoliciesOptions{
		TeamIDs: teamIDs,
		Limit:   defaultPageLimit,
		Offset:  defaultOffset,
	}

	for {
		response, err := client.ListEscalationPoliciesWithContext(ctx, opts)
		if err != nil {
			return policies, fmt.Errorf("pd.GetTeamEscalationPolicies(): failed to list escalation policies: %w", err)
		}

		policies = append(policies, response.EscalationPolicies...)

		opts.Offset += opts.Limit

		if !response.More {
			break
		}
	}

	return policies, nil
}
