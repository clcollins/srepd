package alert

import (
	"strings"

	"github.com/PagerDuty/go-pagerduty"
)

// NormalizedAlert is the canonical representation of a PagerDuty incident alert,
// normalized across all alert types (osd_hive, appsre, rhobs_hcp, etc.).
// Fields marked "required" are always populated; "optional" fields are populated
// when available from the source data.
type NormalizedAlert struct {
	// Required fields
	AlertType   string // "osd_hive", "appsre", "rhobs_hcp", "rhobs_infra", "deadmanssnitch", "cee_escalation", "cad", "unknown"
	AlertName   string // Normalized alert name (e.g., "ClusterOperatorDown")
	ClusterID   string // Cluster UUID or subscription ID (empty string if not applicable)
	Severity    string // Normalized to lowercase: "critical", "warning", "high", "soaking"
	Title       string // Original PD incident title (preserved for display)
	Status      string // PD alert status: "triggered", "acknowledged", "resolved"
	CreatedAt   string // ISO 8601 timestamp
	IncidentID  string // PD incident ID
	ServiceName string // PD service summary

	// Optional fields - populated when available
	SOPLink       string   // SOP/runbook URL
	OCMLink       string   // OCM console link
	DashboardLink string   // Grafana/monitoring dashboard URL
	ClusterName   string   // Human-readable cluster name or URL
	Region        string   // AWS region (e.g., "us-east-1")
	Namespace     string   // Kubernetes namespace
	Condition     string   // Failure condition (e.g., "ProvisionFailed")
	Reason        string   // Failure reason (e.g., "BootstrapFailed")
	Tags          []string // SRE-added title tags: ["SL Sent", "OHSS-54318", ...]
	FiringCount   int      // Number of firing alerts
}

// IdentifyType determines the alert type from the PagerDuty service name pattern.
// Match order is specific-to-general to avoid ambiguity.
func IdentifyType(serviceSummary string) string {
	switch {
	case strings.Contains(serviceSummary, "-hive-cluster"):
		return "osd_hive"
	case strings.HasPrefix(serviceSummary, "app-sre-alertmanager"):
		return "appsre"
	case strings.HasPrefix(serviceSummary, "rhobs-hcp-"):
		return "rhobs_hcp"
	case strings.HasPrefix(serviceSummary, "rhobs-infra-"):
		return "rhobs_infra"
	case strings.HasSuffix(serviceSummary, "-deadmanssnitch"):
		return "deadmanssnitch"
	case strings.HasPrefix(serviceSummary, "cee-"):
		return "cee_escalation"
	case strings.HasPrefix(serviceSummary, "CAD "):
		return "cad"
	default:
		return "unknown"
	}
}

// NormalizeAlert normalizes a PagerDuty alert into the canonical NormalizedAlert
// structure. It dispatches to per-type parsers based on the service name.
func NormalizeAlert(serviceSummary string, title string, alert pagerduty.IncidentAlert) NormalizedAlert {
	alertType := IdentifyType(serviceSummary)

	// Build base normalized alert with common fields
	normalized := NormalizedAlert{
		AlertType:   alertType,
		Title:       title,
		Status:      alert.Status,
		CreatedAt:   alert.CreatedAt,
		IncidentID:  alert.Incident.ID,
		ServiceName: serviceSummary,
	}

	// Strip bracket prefixes from title before type-specific parsing
	_, tags := StripBracketPrefixes(title)
	if len(tags) > 0 {
		normalized.Tags = tags
	}

	switch alertType {
	case "osd_hive":
		parseOSDHive(&normalized, title, alert)
	case "appsre":
		parseAppSRE(&normalized, title, alert)
	case "rhobs_hcp":
		parseRHOBSHCP(&normalized, title, alert)
	case "rhobs_infra":
		parseRHOBSInfra(&normalized, title, alert)
	case "deadmanssnitch":
		parseDeadMansSnitch(&normalized, title, alert)
	case "cee_escalation":
		parseCEE(&normalized, title, alert)
	case "cad":
		parseCAD(&normalized, title, alert)
	default:
		parseUnknown(&normalized, title, alert)
	}

	return normalized
}
