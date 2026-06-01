package pd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/rand"
)

// FixtureConfig holds the dev mode configuration data loaded from config.json
type FixtureConfig struct {
	User               fixtureUser                        `json:"user"`
	Teams              []fixtureTeam                      `json:"teams"`
	TeamMembers        []fixtureUser                      `json:"team_members"`
	EscalationPolicies map[string]fixtureEscalationPolicy `json:"escalation_policies"`
}

type fixtureUser struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Type    string `json:"type"`
	Self    string `json:"self"`
	HTMLURL string `json:"html_url"`
}

type fixtureTeam struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Self    string `json:"self"`
	HTMLURL string `json:"html_url"`
}

type fixtureEscalationPolicy struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// fixtureIncident is a simplified incident for JSON unmarshaling.
// Fields match the PagerDuty API JSON structure.
type fixtureIncident struct {
	ID                 string              `json:"id"`
	Type               string              `json:"type"`
	Self               string              `json:"self"`
	HTMLURL            string              `json:"html_url"`
	IncidentNumber     uint                `json:"incident_number"`
	Title              string              `json:"title"`
	Status             string              `json:"status"`
	Urgency            string              `json:"urgency"`
	CreatedAt          string              `json:"created_at"`
	LastStatusChangeAt string              `json:"last_status_change_at"`
	Service            fixtureServiceRef   `json:"service"`
	EscalationPolicy   fixtureAPIRef       `json:"escalation_policy"`
	Assignments        []fixtureAssignment `json:"assignments"`
	Acknowledgements   []fixtureAck        `json:"acknowledgements"`
}

type fixtureServiceRef struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
	Self    string `json:"self"`
	HTMLURL string `json:"html_url"`
}

type fixtureAPIRef struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
}

type fixtureAssignment struct {
	At       string        `json:"at"`
	Assignee fixtureAPIRef `json:"assignee"`
}

type fixtureAck struct {
	At           string        `json:"at"`
	Acknowledger fixtureAPIRef `json:"acknowledger"`
}

// fixtureAlert is a simplified alert for JSON unmarshaling
type fixtureAlert struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	HTMLURL   string                 `json:"html_url"`
	Status    string                 `json:"status"`
	CreatedAt string                 `json:"created_at"`
	Service   fixtureServiceRef      `json:"service"`
	Incident  fixtureAPIRef          `json:"incident"`
	Body      map[string]interface{} `json:"body"`
}

// fixtureNote is a simplified note for JSON unmarshaling
type fixtureNote struct {
	ID        string        `json:"id"`
	Content   string        `json:"content"`
	CreatedAt string        `json:"created_at"`
	User      fixtureAPIRef `json:"user"`
}

// Fixtures holds all loaded fixture data
type Fixtures struct {
	Incidents []fixtureIncident
	Alerts    map[string][]fixtureAlert
	Notes     map[string][]fixtureNote
	Config    FixtureConfig
}

// LoadFixtures reads JSON fixture files from the given directory
func LoadFixtures(dir string) (*Fixtures, error) {
	var fixtures Fixtures

	// Load incidents
	incidentsData, err := os.ReadFile(filepath.Join(dir, "incidents.json"))
	if err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to read incidents.json: %w", err)
	}
	if err := json.Unmarshal(incidentsData, &fixtures.Incidents); err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to parse incidents.json: %w", err)
	}

	// Load alerts
	alertsData, err := os.ReadFile(filepath.Join(dir, "alerts.json"))
	if err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to read alerts.json: %w", err)
	}
	if err := json.Unmarshal(alertsData, &fixtures.Alerts); err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to parse alerts.json: %w", err)
	}

	// Load notes
	notesData, err := os.ReadFile(filepath.Join(dir, "notes.json"))
	if err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to read notes.json: %w", err)
	}
	if err := json.Unmarshal(notesData, &fixtures.Notes); err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to parse notes.json: %w", err)
	}

	// Load config
	configData, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to read config.json: %w", err)
	}
	if err := json.Unmarshal(configData, &fixtures.Config); err != nil {
		return nil, fmt.Errorf("LoadFixtures: failed to parse config.json: %w", err)
	}

	log.Debug("LoadFixtures", "incidents", len(fixtures.Incidents), "alert_keys", len(fixtures.Alerts), "note_keys", len(fixtures.Notes))
	return &fixtures, nil
}

// DevPagerDutyClient implements PagerDutyClientInterface with in-memory mutable state
// for development and testing without a live PagerDuty connection.
type DevPagerDutyClient struct {
	mu        sync.RWMutex
	incidents map[string]*pagerduty.Incident
	alerts    map[string][]pagerduty.IncidentAlert
	notes     map[string][]pagerduty.IncidentNote

	currentUser        *pagerduty.User
	teams              map[string]*pagerduty.Team
	teamMembers        []pagerduty.Member
	escalationPolicies map[string]*pagerduty.EscalationPolicy
	users              map[string]*pagerduty.User
}

// NewDevPagerDutyClient creates a new DevPagerDutyClient from loaded fixtures
func NewDevPagerDutyClient(fixtures *Fixtures) (*DevPagerDutyClient, error) {
	client := &DevPagerDutyClient{
		incidents:          make(map[string]*pagerduty.Incident),
		alerts:             make(map[string][]pagerduty.IncidentAlert),
		notes:              make(map[string][]pagerduty.IncidentNote),
		teams:              make(map[string]*pagerduty.Team),
		escalationPolicies: make(map[string]*pagerduty.EscalationPolicy),
		users:              make(map[string]*pagerduty.User),
	}

	// Convert fixture user to PagerDuty user
	client.currentUser = &pagerduty.User{
		APIObject: pagerduty.APIObject{
			ID:      fixtures.Config.User.ID,
			Type:    fixtures.Config.User.Type,
			Self:    fixtures.Config.User.Self,
			HTMLURL: fixtures.Config.User.HTMLURL,
		},
		Name:  fixtures.Config.User.Name,
		Email: fixtures.Config.User.Email,
	}
	client.users[fixtures.Config.User.ID] = client.currentUser

	// Convert fixture team members to PagerDuty users and members
	for _, fm := range fixtures.Config.TeamMembers {
		user := &pagerduty.User{
			APIObject: pagerduty.APIObject{
				ID:   fm.ID,
				Type: fm.Type,
			},
			Name:  fm.Name,
			Email: fm.Email,
		}
		client.users[fm.ID] = user
		client.teamMembers = append(client.teamMembers, pagerduty.Member{
			User: pagerduty.APIObject{
				ID:   fm.ID,
				Type: fm.Type,
			},
		})
	}

	// Convert fixture teams
	for _, ft := range fixtures.Config.Teams {
		client.teams[ft.ID] = &pagerduty.Team{
			APIObject: pagerduty.APIObject{
				ID:      ft.ID,
				Type:    ft.Type,
				Self:    ft.Self,
				HTMLURL: ft.HTMLURL,
			},
			Name: ft.Name,
		}
	}

	// Convert fixture escalation policies
	for key, fp := range fixtures.Config.EscalationPolicies {
		client.escalationPolicies[fp.ID] = &pagerduty.EscalationPolicy{
			APIObject: pagerduty.APIObject{
				ID:   fp.ID,
				Type: fp.Type,
			},
			Name: fp.Name,
		}
		// Also store by config key name for lookup
		client.escalationPolicies[key] = client.escalationPolicies[fp.ID]
	}

	// Convert fixture incidents to PagerDuty incidents, preserving fixture order
	for _, fi := range fixtures.Incidents {
		incident := convertFixtureIncident(fi)
		client.incidents[fi.ID] = incident
	}

	// Convert fixture alerts
	for incidentID, fixtureAlerts := range fixtures.Alerts {
		var pdAlerts []pagerduty.IncidentAlert
		for _, fa := range fixtureAlerts {
			pdAlerts = append(pdAlerts, convertFixtureAlert(fa))
		}
		client.alerts[incidentID] = pdAlerts
	}

	// Convert fixture notes
	for incidentID, fixtureNotes := range fixtures.Notes {
		var pdNotes []pagerduty.IncidentNote
		for _, fn := range fixtureNotes {
			pdNotes = append(pdNotes, convertFixtureNote(fn))
		}
		client.notes[incidentID] = pdNotes
	}

	return client, nil
}

// convertFixtureIncident converts a fixture incident to a PagerDuty incident
func convertFixtureIncident(fi fixtureIncident) *pagerduty.Incident {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID:      fi.ID,
			Type:    fi.Type,
			Self:    fi.Self,
			HTMLURL: fi.HTMLURL,
		},
		IncidentNumber:     fi.IncidentNumber,
		Title:              fi.Title,
		Status:             fi.Status,
		Urgency:            fi.Urgency,
		CreatedAt:          fi.CreatedAt,
		LastStatusChangeAt: fi.LastStatusChangeAt,
		Service: pagerduty.APIObject{
			ID:      fi.Service.ID,
			Type:    fi.Service.Type,
			Summary: fi.Service.Summary,
			Self:    fi.Service.Self,
			HTMLURL: fi.Service.HTMLURL,
		},
		EscalationPolicy: pagerduty.APIObject{
			ID:      fi.EscalationPolicy.ID,
			Type:    fi.EscalationPolicy.Type,
			Summary: fi.EscalationPolicy.Summary,
		},
	}

	for _, fa := range fi.Assignments {
		incident.Assignments = append(incident.Assignments, pagerduty.Assignment{
			At: fa.At,
			Assignee: pagerduty.APIObject{
				ID:      fa.Assignee.ID,
				Type:    fa.Assignee.Type,
				Summary: fa.Assignee.Summary,
			},
		})
	}

	for _, fack := range fi.Acknowledgements {
		incident.Acknowledgements = append(incident.Acknowledgements, pagerduty.Acknowledgement{
			At: fack.At,
			Acknowledger: pagerduty.APIObject{
				ID:      fack.Acknowledger.ID,
				Type:    fack.Acknowledger.Type,
				Summary: fack.Acknowledger.Summary,
			},
		})
	}

	return incident
}

// convertFixtureAlert converts a fixture alert to a PagerDuty IncidentAlert
func convertFixtureAlert(fa fixtureAlert) pagerduty.IncidentAlert {
	return pagerduty.IncidentAlert{
		APIObject: pagerduty.APIObject{
			ID:      fa.ID,
			Type:    fa.Type,
			HTMLURL: fa.HTMLURL,
		},
		Status:    fa.Status,
		CreatedAt: fa.CreatedAt,
		Service: pagerduty.APIObject{
			ID:      fa.Service.ID,
			Type:    fa.Service.Type,
			Summary: fa.Service.Summary,
		},
		Incident: pagerduty.APIReference{
			ID:   fa.Incident.ID,
			Type: fa.Incident.Type,
		},
		Body: fa.Body,
	}
}

// convertFixtureNote converts a fixture note to a PagerDuty IncidentNote
func convertFixtureNote(fn fixtureNote) pagerduty.IncidentNote {
	return pagerduty.IncidentNote{
		ID:        fn.ID,
		Content:   fn.Content,
		CreatedAt: fn.CreatedAt,
		User: pagerduty.APIObject{
			ID:      fn.User.ID,
			Type:    fn.User.Type,
			Summary: fn.User.Summary,
		},
	}
}

// --- PagerDutyClientInterface implementation ---

func (d *DevPagerDutyClient) CreateIncidentNoteWithContext(_ context.Context, id string, note pagerduty.IncidentNote) (*pagerduty.IncidentNote, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	log.Debug("DevPagerDutyClient.CreateIncidentNoteWithContext", "incident_id", id, "content_len", len(note.Content))

	newNote := pagerduty.IncidentNote{
		ID:        rand.ID("N"),
		Content:   note.Content,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		User:      note.User,
	}

	d.notes[id] = append(d.notes[id], newNote)
	return &newNote, nil
}

func (d *DevPagerDutyClient) GetCurrentUserWithContext(_ context.Context, _ pagerduty.GetCurrentUserOptions) (*pagerduty.User, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.currentUser, nil
}

func (d *DevPagerDutyClient) GetEscalationPolicyWithContext(_ context.Context, id string, _ *pagerduty.GetEscalationPolicyOptions) (*pagerduty.EscalationPolicy, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	policy, ok := d.escalationPolicies[id]
	if !ok {
		return nil, fmt.Errorf("DevPagerDutyClient: escalation policy %q not found", id)
	}
	return policy, nil
}

func (d *DevPagerDutyClient) GetIncidentWithContext(_ context.Context, id string) (*pagerduty.Incident, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	incident, ok := d.incidents[id]
	if !ok {
		return nil, fmt.Errorf("DevPagerDutyClient: incident %q not found", id)
	}

	// Return a copy to prevent external mutation
	copy := *incident
	return &copy, nil
}

func (d *DevPagerDutyClient) GetTeamWithContext(_ context.Context, id string) (*pagerduty.Team, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	team, ok := d.teams[id]
	if !ok {
		// Fallback: return a team with the ID as name (matches mock behavior)
		return &pagerduty.Team{
			APIObject: pagerduty.APIObject{ID: id},
			Name:      id,
		}, nil
	}
	return team, nil
}

func (d *DevPagerDutyClient) ListMembersWithContext(_ context.Context, _ string, _ pagerduty.ListTeamMembersOptions) (*pagerduty.ListTeamMembersResponse, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return &pagerduty.ListTeamMembersResponse{
		Members: d.teamMembers,
	}, nil
}

func (d *DevPagerDutyClient) GetUserWithContext(_ context.Context, id string, _ pagerduty.GetUserOptions) (*pagerduty.User, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	user, ok := d.users[id]
	if !ok {
		return nil, fmt.Errorf("DevPagerDutyClient: user %q not found", id)
	}
	return user, nil
}

func (d *DevPagerDutyClient) ListIncidentAlertsWithContext(_ context.Context, id string, _ pagerduty.ListIncidentAlertsOptions) (*pagerduty.ListAlertsResponse, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	alerts := d.alerts[id]
	if alerts == nil {
		alerts = []pagerduty.IncidentAlert{}
	}

	return &pagerduty.ListAlertsResponse{
		Alerts: alerts,
	}, nil
}

func (d *DevPagerDutyClient) ListIncidentsWithContext(_ context.Context, opts pagerduty.ListIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var incidents []pagerduty.Incident

	for _, incident := range d.incidents {
		// Filter by status if specified
		if len(opts.Statuses) > 0 {
			matched := false
			for _, status := range opts.Statuses {
				if incident.Status == status {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		incidentCopy := *incident
		incidents = append(incidents, incidentCopy)
	}

	// Sort by CreatedAt descending (newest first), matching PagerDuty API behavior
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].CreatedAt > incidents[j].CreatedAt
	})

	return &pagerduty.ListIncidentsResponse{
		Incidents: incidents,
	}, nil
}

func (d *DevPagerDutyClient) ListIncidentNotesWithContext(_ context.Context, id string) ([]pagerduty.IncidentNote, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	notes := d.notes[id]
	if notes == nil {
		notes = []pagerduty.IncidentNote{}
	}

	return notes, nil
}

func (d *DevPagerDutyClient) ListOnCallsWithContext(_ context.Context, _ pagerduty.ListOnCallOptions) (*pagerduty.ListOnCallsResponse, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return on-call entries for the fixture user covering the current time
	now := time.Now().UTC()
	return &pagerduty.ListOnCallsResponse{
		OnCalls: []pagerduty.OnCall{
			{
				User: pagerduty.User{
					APIObject: pagerduty.APIObject{
						ID:      d.currentUser.ID,
						Type:    "user_reference",
						Summary: d.currentUser.Name,
					},
					Name:  d.currentUser.Name,
					Email: d.currentUser.Email,
				},
				Start:           now.Add(-1 * time.Hour).Format(time.RFC3339),
				End:             now.Add(12 * time.Hour).Format(time.RFC3339),
				EscalationLevel: 1,
			},
		},
	}, nil
}

func (d *DevPagerDutyClient) ManageIncidentsWithContext(_ context.Context, _ string, opts []pagerduty.ManageIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var managed []pagerduty.Incident
	now := time.Now().UTC().Format(time.RFC3339)

	for _, opt := range opts {
		incident, ok := d.incidents[opt.ID]
		if !ok {
			return nil, fmt.Errorf("DevPagerDutyClient: incident %q not found", opt.ID)
		}

		// Handle status change (acknowledge)
		if opt.Status != "" {
			log.Debug("DevPagerDutyClient.ManageIncidents", "action", "status_change", "id", opt.ID, "new_status", opt.Status)
			incident.Status = opt.Status
			incident.LastStatusChangeAt = now

			// Add acknowledgement if acknowledging
			if opt.Status == "acknowledged" && len(opt.Assignments) > 0 {
				incident.Acknowledgements = append(incident.Acknowledgements, pagerduty.Acknowledgement{
					At: now,
					Acknowledger: pagerduty.APIObject{
						ID:   opt.Assignments[0].Assignee.ID,
						Type: opt.Assignments[0].Assignee.Type,
					},
				})
			}
		}

		// Handle escalation policy change (silence)
		if opt.EscalationPolicy != nil {
			log.Debug("DevPagerDutyClient.ManageIncidents", "action", "policy_change", "id", opt.ID, "new_policy", opt.EscalationPolicy.ID)
			incident.EscalationPolicy = pagerduty.APIObject{
				ID:   opt.EscalationPolicy.ID,
				Type: opt.EscalationPolicy.Type,
			}
			incident.LastStatusChangeAt = now
		}

		// Handle assignment change (reassign/re-escalate)
		if len(opt.Assignments) > 0 && opt.Status == "" && opt.EscalationPolicy == nil {
			log.Debug("DevPagerDutyClient.ManageIncidents", "action", "reassign", "id", opt.ID)
			var assignments []pagerduty.Assignment
			for _, a := range opt.Assignments {
				assignments = append(assignments, pagerduty.Assignment{
					At:       now,
					Assignee: a.Assignee,
				})
			}
			incident.Assignments = assignments
			incident.LastStatusChangeAt = now
		}

		// Handle escalation level change (un-acknowledge)
		if opt.EscalationLevel > 0 && opt.Status == "" && opt.EscalationPolicy == nil && len(opt.Assignments) == 0 {
			log.Debug("DevPagerDutyClient.ManageIncidents", "action", "escalation_level", "id", opt.ID, "level", opt.EscalationLevel)
			incident.LastStatusChangeAt = now
		}

		copy := *incident
		managed = append(managed, copy)
	}

	return &pagerduty.ListIncidentsResponse{
		Incidents: managed,
	}, nil
}

// NewDevConfig creates a pd.Config using the DevPagerDutyClient, bypassing live PD API calls.
// This is used when --dev mode is active.
func NewDevConfig(fixturesDir string) (*Config, error) {
	fixtures, err := LoadFixtures(fixturesDir)
	if err != nil {
		return nil, fmt.Errorf("NewDevConfig: %w", err)
	}

	client, err := NewDevPagerDutyClient(fixtures)
	if err != nil {
		return nil, fmt.Errorf("NewDevConfig: %w", err)
	}

	config := &Config{
		Client:             client,
		CurrentUser:        client.currentUser,
		EscalationPolicies: make(map[string]*pagerduty.EscalationPolicy),
	}

	// Set up teams
	for _, team := range client.teams {
		config.Teams = append(config.Teams, team)
	}

	// Set up team member IDs
	for _, member := range client.teamMembers {
		config.TeamsMemberIDs = append(config.TeamsMemberIDs, member.User.ID)
	}

	// Set up escalation policies using config keys
	for key, fp := range fixtures.Config.EscalationPolicies {
		config.EscalationPolicies[key] = &pagerduty.EscalationPolicy{
			APIObject: pagerduty.APIObject{
				ID:   fp.ID,
				Type: fp.Type,
			},
			Name: fp.Name,
		}
	}

	log.Info("DevPagerDutyClient initialized", "incidents", len(client.incidents), "user", client.currentUser.Email)
	return config, nil
}
