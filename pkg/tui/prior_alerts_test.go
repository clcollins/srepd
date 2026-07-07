package tui

import (
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeIncidentWithLogEntry(id, title, serviceName, clusterID, alertName, createdAt string) pagerduty.Incident {
	channel := map[string]interface{}{
		"details": map[string]interface{}{
			"cluster_id": clusterID,
			"alert_name": alertName,
		},
	}
	return pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: id, HTMLURL: "https://pd.example.com/" + id},
		Title:     title,
		CreatedAt: createdAt,
		Service:   pagerduty.APIObject{Summary: serviceName},
		FirstTriggerLogEntry: pagerduty.FirstTriggerLogEntry{
			CommonLogEntryField: pagerduty.CommonLogEntryField{
				Channel: pagerduty.Channel{Raw: channel},
			},
		},
	}
}

func TestFetchPriorAlerts_Tier1_HiveService(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					makeIncidentWithLogEntry("INC1", "ClusterOperatorDown CRITICAL (1)",
						"osd-test.abc.p1.openshiftapps.com-hive-cluster", "cluster-abc-123", "ClusterOperatorDown", "2026-06-15T10:00:00Z"),
					makeIncidentWithLogEntry("INC2", "KubePodNotReady WARNING (1)",
						"osd-test.abc.p1.openshiftapps.com-hive-cluster", "cluster-abc-123", "KubePodNotReady", "2026-06-10T08:00:00Z"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, []string{"SVC_001"}, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT",
		priorAlertWeek{since: time.Now().AddDate(0, 0, -90), until: time.Now()}, nil)
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	assert.NoError(t, msg.err)
	require.NotNil(t, msg.data)
	assert.Len(t, msg.data.SameAlert, 1)
	assert.Equal(t, "ClusterOperatorDown", msg.data.SameAlert[0].AlertName)
	assert.Len(t, msg.data.OtherAlerts, 1)
	assert.Equal(t, "KubePodNotReady", msg.data.OtherAlerts[0].AlertName)
	assert.Equal(t, 0, mock.CallCounts["ListIncidentAlertsWithContext"], "should never call GetAlerts")
}

func TestFetchPriorAlerts_Tier2_RHOBSTitleMatch(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					makeIncidentWithLogEntry("INC1", "[HCP] [RHOBS] (Critical) SomeAlert for HCP: cluster-abc-123",
						"rhobs-hcp-prod-critical-us-east-1", "cluster-abc-123", "SomeAlert", "2026-06-15T10:00:00Z"),
					makeIncidentWithLogEntry("INC2", "[HCP] [RHOBS] (Critical) OtherAlert for HCP: cluster-xyz-999",
						"rhobs-hcp-prod-critical-us-east-1", "cluster-xyz-999", "OtherAlert", "2026-06-10T08:00:00Z"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, []string{"SVC_RHOBS"}, []string{"TEAM_001"}, "cluster-abc-123", "SomeAlert", "INC_CURRENT",
		priorAlertWeek{since: time.Now().AddDate(0, 0, -90), until: time.Now()}, nil)
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	require.NotNil(t, msg.data)
	assert.Len(t, msg.data.SameAlert, 1, "should match by title")
	assert.Empty(t, msg.data.OtherAlerts, "cluster-xyz-999 should not match")
	assert.Equal(t, 0, mock.CallCounts["ListIncidentAlertsWithContext"], "should never call GetAlerts")
}

func TestFetchPriorAlerts_Tier3_LogEntryMatch(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					makeIncidentWithLogEntry("INC1", "some-cluster has gone missing",
						"prod-deadmanssnitch", "cluster-abc-123", "", "2026-06-15T10:00:00Z"),
					makeIncidentWithLogEntry("INC2", "other-cluster has gone missing",
						"prod-deadmanssnitch", "cluster-xyz-999", "", "2026-06-10T08:00:00Z"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, []string{"SVC_DMS"}, []string{"TEAM_001"}, "cluster-abc-123", "ClusterGoneMissing", "INC_CURRENT",
		priorAlertWeek{since: time.Now().AddDate(0, 0, -90), until: time.Now()}, nil)
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	require.NotNil(t, msg.data)
	assert.Len(t, msg.data.SameAlert, 1, "should match via log entry cluster_id")
	assert.Empty(t, msg.data.OtherAlerts, "cluster-xyz-999 should not match")
	assert.Equal(t, 0, mock.CallCounts["ListIncidentAlertsWithContext"], "should never call GetAlerts")
}

func TestFetchPriorAlerts_SkipsCurrentIncident(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{
				Incidents: []pagerduty.Incident{
					makeIncidentWithLogEntry("INC_CURRENT", "ClusterOperatorDown CRITICAL (1)",
						"osd-test-hive-cluster", "cluster-abc-123", "ClusterOperatorDown", "2026-07-01T10:00:00Z"),
				},
			},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, []string{"SVC_001"}, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT",
		priorAlertWeek{since: time.Now().AddDate(0, 0, -90), until: time.Now()}, nil)
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	require.NotNil(t, msg.data)
	assert.Empty(t, msg.data.SameAlert, "current incident should be skipped")
}

func TestFetchPriorAlerts_NoMatches(t *testing.T) {
	mock := &pd.MockPagerDutyClient{
		ListIncidentsResponses: []pagerduty.ListIncidentsResponse{
			{Incidents: []pagerduty.Incident{}},
		},
	}

	cmd := fetchPriorAlertsWeek(mock, nil, []string{"TEAM_001"}, "cluster-abc-123", "ClusterOperatorDown", "INC_CURRENT",
		priorAlertWeek{since: time.Now().AddDate(0, 0, -90), until: time.Now()}, nil)
	result := cmd()

	msg, ok := result.(priorAlertsMsg)
	require.True(t, ok)
	assert.NoError(t, msg.err)
	require.NotNil(t, msg.data)
	assert.Empty(t, msg.data.SameAlert)
	assert.Empty(t, msg.data.OtherAlerts)
}

func TestMatchIncidentToCluster_Tiers(t *testing.T) {
	tests := []struct {
		name      string
		incident  pagerduty.Incident
		clusterID string
		expected  bool
	}{
		{
			name: "Tier 1: hive service always matches",
			incident: pagerduty.Incident{
				Service: pagerduty.APIObject{Summary: "osd-test-hive-cluster"},
			},
			clusterID: "anything",
			expected:  true,
		},
		{
			name: "Tier 2: rhobs_hcp title contains cluster UUID",
			incident: pagerduty.Incident{
				Title:   "[HCP] [RHOBS] (Critical) AlertName for HCP: abc-123",
				Service: pagerduty.APIObject{Summary: "rhobs-hcp-prod-critical-us-east-1"},
			},
			clusterID: "abc-123",
			expected:  true,
		},
		{
			name: "Tier 2: rhobs_hcp title wrong cluster",
			incident: pagerduty.Incident{
				Title:   "[HCP] [RHOBS] (Critical) AlertName for HCP: xyz-999",
				Service: pagerduty.APIObject{Summary: "rhobs-hcp-prod-critical-us-east-1"},
			},
			clusterID: "abc-123",
			expected:  false,
		},
		{
			name: "Tier 3: log entry cluster_id match",
			incident: pagerduty.Incident{
				Service: pagerduty.APIObject{Summary: "prod-deadmanssnitch"},
				FirstTriggerLogEntry: pagerduty.FirstTriggerLogEntry{
					CommonLogEntryField: pagerduty.CommonLogEntryField{
						Channel: pagerduty.Channel{Raw: map[string]interface{}{
							"details": map[string]interface{}{"cluster_id": "abc-123"},
						}},
					},
				},
			},
			clusterID: "abc-123",
			expected:  true,
		},
		{
			name: "Tier 3: log entry custom_details match",
			incident: pagerduty.Incident{
				Service: pagerduty.APIObject{Summary: "app-sre-alertmanager"},
				FirstTriggerLogEntry: pagerduty.FirstTriggerLogEntry{
					CommonLogEntryField: pagerduty.CommonLogEntryField{
						Channel: pagerduty.Channel{Raw: map[string]interface{}{
							"custom_details": map[string]interface{}{"cluster_id": "abc-123"},
						}},
					},
				},
			},
			clusterID: "abc-123",
			expected:  true,
		},
		{
			name: "Tier 3: no log entry data",
			incident: pagerduty.Incident{
				Service: pagerduty.APIObject{Summary: "app-sre-alertmanager"},
			},
			clusterID: "abc-123",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchIncidentToCluster(tt.incident, tt.clusterID)
			assert.Equal(t, tt.expected, result)
		})
	}
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
	assert.Contains(t, result, "Same Alert")
	assert.Contains(t, result, "Other Alerts for this Cluster")
	assert.Contains(t, result, "ClusterOperatorDown")
	assert.Contains(t, result, "KubePodNotReady")
	assert.Contains(t, result, "INC1")
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
			"cluster-abc-123": {SameAlert: nil, OtherAlerts: nil},
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
