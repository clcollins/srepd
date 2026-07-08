package tui

import (
	"context"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/alert"
	"github.com/clcollins/srepd/pkg/pd"
)

const (
	priorAlertMaxMatches         = 15
	priorAlertLookbackDays       = 90
	priorAlertWeekDays           = 7
	priorAlertMaxIncidentsToScan = 500
	priorAlertTimeout            = 5 * time.Minute
)

type PriorAlert struct {
	Date        string
	AlertName   string
	IncidentURL string
	IncidentID  string
}

type PriorAlertData struct {
	SameAlert   []PriorAlert
	OtherAlerts []PriorAlert
}

type priorAlertWeek struct {
	since time.Time
	until time.Time
}

type priorAlertsMsg struct {
	clusterID        string
	currentAlertName string
	data             *PriorAlertData
	err              error
	nextWeeks        []priorAlertWeek
	client           pd.PagerDutyClient
	serviceIDs       []string
	teamIDs          []string
	incidentID       string
}

func buildPriorAlertWeeks() []priorAlertWeek {
	now := time.Now()
	lookbackStart := now.AddDate(0, 0, -priorAlertLookbackDays)

	var weeks []priorAlertWeek
	weekEnd := now
	for weekEnd.After(lookbackStart) {
		weekStart := weekEnd.AddDate(0, 0, -priorAlertWeekDays)
		if weekStart.Before(lookbackStart) {
			weekStart = lookbackStart
		}
		weeks = append(weeks, priorAlertWeek{since: weekStart, until: weekEnd})
		weekEnd = weekStart
	}
	return weeks
}

// matchIncidentToCluster checks if an incident is related to the target cluster
// using a three-tier strategy that avoids separate GetAlerts API calls:
//
// matchIncidentToCluster checks if an incident is related to the target cluster.
// For service-scoped queries (1:1 hive services), all results already match —
// call this only for the team-wide fallback scan.
//
// Tier 1 (title match): rhobs_hcp titles contain the cluster UUID
// ("for HCP: <uuid>"). Check with strings.Contains.
//
// Tier 2 (log entry): Use FirstTriggerLogEntry.Channel.Raw to extract
// cluster_id from the inline alert payload (requires include[]=first_trigger_log_entries).
func matchIncidentToCluster(incident pagerduty.Incident, clusterID string) bool {
	if strings.Contains(incident.Title, clusterID) {
		return true
	}

	return extractClusterFromLogEntry(incident) == clusterID
}

func extractClusterFromLogEntry(incident pagerduty.Incident) string {
	raw := incident.FirstTriggerLogEntry.Channel.Raw
	if raw == nil {
		return ""
	}

	if details, ok := raw["details"].(map[string]interface{}); ok {
		if cid, ok := details["cluster_id"].(string); ok && cid != "" {
			return cid
		}
	}

	if details, ok := raw["custom_details"].(map[string]interface{}); ok {
		if cid, ok := details["cluster_id"].(string); ok && cid != "" {
			return cid
		}
	}

	return ""
}

func extractAlertNameFromIncident(incident pagerduty.Incident) string {
	normalized := alert.NormalizeAlert(incident.Service.Summary, incident.Title, pagerduty.IncidentAlert{})
	if normalized.AlertName != "" {
		return normalized.AlertName
	}

	raw := incident.FirstTriggerLogEntry.Channel.Raw
	if raw == nil {
		return ""
	}
	for _, key := range []string{"details", "custom_details"} {
		if details, ok := raw[key].(map[string]interface{}); ok {
			if name, ok := details["alert_name"].(string); ok && name != "" {
				return name
			}
		}
	}

	return ""
}

func fetchPriorAlertsWeek(client pd.PagerDutyClient, serviceIDs []string, teamIDs []string, clusterID string, currentAlertName string, currentIncidentID string, week priorAlertWeek, remainingWeeks []priorAlertWeek) tea.Cmd {
	return func() tea.Msg {
		since := week.since.Format(time.RFC3339)
		until := week.until.Format(time.RFC3339)
		weekLabel := week.since.Format("Jan 02") + " - " + week.until.Format("Jan 02")

		log.Debug("priorAlerts: week starting", "cluster_id", clusterID, "week", weekLabel,
			"remaining", len(remainingWeeks), "alert_name", currentAlertName)

		ctx, cancel := context.WithTimeout(context.Background(), priorAlertTimeout)
		defer cancel()

		var sameAlert []PriorAlert
		var otherAlerts []PriorAlert
		seen := make(map[string]bool)

		full := func() bool {
			return len(sameAlert) >= priorAlertMaxMatches && len(otherAlerts) >= priorAlertMaxMatches
		}

		scan := func(label string, opts pagerduty.ListIncidentsOptions) {
			opts.Includes = []string{"first_trigger_log_entries"}
			checkedCount := 0
			pageCount := 0
			for {
				pageCount++
				resp, err := client.ListIncidentsWithContext(ctx, opts)
				if err != nil {
					log.Debug("priorAlerts: ListIncidents failed", "scan", label, "week", weekLabel, "error", err)
					return
				}

				log.Debug("priorAlerts: page fetched", "scan", label, "week", weekLabel,
					"page", pageCount, "incidents", len(resp.Incidents), "more", resp.More)

				for _, incident := range resp.Incidents {
					if incident.ID == currentIncidentID || seen[incident.ID] {
						continue
					}
					seen[incident.ID] = true

					checkedCount++
					if checkedCount > priorAlertMaxIncidentsToScan {
						log.Debug("priorAlerts: scan cap reached", "scan", label, "week", weekLabel, "checked", checkedCount)
						return
					}

					if !matchIncidentToCluster(incident, clusterID) {
						continue
					}

					alertName := extractAlertNameFromIncident(incident)

					entry := PriorAlert{
						Date:        incident.CreatedAt,
						AlertName:   alertName,
						IncidentURL: incident.HTMLURL,
						IncidentID:  incident.ID,
					}

					if alertName == currentAlertName && len(sameAlert) < priorAlertMaxMatches {
						sameAlert = append(sameAlert, entry)
						log.Debug("priorAlerts: match (same)", "scan", label, "incident", incident.ID,
							"alert", alertName, "date", incident.CreatedAt)
					} else if alertName != currentAlertName && len(otherAlerts) < priorAlertMaxMatches {
						otherAlerts = append(otherAlerts, entry)
						log.Debug("priorAlerts: match (other)", "scan", label, "incident", incident.ID,
							"alert", alertName, "date", incident.CreatedAt)
					}

					if full() {
						log.Debug("priorAlerts: both tables full", "scan", label, "week", weekLabel)
						return
					}
				}

				if checkedCount > priorAlertMaxIncidentsToScan || full() {
					return
				}

				opts.Offset += opts.Limit
				if !resp.More {
					return
				}
			}
		}

		if len(serviceIDs) > 0 {
			scan("service", pagerduty.ListIncidentsOptions{
				ServiceIDs: serviceIDs,
				Since:      since,
				Until:      until,
				Statuses:   []string{"triggered", "acknowledged", "resolved"},
				Limit:      100,
				SortBy:     "created_at:desc",
			})
		}

		if !full() && len(teamIDs) > 0 {
			scan("team", pagerduty.ListIncidentsOptions{
				TeamIDs:  teamIDs,
				Since:    since,
				Until:    until,
				Statuses: []string{"triggered", "acknowledged", "resolved"},
				Limit:    100,
				SortBy:   "created_at:desc",
			})
		}

		log.Debug("priorAlerts: week done", "cluster_id", clusterID, "week", weekLabel,
			"same_count", len(sameAlert), "other_count", len(otherAlerts))

		return priorAlertsMsg{
			clusterID:        clusterID,
			currentAlertName: currentAlertName,
			data: &PriorAlertData{
				SameAlert:   sameAlert,
				OtherAlerts: otherAlerts,
			},
			nextWeeks:  remainingWeeks,
			client:     client,
			serviceIDs: serviceIDs,
			teamIDs:    teamIDs,
			incidentID: currentIncidentID,
		}
	}
}
