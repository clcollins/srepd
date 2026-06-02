package alert

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
)

// --- TestIdentifyType ---

func TestIdentifyType(t *testing.T) {
	tests := []struct {
		name     string
		service  string
		expected string
	}{
		{
			name:     "osd_hive cluster service",
			service:  "osd-mycluster.abc1.p1.openshiftapps.com-hive-cluster",
			expected: "osd_hive",
		},
		{
			name:     "appsre alertmanager prod",
			service:  "app-sre-alertmanager",
			expected: "appsre",
		},
		{
			name:     "appsre alertmanager stage",
			service:  "app-sre-alertmanager-stage",
			expected: "appsre",
		},
		{
			name:     "rhobs_hcp prod critical us-east-1",
			service:  "rhobs-hcp-prod-critical-us-east-1",
			expected: "rhobs_hcp",
		},
		{
			name:     "rhobs_hcp stage us-west-2",
			service:  "rhobs-hcp-stage-us-west-2",
			expected: "rhobs_hcp",
		},
		{
			name:     "rhobs_infra stage us-east-1",
			service:  "rhobs-infra-stage-us-east-1",
			expected: "rhobs_infra",
		},
		{
			name:     "rhobs_infra warning region",
			service:  "rhobs-infra-warning-ap-southeast-2",
			expected: "rhobs_infra",
		},
		{
			name:     "deadmanssnitch prod",
			service:  "prod-deadmanssnitch",
			expected: "deadmanssnitch",
		},
		{
			name:     "deadmanssnitch stage",
			service:  "stage-deadmanssnitch",
			expected: "deadmanssnitch",
		},
		{
			name:     "cee_escalation",
			service:  "cee-srep-sev1-escalation-pager",
			expected: "cee_escalation",
		},
		{
			name:     "cad stage",
			service:  "CAD Stage",
			expected: "cad",
		},
		{
			name:     "cad testing",
			service:  "CAD Testing",
			expected: "cad",
		},
		{
			name:     "unknown service",
			service:  "some-other-service",
			expected: "unknown",
		},
		{
			name:     "empty service",
			service:  "",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IdentifyType(tt.service)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- TestNormalizeAlert per type ---

func makeAlert(details map[string]interface{}) pagerduty.IncidentAlert {
	return pagerduty.IncidentAlert{
		APIObject: pagerduty.APIObject{
			ID:      "ALERT001",
			HTMLURL: "https://pagerduty.com/alerts/ALERT001",
		},
		Status:    "triggered",
		CreatedAt: "2026-05-29T10:00:00Z",
		Service: pagerduty.APIObject{
			Summary: "test-service",
		},
		Incident: pagerduty.APIReference{
			ID: "INC001",
		},
		Body: map[string]interface{}{
			"details": details,
		},
	}
}

func TestNormalizeAlert_OSDHive_FullFields(t *testing.T) {
	details := map[string]interface{}{
		"alert_name":   "MachineHealthCheckUnterminatedShortCircuitSRE",
		"cluster_id":   "e7c5363a-b69b-47bf-98ff-edf99fc3ea25",
		"link":         "https://github.com/openshift/ops-sop/blob/master/v4/alerts/MachineHealthCheckUnterminatedShortCircuitSRE.md",
		"ocm_link":     "https://console.redhat.com/openshift/details/e7c5363a-b69b-47bf-98ff-edf99fc3ea25",
		"num_firing":   "1",
		"num_resolved": "0",
		"firing":       "Labels:\n - alertname = MachineHealthCheckUnterminatedShortCircuitSRE\n - namespace = openshift-machine-api\nAnnotations:\n - description = test\nSource: https://example.com",
		"resolved":     "",
	}

	alert := makeAlert(details)
	alert.Service.Summary = "osd-mycluster.abc1.p1.openshiftapps.com-hive-cluster"

	serviceSummary := "osd-mycluster.abc1.p1.openshiftapps.com-hive-cluster"
	title := "MachineHealthCheckUnterminatedShortCircuitSRE CRITICAL (1)"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "osd_hive", result.AlertType)
	assert.Equal(t, "MachineHealthCheckUnterminatedShortCircuitSRE", result.AlertName)
	assert.Equal(t, "e7c5363a-b69b-47bf-98ff-edf99fc3ea25", result.ClusterID)
	assert.Equal(t, "critical", result.Severity)
	assert.Equal(t, title, result.Title)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/MachineHealthCheckUnterminatedShortCircuitSRE.md", result.SOPLink)
	assert.Equal(t, "https://console.redhat.com/openshift/details/e7c5363a-b69b-47bf-98ff-edf99fc3ea25", result.OCMLink)
	assert.Equal(t, 1, result.FiringCount)
}

func TestNormalizeAlert_OSDHive_WithBracketPrefix(t *testing.T) {
	details := map[string]interface{}{
		"alert_name": "ClusterOperatorDown",
		"cluster_id": "abc-123",
		"link":       "https://example.com/sop",
		"ocm_link":   "https://console.redhat.com/openshift/details/abc-123",
		"num_firing": "2",
	}

	alert := makeAlert(details)

	serviceSummary := "osd-mycluster.abc1.p1.openshiftapps.com-hive-cluster"
	title := "[SL Sent] ClusterOperatorDown CRITICAL (2)"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "osd_hive", result.AlertType)
	assert.Equal(t, "ClusterOperatorDown", result.AlertName)
	assert.Equal(t, "critical", result.Severity)
	assert.Equal(t, []string{"SL Sent"}, result.Tags)
	assert.Equal(t, title, result.Title) // Original title preserved
}

func TestNormalizeAlert_AppSRE_ExtractsFromFiring(t *testing.T) {
	firing := `Labels:
 - alertname = ClusterProvisioningDelay - production
 - cluster = hivep04ew2
 - cluster_deployment = edf-mas
 - condition = ProvisionFailed
 - reason = UnknownError
 - severity = high
 - platform = gcp
Annotations:
 - message = cluster edf-mas has been in a failed state. SOP: https://github.com/openshift/ops-sop/blob/master/v4/alerts/ClusterProvisioningDelay.md
 - runbook = https://github.com/openshift/ops-sop/blob/master/v4/alerts/ClusterProvisioningDelay.md
Source: https://prometheus.example.com`

	details := map[string]interface{}{
		"cluster_id":   "2qjmjvgq7808oqt2hrg190r76h7v4etu",
		"firing":       firing,
		"num_firing":   "1",
		"num_resolved": "0",
		"resolved":     "",
	}

	alert := makeAlert(details)

	serviceSummary := "app-sre-alertmanager"
	title := "[FIRING:1] ClusterProvisioningDelay - production hivep04ew2 uhc-production-2qjmjvgq7808oqt2hrg190r76h7v4etu (stuff here)"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "appsre", result.AlertType)
	assert.Equal(t, "ClusterProvisioningDelay", result.AlertName)
	assert.Equal(t, "2qjmjvgq7808oqt2hrg190r76h7v4etu", result.ClusterID)
	assert.Equal(t, "high", result.Severity)
	assert.Equal(t, "ProvisionFailed", result.Condition)
	assert.Equal(t, "UnknownError", result.Reason)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/ClusterProvisioningDelay.md", result.SOPLink)
	assert.Equal(t, 1, result.FiringCount)
}

func TestNormalizeAlert_AppSRE_SOPFromMessage(t *testing.T) {
	// Test that SOP is extracted from message annotation when runbook is absent
	firing := `Labels:
 - alertname = SomeAlert
 - severity = critical
Annotations:
 - message = Something happened. SOP: https://github.com/openshift/ops-sop/blob/master/v4/alerts/SomeAlert.md
Source: https://prometheus.example.com`

	details := map[string]interface{}{
		"cluster_id": "testcluster123",
		"firing":     firing,
		"num_firing": "1",
	}

	alert := makeAlert(details)

	serviceSummary := "app-sre-alertmanager"
	title := "[FIRING:1] SomeAlert - production (stuff)"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "appsre", result.AlertType)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/SomeAlert.md", result.SOPLink)
}

func TestNormalizeAlert_RHOBSHCP_AllStructured(t *testing.T) {
	details := map[string]interface{}{
		"alert_name": "ClusterOperatorDown",
		"cluster_id": "a4ba96fe-ac69-4573-a78b-17d38eeaab99",
		"dashboard":  "https://grafana.app-sre.devshift.net/d/cf6ntunq7rb40c/rhobs",
		"link":       "https://github.com/openshift/ops-sop/blob/master/hypershift/alerts/ClusterOperatorDown.md",
		"ocm_link":   "https://console.redhat.com/openshift/details/a4ba96fe-ac69-4573-a78b-17d38eeaab99",
		"firing":     "\n\n  - alertname: ClusterOperatorDown\n    cluster_id: a4ba96fe-ac69-4573-a78b-17d38eeaab99\n    namespace: ocm-production-abc\n    description: The ingress operator is unavailable\n\n",
		"num_firing": "1",
	}

	alert := makeAlert(details)

	serviceSummary := "rhobs-hcp-prod-critical-us-east-1"
	title := "[HCP] [RHOBS] (Critical) ClusterOperatorDown for HCP: a4ba96fe-ac69-4573-a78b-17d38eeaab99"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "rhobs_hcp", result.AlertType)
	assert.Equal(t, "ClusterOperatorDown", result.AlertName)
	assert.Equal(t, "a4ba96fe-ac69-4573-a78b-17d38eeaab99", result.ClusterID)
	assert.Equal(t, "critical", result.Severity)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/hypershift/alerts/ClusterOperatorDown.md", result.SOPLink)
	assert.Equal(t, "https://console.redhat.com/openshift/details/a4ba96fe-ac69-4573-a78b-17d38eeaab99", result.OCMLink)
	assert.Equal(t, "https://grafana.app-sre.devshift.net/d/cf6ntunq7rb40c/rhobs", result.DashboardLink)
	assert.Equal(t, "us-east-1", result.Region)
}

func TestNormalizeAlert_RHOBSHCP_WithSoakingSeverity(t *testing.T) {
	details := map[string]interface{}{
		"alert_name": "SomeAlert",
		"cluster_id": "abc-123",
		"num_firing": "1",
	}

	alert := makeAlert(details)

	serviceSummary := "rhobs-hcp-prod-critical-ap-southeast-2"
	title := "[HCP] [RHOBS] (Soaking) SomeAlert for HCP: abc-123"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "soaking", result.Severity)
	assert.Equal(t, "ap-southeast-2", result.Region)
}

func TestNormalizeAlert_RHOBSInfra_AllStructured(t *testing.T) {
	details := map[string]interface{}{
		"alert_name": "RMOAPIRequestErrorRate",
		"link":       "https://github.com/openshift/ops-sop/blob/master/hypershift/alerts/rhobs-synthetics/RMOAPIRequestErrorRate.md",
		"firing":     "\n\n  - alertname: RMOAPIRequestErrorRate\n    cluster_id: \n    namespace: openshift-route-monitor-operator\n    description: Route-monitor-operator on MC hs-mc-n1q4m3hb0 (ap-southeast-2) has >50% API request error rate\n\n",
		"num_firing": "1",
	}

	alert := makeAlert(details)

	serviceSummary := "rhobs-infra-stage-us-east-1"
	title := "[HCP] [RHOBS Infra] (Critical) RMOAPIRequestErrorRate"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "rhobs_infra", result.AlertType)
	assert.Equal(t, "RMOAPIRequestErrorRate", result.AlertName)
	assert.Equal(t, "critical", result.Severity)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/hypershift/alerts/rhobs-synthetics/RMOAPIRequestErrorRate.md", result.SOPLink)
	assert.Equal(t, "us-east-1", result.Region)
	// cluster_id is empty for infra alerts
	assert.Equal(t, "", result.ClusterID)
}

func TestNormalizeAlert_DeadMansSnitch_RunbookFromNotes(t *testing.T) {
	details := map[string]interface{}{
		"cluster_id":            "06f058a0-35fa-42c5-83a1-f39184fee819",
		"last healthy check-in": "2026-05-22T19:10:12.339Z",
		"name":                  "jfrogdevrrrooo.5uqt.p1.openshiftapps.com",
		"notes":                 "cluster_id: 06f058a0-35fa-42c5-83a1-f39184fee819\nrunbook: https://github.com/openshift/ops-sop/blob/master/v4/alerts/cluster_has_gone_missing.md\n",
		"token":                 "e686efc420",
	}

	alert := makeAlert(details)

	serviceSummary := "prod-deadmanssnitch"
	title := "jfrogdevrrrooo.5uqt.p1.openshiftapps.com has gone missing"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "deadmanssnitch", result.AlertType)
	assert.Equal(t, "ClusterGoneMissing", result.AlertName)
	assert.Equal(t, "06f058a0-35fa-42c5-83a1-f39184fee819", result.ClusterID)
	assert.Equal(t, "critical", result.Severity) // Always critical
	assert.Equal(t, "jfrogdevrrrooo.5uqt.p1.openshiftapps.com", result.ClusterName)
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/cluster_has_gone_missing.md", result.SOPLink)
	assert.Equal(t, title, result.Title)
}

func TestNormalizeAlert_CEE_NoAlerts(t *testing.T) {
	// CEE escalations have no alert body details
	alert := pagerduty.IncidentAlert{
		APIObject: pagerduty.APIObject{
			ID: "ALERT001",
		},
		Status:    "triggered",
		CreatedAt: "2026-05-29T10:00:00Z",
		Service: pagerduty.APIObject{
			Summary: "cee-srep-sev1-escalation-pager",
		},
		Incident: pagerduty.APIReference{
			ID: "INC001",
		},
		Body: nil,
	}

	serviceSummary := "cee-srep-sev1-escalation-pager"
	title := "OHSS-54116: Access request tracker for cluster '2m89uoc2k3hg86u969bss1q0godb07ek'"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "cee_escalation", result.AlertType)
	assert.Equal(t, "OHSS-54116", result.AlertName)
	assert.Equal(t, "critical", result.Severity) // Sev1 escalation is critical
	assert.Equal(t, title, result.Title)
	assert.Equal(t, 0, result.FiringCount) // No alerts for CEE
}

func TestNormalizeAlert_Unknown_DoesNotCrash(t *testing.T) {
	// Test with nil body
	alert := pagerduty.IncidentAlert{
		APIObject: pagerduty.APIObject{
			ID: "ALERT001",
		},
		Status:    "triggered",
		CreatedAt: "2026-05-29T10:00:00Z",
		Service: pagerduty.APIObject{
			Summary: "some-unknown-service-name",
		},
		Incident: pagerduty.APIReference{
			ID: "INC001",
		},
		Body: nil,
	}

	serviceSummary := "some-unknown-service-name"
	title := "Some Random Alert Title"

	// Must not panic
	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "unknown", result.AlertType)
	assert.Equal(t, title, result.Title)
	assert.Equal(t, "some-unknown-service-name", result.ServiceName)
	assert.Equal(t, "triggered", result.Status)
}

func TestNormalizeAlert_Unknown_WithBody(t *testing.T) {
	// Unknown type but has body details - should extract what it can
	details := map[string]interface{}{
		"alert_name": "SomeNewAlert",
		"cluster_id": "xyz-789",
	}

	alert := makeAlert(details)

	serviceSummary := "brand-new-pipeline-v2"
	title := "SomeNewAlert triggered"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "unknown", result.AlertType)
	assert.Equal(t, "SomeNewAlert", result.AlertName)
	assert.Equal(t, "xyz-789", result.ClusterID)
}

func TestNormalizeAlert_CAD_IdentifiedCorrectly(t *testing.T) {
	alert := pagerduty.IncidentAlert{
		APIObject: pagerduty.APIObject{
			ID: "ALERT001",
		},
		Status:    "triggered",
		CreatedAt: "2026-05-29T10:00:00Z",
		Service: pagerduty.APIObject{
			Summary: "CAD Stage",
		},
		Incident: pagerduty.APIReference{
			ID: "INC001",
		},
		Body: nil,
	}

	serviceSummary := "CAD Stage"
	title := "AlertName - E2E"

	result := NormalizeAlert(serviceSummary, title, alert)

	assert.Equal(t, "cad", result.AlertType)
}

func TestNormalizeAlert_OSDHive_SeverityNormalization(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "CRITICAL uppercase",
			title:    "AlertName CRITICAL (1)",
			expected: "critical",
		},
		{
			name:     "Critical mixed case",
			title:    "AlertName Critical (1)",
			expected: "critical",
		},
		{
			name:     "WARNING uppercase",
			title:    "AlertName WARNING (1)",
			expected: "warning",
		},
		{
			name:     "Warning mixed case",
			title:    "AlertName Warning (1)",
			expected: "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := map[string]interface{}{
				"alert_name": "AlertName",
				"cluster_id": "test-123",
				"num_firing": "1",
			}
			alert := makeAlert(details)

			result := NormalizeAlert(
				"osd-mycluster.abc1.p1.openshiftapps.com-hive-cluster",
				tt.title,
				alert,
			)
			assert.Equal(t, tt.expected, result.Severity)
		})
	}
}

func TestRHOBSHCP_ExtractsNamespaceAndDescription(t *testing.T) {
	alert := pagerduty.IncidentAlert{
		Service: pagerduty.APIObject{Summary: "rhobs-hcp-prod-critical-us-west-2"},
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"alert_name": "ClusterOperatorDown",
				"cluster_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				"firing":     "\n\n  - alertname: ClusterOperatorDown\n    cluster_id: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee\n    namespace: ocm-production-aaaabbbbccccddddeeee-test-us-east-2\n    description: The ingress operator is unavailable for aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee.\n\n",
				"link":       "https://github.com/openshift/ops-sop/blob/master/hypershift/alerts/ClusterOperatorDown.md",
			},
		},
	}

	result := NormalizeAlert("rhobs-hcp-prod-critical-us-west-2", "[HCP] [RHOBS] (Critical) ClusterOperatorDown for HCP: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", alert)

	assert.Equal(t, "ocm-production-aaaabbbbccccddddeeee-test-us-east-2", result.Namespace)
	assert.Equal(t, "The ingress operator is unavailable for aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee.", result.Description)
}

func TestOSDHive_ExtractsNamespaceAndMessage(t *testing.T) {
	alert := pagerduty.IncidentAlert{
		Service: pagerduty.APIObject{Summary: "osd-testcluster.p1.openshiftapps.com-hive-cluster"},
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"alert_name": "PruningCronjobErrorSRE",
				"cluster_id": "11111111-2222-3333-4444-555555555555",
				"firing":     "Labels:\n - alertname = PruningCronjobErrorSRE\n - namespace = openshift-sre-pruning\n - severity = critical\nAnnotations:\n - message = SRE Pruning Job openshift-sre-pruning/builds-pruner is taking more than thirty minutes to complete.\n - summary = SRE pruning cronjob error\nSource: https://console.redhat.com/monitoring",
				"link":       "https://github.com/openshift/ops-sop/blob/master/v4/alerts/PruningCronjobErrorSRE.md",
			},
		},
	}

	result := NormalizeAlert("osd-testcluster.p1.openshiftapps.com-hive-cluster", "PruningCronjobErrorSRE CRITICAL (1)", alert)

	assert.Equal(t, "openshift-sre-pruning", result.Namespace)
	assert.Equal(t, "SRE Pruning Job openshift-sre-pruning/builds-pruner is taking more than thirty minutes to complete.", result.Description)
}
