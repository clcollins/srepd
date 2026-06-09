package ai

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/alert"
	"github.com/stretchr/testify/assert"
)

func TestBuildSystemPrompt_WithIncident(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P123ABC"},
		Title:     "ClusterOperatorDown WARNING (1)",
		Status:    "triggered",
		Urgency:   "high",
		Service:   pagerduty.APIObject{Summary: "osd-test-cluster-hive-cluster"},
	}

	alerts := []alert.NormalizedAlert{
		{AlertName: "ClusterOperatorDown", ClusterID: "abc-123-def-456", Severity: "warning"},
		{AlertName: "PodDisruptionBudgetLimit", ClusterID: "abc-123-def-456", Severity: "critical"},
	}

	prompt := BuildSystemPrompt(incident, alerts)

	assert.Contains(t, prompt, "ClusterOperatorDown WARNING (1)")
	assert.Contains(t, prompt, "P123ABC")
	assert.Contains(t, prompt, "osd-test-cluster-hive-cluster")
	assert.Contains(t, prompt, "triggered")
	assert.Contains(t, prompt, "high")
	assert.Contains(t, prompt, "abc-123-def-456")
	assert.Contains(t, prompt, "2")
	assert.Contains(t, prompt, "ClusterOperatorDown")
	assert.Contains(t, prompt, "PodDisruptionBudgetLimit")
	assert.Contains(t, prompt, "Do not suggest destructive commands")
}

func TestBuildSystemPrompt_NilIncident(t *testing.T) {
	prompt := BuildSystemPrompt(nil, nil)

	assert.Contains(t, prompt, "SRE assistant")
	assert.Contains(t, prompt, "Do not suggest destructive commands")
	assert.NotContains(t, prompt, "Incident:")
}

func TestBuildSystemPrompt_EmptyAlerts(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P456DEF"},
		Title:     "Test Incident",
		Status:    "acknowledged",
		Urgency:   "low",
		Service:   pagerduty.APIObject{Summary: "test-service"},
	}

	prompt := BuildSystemPrompt(incident, []alert.NormalizedAlert{})

	assert.Contains(t, prompt, "P456DEF")
	assert.Contains(t, prompt, "Alert count: 0")
}

func TestBuildSystemPrompt_AlertsWithEmptyNames(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "P789GHI"},
		Title:     "Test Incident",
		Status:    "triggered",
		Urgency:   "high",
		Service:   pagerduty.APIObject{Summary: "test-service"},
	}

	alerts := []alert.NormalizedAlert{
		{AlertName: "SomeAlert", ClusterID: "cluster-1"},
		{AlertName: "", ClusterID: "cluster-1"},
		{AlertName: "AnotherAlert", ClusterID: ""},
	}

	prompt := BuildSystemPrompt(incident, alerts)

	assert.Contains(t, prompt, "SomeAlert")
	assert.Contains(t, prompt, "AnotherAlert")
	assert.Contains(t, prompt, "cluster-1")
	assert.Contains(t, prompt, "Alert count: 3")
}

func TestExtractClusterID_FirstWithID(t *testing.T) {
	alerts := []alert.NormalizedAlert{
		{ClusterID: ""},
		{ClusterID: "cluster-abc"},
		{ClusterID: "cluster-def"},
	}

	assert.Equal(t, "cluster-abc", extractClusterID(alerts))
}

func TestExtractClusterID_NoneWithID(t *testing.T) {
	alerts := []alert.NormalizedAlert{
		{ClusterID: ""},
		{ClusterID: ""},
	}

	assert.Equal(t, "", extractClusterID(alerts))
}

func TestExtractClusterID_EmptySlice(t *testing.T) {
	assert.Equal(t, "", extractClusterID([]alert.NormalizedAlert{}))
}

func TestExtractClusterID_NilSlice(t *testing.T) {
	assert.Equal(t, "", extractClusterID(nil))
}
