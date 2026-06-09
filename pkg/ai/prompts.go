package ai

import (
	"fmt"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/alert"
)

// BuildSystemPrompt constructs a system prompt for SRE incident analysis
// from a PagerDuty incident and its normalized alerts.
func BuildSystemPrompt(incident *pagerduty.Incident, alerts []alert.NormalizedAlert) string {
	if incident == nil {
		return "You are an SRE assistant. Provide concise analysis. Do not suggest destructive commands."
	}

	alertNames := make([]string, 0, len(alerts))
	for _, a := range alerts {
		if a.AlertName != "" {
			alertNames = append(alertNames, a.AlertName)
		}
	}

	clusterID := extractClusterID(alerts)

	return fmt.Sprintf(`You are an SRE assistant analyzing a PagerDuty incident.

Incident: %s (%s)
Service: %s
Status: %s, Urgency: %s
Cluster: %s
Alert count: %d
Alert names: %s

Provide concise analysis. Do not suggest destructive commands.`,
		incident.Title, incident.ID,
		incident.Service.Summary,
		incident.Status, incident.Urgency,
		clusterID,
		len(alerts),
		strings.Join(alertNames, ", "),
	)
}

func extractClusterID(alerts []alert.NormalizedAlert) string {
	for _, a := range alerts {
		if a.ClusterID != "" {
			return a.ClusterID
		}
	}
	return ""
}
