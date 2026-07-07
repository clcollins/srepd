package tui

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAlertWithCluster(clusterID, alertName, serviceSummary string) pagerduty.IncidentAlert {
	return pagerduty.IncidentAlert{
		APIObject: pagerduty.APIObject{ID: "ALERT_001"},
		Service:   pagerduty.APIObject{Summary: serviceSummary},
		Body: map[string]interface{}{
			"details": map[string]interface{}{
				"cluster_id": clusterID,
				"alert_name": alertName,
			},
		},
	}
}

func TestFetchPriorAlerts_MatchingSameAlert(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					{
						APIObject: pagerduty.APIObject{ID: "INC_PRIOR_1", HTMLURL: "https://pd.example.com/INC_PRIOR_1"},
						Title:     "ClusterOperatorDown CRITICAL (1)",
						CreatedAt: "2026-06-15T10:00:00Z",
						Service:   pagerduty.APIObject{Summary: "osd-test-hive-cluster"},
					},
					{
						APIObject: pagerduty.APIObject{ID: "INC_PRIOR_2", HTMLURL: "https://pd.example.com/INC_PRIOR_2"},
						Title:     "KubePodNotReady WARNING (1)",
						CreatedAt: "2026-06-10T08:00:00Z",
						Service:   pagerduty.APIObject{Summary: "osd-test-hive-cluster"},
					},
				},
			},
		},
		ListIncidentAlertsResponses: map[string]*pagerduty.ListAlertsResponse{
			"INC_PRIOR_1": {
				Alerts: []pagerduty.IncidentAlert{
					makeAlertWithCluster("cluster-abc-123", "ClusterOperatorDown", "osd-test-hive-cluster"),
				},
			},
			"INC_PRIOR_2": {
				Alerts: []pagerduty.IncidentAlert{
					makeAlertWithCluster("cluster-abc-123", "KubePodNotReady", "osd-test-hive-cluster"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, nil, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT", time.Now().AddDate(0, 0, -90), time.Now())
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok, "expected priorAlertsMsg")
	assert.NoError(t, msg.err)
	assert.Equal(t, "cluster-abc-123", msg.clusterID)

	require.NotNil(t, msg.data)
	assert.Len(t, msg.data.SameAlert, 1, "should find 1 same-alert match")
	assert.Equal(t, "ClusterOperatorDown", msg.data.SameAlert[0].AlertName)
	assert.Equal(t, "INC_PRIOR_1", msg.data.SameAlert[0].IncidentID)
	assert.Equal(t, "https://pd.example.com/INC_PRIOR_1", msg.data.SameAlert[0].IncidentURL)

	assert.Len(t, msg.data.OtherAlerts, 1, "should find 1 other-alert match")
	assert.Equal(t, "KubePodNotReady", msg.data.OtherAlerts[0].AlertName)
	assert.Equal(t, "INC_PRIOR_2", msg.data.OtherAlerts[0].IncidentID)
}

func TestFetchPriorAlerts_NoMatches(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					{
						APIObject: pagerduty.APIObject{ID: "INC_OTHER"},
						Title:     "SomeAlert CRITICAL (1)",
						CreatedAt: "2026-06-15T10:00:00Z",
						Service:   pagerduty.APIObject{Summary: "osd-other-hive-cluster"},
					},
				},
			},
		},
		ListIncidentAlertsResponses: map[string]*pagerduty.ListAlertsResponse{
			"INC_OTHER": {
				Alerts: []pagerduty.IncidentAlert{
					makeAlertWithCluster("cluster-xyz-999", "SomeAlert", "osd-other-hive-cluster"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, nil, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT", time.Now().AddDate(0, 0, -90), time.Now())
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	assert.NoError(t, msg.err)
	require.NotNil(t, msg.data)
	assert.Empty(t, msg.data.SameAlert)
	assert.Empty(t, msg.data.OtherAlerts)
}

func TestFetchPriorAlerts_SkipsCurrentIncident(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					{
						APIObject: pagerduty.APIObject{ID: "INC_CURRENT"},
						Title:     "ClusterOperatorDown CRITICAL (1)",
						CreatedAt: "2026-07-01T10:00:00Z",
						Service:   pagerduty.APIObject{Summary: "osd-test-hive-cluster"},
					},
				},
			},
		},
		ListIncidentAlertsResponses: map[string]*pagerduty.ListAlertsResponse{
			"INC_CURRENT": {
				Alerts: []pagerduty.IncidentAlert{
					makeAlertWithCluster("cluster-abc-123", "ClusterOperatorDown", "osd-test-hive-cluster"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, nil, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT", time.Now().AddDate(0, 0, -90), time.Now())
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	assert.NoError(t, msg.err)
	require.NotNil(t, msg.data)
	assert.Empty(t, msg.data.SameAlert, "current incident should not appear in results")
	assert.Empty(t, msg.data.OtherAlerts)
}

func TestFetchPriorAlerts_CrossServiceMatch(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					{
						APIObject: pagerduty.APIObject{ID: "INC_RHOBS_1", HTMLURL: "https://pd.example.com/INC_RHOBS_1"},
						Title:     "[HCP] [RHOBS] (critical) SomeRHOBSAlert for HCP: cluster-abc-123",
						CreatedAt: "2026-06-20T14:00:00Z",
						Service:   pagerduty.APIObject{Summary: "rhobs-hcp-prod-critical-us-east-1"},
					},
				},
			},
		},
		ListIncidentAlertsResponses: map[string]*pagerduty.ListAlertsResponse{
			"INC_RHOBS_1": {
				Alerts: []pagerduty.IncidentAlert{
					makeAlertWithCluster("cluster-abc-123", "SomeRHOBSAlert", "rhobs-hcp-prod-critical-us-east-1"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, nil, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT", time.Now().AddDate(0, 0, -90), time.Now())
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	assert.NoError(t, msg.err)
	require.NotNil(t, msg.data)
	assert.Empty(t, msg.data.SameAlert, "RHOBS alert should not match ClusterOperatorDown")
	assert.Len(t, msg.data.OtherAlerts, 1, "RHOBS alert should appear in other alerts")
	assert.Equal(t, "SomeRHOBSAlert", msg.data.OtherAlerts[0].AlertName)
}

func TestFetchPriorAlerts_EmptyResponse(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{Incidents: []pagerduty.Incident{}},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, nil, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT", time.Now().AddDate(0, 0, -90), time.Now())
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	assert.NoError(t, msg.err)
	require.NotNil(t, msg.data)
	assert.Empty(t, msg.data.SameAlert)
	assert.Empty(t, msg.data.OtherAlerts)
}

func TestRenderPDHistoryTab_WithData(t *testing.T) {
	m := model{
		selectedIncident: &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC_CURRENT"},
		},
		incidentClusterMap: map[string][]string{
			"INC_CURRENT": {"cluster-abc-123"},
		},
		priorAlertCache: map[string]*PriorAlertData{
			"cluster-abc-123": {
				SameAlert: []PriorAlert{
					{Date: "2026-06-15T10:00:00Z", AlertName: "ClusterOperatorDown", IncidentURL: "https://pd.example.com/INC1", IncidentID: "INC1"},
					{Date: "2026-06-10T08:00:00Z", AlertName: "ClusterOperatorDown", IncidentURL: "https://pd.example.com/INC2", IncidentID: "INC2"},
				},
				OtherAlerts: []PriorAlert{
					{Date: "2026-06-12T12:00:00Z", AlertName: "KubePodNotReady", IncidentURL: "https://pd.example.com/INC3", IncidentID: "INC3"},
				},
			},
		},
		priorAlertPending: make(map[string]int),
	}

	result, err := m.renderPDHistoryTab()
	assert.NoError(t, err)

	assert.Contains(t, result, "## Same Alert")
	assert.Contains(t, result, "## Other Alerts for this Cluster")
	assert.Contains(t, result, "ClusterOperatorDown")
	assert.Contains(t, result, "KubePodNotReady")
	assert.Contains(t, result, "INC1")
	assert.Contains(t, result, "INC2")
	assert.Contains(t, result, "INC3")
	assert.Contains(t, result, "2026-06-15 10:00")
}

func TestRenderPDHistoryTab_Loading(t *testing.T) {
	m := model{
		selectedIncident: &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC_CURRENT"},
		},
		incidentClusterMap: map[string][]string{
			"INC_CURRENT": {"cluster-abc-123"},
		},
		priorAlertCache: make(map[string]*PriorAlertData),
		priorAlertPending: map[string]int{
			"cluster-abc-123": 3,
		},
	}

	result, err := m.renderPDHistoryTab()
	assert.NoError(t, err)
	assert.Contains(t, result, "Loading")
}

func TestRenderPDHistoryTab_EmptyResults(t *testing.T) {
	m := model{
		selectedIncident: &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC_CURRENT"},
		},
		incidentClusterMap: map[string][]string{
			"INC_CURRENT": {"cluster-abc-123"},
		},
		priorAlertCache: map[string]*PriorAlertData{
			"cluster-abc-123": {
				SameAlert:   nil,
				OtherAlerts: nil,
			},
		},
		priorAlertPending: make(map[string]int),
	}

	result, err := m.renderPDHistoryTab()
	assert.NoError(t, err)
	assert.Contains(t, result, "No prior instances")
	assert.Contains(t, result, "No other alerts")
}

func TestRenderPDHistoryTab_FetchDoneNoData(t *testing.T) {
	m := model{
		selectedIncident: &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC_CURRENT"},
		},
		incidentClusterMap: map[string][]string{
			"INC_CURRENT": {"cluster-abc-123"},
		},
		priorAlertCache:   make(map[string]*PriorAlertData),
		priorAlertPending: make(map[string]int),
	}

	result, err := m.renderPDHistoryTab()
	assert.NoError(t, err)
	assert.Contains(t, result, "No PD history available")
}

func TestTabConstants(t *testing.T) {
	assert.Equal(t, 7, tabPDHistory, "PD History tab should be index 7")
	assert.Equal(t, 8, tabCount, "tabCount should be 8 with PD History tab")
}
