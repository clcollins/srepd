package tui

import (
	"context"
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
	priorAlertMaxIncidentsToScan = 200
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

type priorAlertsMsg struct {
	clusterID        string
	currentAlertName string
	data             *PriorAlertData
	err              error
}

func fetchPriorAlertsWeek(client pd.PagerDutyClient, serviceIDs []string, teamIDs []string, clusterID string, currentAlertName string, currentIncidentID string, weekSince time.Time, weekUntil time.Time) tea.Cmd {
	return func() tea.Msg {
		since := weekSince.Format(time.RFC3339)
		until := weekUntil.Format(time.RFC3339)
		weekLabel := weekSince.Format("Jan 02") + " - " + weekUntil.Format("Jan 02")

		log.Debug("priorAlerts: week starting", "cluster_id", clusterID, "week", weekLabel,
			"alert_name", currentAlertName, "service_ids", serviceIDs, "team_ids", teamIDs)

		ctx, cancel := context.WithTimeout(context.Background(), priorAlertTimeout)
		defer cancel()

		var sameAlert []PriorAlert
		var otherAlerts []PriorAlert
		seen := make(map[string]bool)

		full := func() bool {
			return len(sameAlert) >= priorAlertMaxMatches && len(otherAlerts) >= priorAlertMaxMatches
		}

		scan := func(label string, opts pagerduty.ListIncidentsOptions) {
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
					"page", pageCount, "incidents_in_page", len(resp.Incidents), "more", resp.More)

				for _, incident := range resp.Incidents {
					if incident.ID == currentIncidentID || seen[incident.ID] {
						continue
					}

					checkedCount++
					if checkedCount > priorAlertMaxIncidentsToScan {
						log.Debug("priorAlerts: scan cap reached", "scan", label, "week", weekLabel, "checked", checkedCount)
						return
					}

					alerts, err := pd.GetAlerts(client, incident.ID, pagerduty.ListIncidentAlertsOptions{})
					if err != nil {
						log.Debug("priorAlerts: GetAlerts failed", "scan", label, "incident", incident.ID, "error", err)
						continue
					}

					for _, a := range alerts {
						alertCluster := getDetailFieldFromAlert("cluster_id", a)
						if alertCluster == "" {
							normalized := alert.NormalizeAlert(a.Service.Summary, "", a)
							alertCluster = normalized.ClusterID
						}

						if alertCluster != clusterID {
							continue
						}

						seen[incident.ID] = true

						normalized := alert.NormalizeAlert(a.Service.Summary, incident.Title, a)
						alertName := normalized.AlertName
						if alertName == "" {
							alertName = getDetailFieldFromAlert("alert_name", a)
						}

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

						break
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
		}
	}
}

func dispatchPriorAlertWeeks(client pd.PagerDutyClient, serviceIDs []string, teamIDs []string, clusterID string, currentAlertName string, currentIncidentID string) []tea.Cmd {
	now := time.Now()
	lookbackStart := now.AddDate(0, 0, -priorAlertLookbackDays)

	var cmds []tea.Cmd
	weekEnd := now
	for weekEnd.After(lookbackStart) {
		weekStart := weekEnd.AddDate(0, 0, -priorAlertWeekDays)
		if weekStart.Before(lookbackStart) {
			weekStart = lookbackStart
		}
		cmds = append(cmds, fetchPriorAlertsWeek(
			client, serviceIDs, teamIDs,
			clusterID, currentAlertName, currentIncidentID,
			weekStart, weekEnd,
		))
		weekEnd = weekStart
	}

	log.Debug("priorAlerts: dispatched weeks", "cluster_id", clusterID,
		"alert_name", currentAlertName, "weeks", len(cmds))

	return cmds
}
