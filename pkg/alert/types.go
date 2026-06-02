package alert

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
)

// --- Helper functions for safe detail extraction ---

// getDetail safely extracts a string field from the alert body details map.
// Returns "" if body, details, or the field is missing or not a string.
func getDetail(field string, alert pagerduty.IncidentAlert) string {
	if alert.Body == nil {
		return ""
	}
	detailsRaw, ok := alert.Body["details"]
	if !ok || detailsRaw == nil {
		return ""
	}
	details, ok := detailsRaw.(map[string]interface{})
	if !ok {
		return ""
	}
	fieldRaw, ok := details[field]
	if !ok || fieldRaw == nil {
		return ""
	}
	fieldStr, ok := fieldRaw.(string)
	if !ok {
		return ""
	}
	return fieldStr
}

// parseFiringCount extracts the num_firing field as an integer.
func parseFiringCount(alert pagerduty.IncidentAlert) int {
	s := getDetail("num_firing", alert)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// --- Title parsing regex patterns ---

// osdHiveTitlePattern matches: AlertName SEVERITY (N)
var osdHiveTitlePattern = regexp.MustCompile(`^(?P<alert_name>[A-Za-z][A-Za-z0-9-]+)\s+(?P<severity>CRITICAL|WARNING|Critical|Warning)\s+\((?P<count>\d+)\)$`)

// appSRETitlePattern matches: [FIRING:N] AlertName ...
var appSRETitlePattern = regexp.MustCompile(`^\[FIRING:(\d+)\]\s+(\S+)`)

// rhobsHCPTitlePattern matches: [HCP] [RHOBS] (Severity) AlertName for HCP: cluster-uuid
var rhobsHCPTitlePattern = regexp.MustCompile(`^\[HCP\]\s+\[RHOBS\]\s+\(([^)]+)\)\s+(.+?)\s+for\s+HCP:\s*(.+)$`)

// rhobsInfraTitlePattern matches: [HCP] [RHOBS Infra] (Severity) AlertName
var rhobsInfraTitlePattern = regexp.MustCompile(`^\[HCP\]\s+\[RHOBS Infra\]\s+\(([^)]+)\)\s+(.+)$`)

// ceeTitlePattern matches: OHSS-NNNNN: description
var ceeTitlePattern = regexp.MustCompile(`^(OHSS-\d+):\s*(.+)$`)

// sopURLPattern matches SOP: <url> within annotation message text
var sopURLPattern = regexp.MustCompile(`SOP:\s*(https?://\S+)`)

// --- Per-type parsers ---

// parseOSDHive parses OSD Hive cluster alerts.
// Source: per-cluster PD services named osd-{cluster-url}-hive-cluster.
func parseOSDHive(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	// Extract from structured details (preferred)
	n.AlertName = getDetail("alert_name", alert)
	n.ClusterID = getDetail("cluster_id", alert)
	n.SOPLink = getDetail("link", alert)
	n.OCMLink = getDetail("ocm_link", alert)
	n.FiringCount = parseFiringCount(alert)

	// Parse severity from title (strip bracket prefixes first)
	cleaned, _ := StripBracketPrefixes(title)
	matches := osdHiveTitlePattern.FindStringSubmatch(cleaned)
	if matches != nil {
		n.Severity = strings.ToLower(matches[2])
		// Fall back to title for alert name if not in details
		if n.AlertName == "" {
			n.AlertName = matches[1]
		}
	}

	// Extract namespace and description from firing field if available
	firingText := getDetail("firing", alert)
	if firingText != "" {
		firingFields := ParseFiring(firingText)
		if ns, ok := firingFields["namespace"]; ok {
			n.Namespace = ns
		}
		if desc, ok := firingFields["description"]; ok {
			n.Description = desc
		} else if msg, ok := firingFields["message"]; ok {
			n.Description = msg
		}
	}
}

// parseAppSRE parses App-SRE Alertmanager alerts.
// This is the worst type for machine parsing - most data must be extracted from firing text.
func parseAppSRE(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	n.ClusterID = getDetail("cluster_id", alert)
	n.FiringCount = parseFiringCount(alert)

	// Extract alert name from title: first token after [FIRING:N]
	// Try raw title first since [FIRING:N] is part of the format, not an SRE tag.
	// If SRE brackets were prepended (e.g. "[SL Sent] [FIRING:1] ..."), strip them
	// progressively until the pattern matches.
	matches := appSRETitlePattern.FindStringSubmatch(title)
	if matches == nil {
		// SRE-added brackets may be in front; strip them and try again
		cleaned, _ := StripBracketPrefixes(title)
		matches = appSRETitlePattern.FindStringSubmatch(cleaned)
	}
	if matches != nil {
		n.AlertName = matches[2]
	}

	// Parse the firing text for structured data
	firingText := getDetail("firing", alert)
	if firingText != "" {
		firingFields := ParseFiring(firingText)

		// Severity from firing labels
		if sev, ok := firingFields["severity"]; ok {
			n.Severity = strings.ToLower(sev)
		}

		// Condition and reason
		if cond, ok := firingFields["condition"]; ok {
			n.Condition = cond
		}
		if reason, ok := firingFields["reason"]; ok {
			n.Reason = reason
		}

		// Namespace
		if ns, ok := firingFields["namespace"]; ok {
			n.Namespace = ns
		}

		// Cluster name from cluster_deployment
		if cd, ok := firingFields["cluster_deployment"]; ok {
			n.ClusterName = cd
		}

		// SOP link: check runbook annotation first, then SOP: in message
		if runbook, ok := firingFields["runbook"]; ok && runbook != "" {
			n.SOPLink = runbook
		} else if msg, ok := firingFields["message"]; ok {
			sopMatches := sopURLPattern.FindStringSubmatch(msg)
			if sopMatches != nil {
				n.SOPLink = sopMatches[1]
			}
		}

		// Dashboard from annotations
		if dash, ok := firingFields["dashboard"]; ok {
			n.DashboardLink = dash
		}

		// Region from firing labels
		if region, ok := firingFields["region"]; ok {
			n.Region = region
		}
	}
}

// parseRHOBSHCP parses RHOBS HCP (Hosted Control Planes) alerts.
// This is the best-structured type - all critical fields are top-level keys.
func parseRHOBSHCP(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	// All structured from details
	n.AlertName = getDetail("alert_name", alert)
	n.ClusterID = getDetail("cluster_id", alert)
	n.SOPLink = getDetail("link", alert)
	n.OCMLink = getDetail("ocm_link", alert)
	n.DashboardLink = getDetail("dashboard", alert)
	n.FiringCount = parseFiringCount(alert)

	// Parse severity from title - try raw title first since [HCP] [RHOBS] are format markers.
	// If SRE brackets were prepended, strip them and try again.
	matches := rhobsHCPTitlePattern.FindStringSubmatch(title)
	if matches == nil {
		cleaned, _ := StripBracketPrefixes(title)
		matches = rhobsHCPTitlePattern.FindStringSubmatch(cleaned)
	}
	if matches != nil {
		n.Severity = strings.ToLower(matches[1])
		// Fall back to title for alert name if not in details
		if n.AlertName == "" {
			n.AlertName = matches[2]
		}
		// Fall back to title for cluster_id if not in details
		if n.ClusterID == "" {
			n.ClusterID = strings.TrimSpace(matches[3])
		}
	}

	// Extract region from service name: rhobs-hcp-{env}-{severity?}-{region}
	n.Region = extractRHOBSRegion(n.ServiceName)

	// Extract namespace and description from firing if available
	firingText := getDetail("firing", alert)
	if firingText != "" {
		firingFields := ParseFiring(firingText)
		if ns, ok := firingFields["namespace"]; ok {
			n.Namespace = ns
		}
		if desc, ok := firingFields["description"]; ok {
			n.Description = desc
		}
	}
}

// parseRHOBSInfra parses RHOBS infrastructure alerts.
// Similar to RHOBS HCP but lacks dashboard and ocm_link.
func parseRHOBSInfra(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	n.AlertName = getDetail("alert_name", alert)
	n.ClusterID = getDetail("cluster_id", alert)
	n.SOPLink = getDetail("link", alert)
	n.FiringCount = parseFiringCount(alert)

	// Parse severity from title - try raw title first since [HCP] [RHOBS Infra] are format markers.
	// If SRE brackets were prepended, strip them and try again.
	matches := rhobsInfraTitlePattern.FindStringSubmatch(title)
	if matches == nil {
		cleaned, _ := StripBracketPrefixes(title)
		matches = rhobsInfraTitlePattern.FindStringSubmatch(cleaned)
	}
	if matches != nil {
		n.Severity = strings.ToLower(matches[1])
		if n.AlertName == "" {
			n.AlertName = matches[2]
		}
	}

	// Extract region from service name: rhobs-infra-{env}-{region}
	n.Region = extractRHOBSRegion(n.ServiceName)
}

// parseDeadMansSnitch parses Dead Man's Snitch alerts.
// Fires when a cluster stops sending heartbeat pings.
func parseDeadMansSnitch(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	n.AlertName = "ClusterGoneMissing"
	n.Severity = "critical" // Always critical
	n.ClusterID = getDetail("cluster_id", alert)
	n.ClusterName = getDetail("name", alert)

	// Extract runbook from notes field
	notes := getDetail("notes", alert)
	if notes != "" {
		for _, line := range strings.Split(notes, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "runbook:") {
				n.SOPLink = strings.TrimSpace(strings.TrimPrefix(line, "runbook:"))
				break
			}
		}
	}
}

// parseCEE parses CEE-to-SREP escalation incidents.
// These are manually triggered by the CEE escalation process and have zero alerts.
func parseCEE(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	n.Severity = "critical" // Sev1 escalation
	n.FiringCount = 0       // No alerts for CEE

	// Extract OHSS ticket from title
	matches := ceeTitlePattern.FindStringSubmatch(title)
	if matches != nil {
		n.AlertName = matches[1] // OHSS-NNNNN
	}
}

// parseCAD parses CAD E2E testing alerts.
// These are synthetic test alerts and should be filtered from actionable views.
func parseCAD(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	n.AlertName = title // Use the full title as the alert name
}

// parseUnknown handles unrecognized alert types.
// Extracts what it can and leaves type-specific fields empty.
// This handler MUST NOT crash on any input.
func parseUnknown(n *NormalizedAlert, title string, alert pagerduty.IncidentAlert) {
	// Try to extract common fields from details if available
	n.AlertName = getDetail("alert_name", alert)
	n.ClusterID = getDetail("cluster_id", alert)
	n.SOPLink = getDetail("link", alert)
	n.OCMLink = getDetail("ocm_link", alert)
	n.FiringCount = parseFiringCount(alert)
}

// extractRHOBSRegion extracts the AWS region from an RHOBS service name.
// Service names follow the pattern: rhobs-{type}-{env}-{severity?}-{region}
// Examples:
//   - rhobs-hcp-prod-critical-us-east-1 -> us-east-1
//   - rhobs-hcp-stage-us-west-2 -> us-west-2
//   - rhobs-infra-stage-us-east-1 -> us-east-1
//   - rhobs-infra-warning-ap-southeast-2 -> ap-southeast-2
func extractRHOBSRegion(serviceName string) string {
	// Known severity segments that appear between env and region
	severities := []string{"-critical-", "-warning-", "-soaking-"}
	// Known environment segments
	envs := []string{"-prod-", "-stage-", "-int-"}

	for _, sev := range severities {
		idx := strings.Index(serviceName, sev)
		if idx >= 0 {
			return serviceName[idx+len(sev):]
		}
	}

	// No severity segment - region comes directly after env
	for _, env := range envs {
		idx := strings.Index(serviceName, env)
		if idx >= 0 {
			return serviceName[idx+len(env):]
		}
	}

	return ""
}
