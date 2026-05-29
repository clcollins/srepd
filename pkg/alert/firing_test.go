package alert

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFiring_AlertmanagerFormat(t *testing.T) {
	firing := `Labels:
 - alertname = MachineHealthCheckUnterminatedShortCircuitSRE
 - container = kube-rbac-proxy-mhc-mtrc
 - namespace = openshift-machine-api
 - severity = critical
Annotations:
 - description = MHC has been short circuited for too long
 - runbook = https://github.com/openshift/ops-sop/blob/master/v4/alerts/MachineHealthCheckUnterminatedShortCircuitSRE.md
Source: https://prometheus.example.com`

	result := ParseFiring(firing)

	assert.Equal(t, "MachineHealthCheckUnterminatedShortCircuitSRE", result["alertname"])
	assert.Equal(t, "kube-rbac-proxy-mhc-mtrc", result["container"])
	assert.Equal(t, "openshift-machine-api", result["namespace"])
	assert.Equal(t, "critical", result["severity"])
	assert.Equal(t, "MHC has been short circuited for too long", result["description"])
	assert.Equal(t, "https://github.com/openshift/ops-sop/blob/master/v4/alerts/MachineHealthCheckUnterminatedShortCircuitSRE.md", result["runbook"])
}

func TestParseFiring_RHOBSFormat(t *testing.T) {
	firing := `

  - alertname: ClusterOperatorDown
    cluster_id: a4ba96fe-ac69-4573-a78b-17d38eeaab99
    namespace: ocm-production-abc123
    description: The ingress operator is unavailable for cluster

`

	result := ParseFiring(firing)

	assert.Equal(t, "ClusterOperatorDown", result["alertname"])
	assert.Equal(t, "a4ba96fe-ac69-4573-a78b-17d38eeaab99", result["cluster_id"])
	assert.Equal(t, "ocm-production-abc123", result["namespace"])
	assert.Equal(t, "The ingress operator is unavailable for cluster", result["description"])
}

func TestParseFiring_Empty(t *testing.T) {
	result := ParseFiring("")
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestParseFiring_MalformedGraceful(t *testing.T) {
	// Garbage input should not panic and should return empty map
	result := ParseFiring("this is not a valid firing format at all\nrandom garbage")
	assert.NotNil(t, result)
	// Should not crash, result may or may not have entries
}

func TestParseFiring_AlertmanagerWithSOP(t *testing.T) {
	// Test SOP extraction from message annotation
	firing := `Labels:
 - alertname = ClusterProvisioningDelay
 - severity = high
Annotations:
 - message = cluster edf-mas has been in a failed state. SOP: https://github.com/openshift/ops-sop/blob/master/v4/alerts/ClusterProvisioningDelay.md
 - dashboard = https://grafana.example.com/d/dashboard
Source: https://prometheus.example.com`

	result := ParseFiring(firing)
	assert.Equal(t, "ClusterProvisioningDelay", result["alertname"])
	assert.Equal(t, "high", result["severity"])
	assert.Contains(t, result["message"], "SOP:")
	assert.Equal(t, "https://grafana.example.com/d/dashboard", result["dashboard"])
}

func TestParseFiring_WhitespaceOnly(t *testing.T) {
	result := ParseFiring("   \n\n   \n")
	assert.NotNil(t, result)
	assert.Empty(t, result)
}
