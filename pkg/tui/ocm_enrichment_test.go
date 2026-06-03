package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
)

func createMockOCMClient() *ocm.MockClient {
	mock := ocm.NewMockClient()
	mock.Clusters["cluster-aaa"] = &ocm.ClusterInfo{
		ID:            "cluster-aaa",
		ExternalID:    "aaaa-bbbb-cccc-dddd",
		Name:          "test-cluster",
		DisplayName:   "Test Cluster Display",
		State:         "ready",
		Region:        "us-east-1",
		CloudProvider: "aws",
		Version:       "4.16.5",
		CCS:           true,
	}
	mock.ServiceLogs["cluster-aaa"] = []ocm.ServiceLog{
		{
			Timestamp:   "2026-06-01T10:00:00Z",
			Severity:    "Warning",
			ServiceName: "SREManualAction",
			Summary:     "Test log",
			Description: "Test description",
			ClusterID:   "cluster-aaa",
		},
	}
	mock.ClusterReports["cluster-aaa"] = []ocm.ClusterReport{
		{Title: "Health Check", Summary: "All good", CreatedAt: "2026-06-01T10:00:00Z"},
	}
	mock.LimitedSupport["cluster-aaa"] = []ocm.LimitedSupportReason{
		{ID: "ls-1", Summary: "Customer modification", CreatedAt: "2026-05-15T08:00:00Z"},
	}
	return mock
}

func TestGetClusterInfo_Success(t *testing.T) {
	t.Run("returns clusterInfoMsg with cluster data", func(t *testing.T) {
		mock := createMockOCMClient()

		cmd := getClusterInfo(mock, "cluster-aaa")
		msg := cmd()

		infoMsg, ok := msg.(clusterInfoMsg)
		assert.True(t, ok, "should return clusterInfoMsg")
		assert.NoError(t, infoMsg.err)
		assert.Equal(t, "cluster-aaa", infoMsg.clusterID)
		assert.Equal(t, "Test Cluster Display", infoMsg.info.DisplayName)
	})
}

func TestGetClusterInfo_NotFound(t *testing.T) {
	t.Run("returns error for unknown cluster", func(t *testing.T) {
		mock := ocm.NewMockClient()

		cmd := getClusterInfo(mock, "nonexistent")
		msg := cmd()

		infoMsg, ok := msg.(clusterInfoMsg)
		assert.True(t, ok)
		assert.Error(t, infoMsg.err)
	})
}

func TestGetServiceLogs_Success(t *testing.T) {
	t.Run("returns serviceLogsMsg with logs", func(t *testing.T) {
		mock := createMockOCMClient()

		cmd := getOCMServiceLogs(mock, "cluster-aaa", "aaaa-bbbb-cccc-dddd")
		msg := cmd()

		logsMsg, ok := msg.(ocmServiceLogsMsg)
		assert.True(t, ok)
		assert.NoError(t, logsMsg.err)
		assert.Len(t, logsMsg.logs, 1)
		assert.Equal(t, "Test log", logsMsg.logs[0].Summary)
	})
}

func TestGetClusterReports_Success(t *testing.T) {
	t.Run("returns clusterReportsMsg with reports", func(t *testing.T) {
		mock := createMockOCMClient()

		cmd := getClusterReports(mock, "cluster-aaa")
		msg := cmd()

		reportsMsg, ok := msg.(clusterReportsMsg)
		assert.True(t, ok)
		assert.NoError(t, reportsMsg.err)
		assert.Len(t, reportsMsg.reports, 1)
	})
}

func TestGetLimitedSupportHistory_Success(t *testing.T) {
	t.Run("returns limitedSupportMsg with reasons", func(t *testing.T) {
		mock := createMockOCMClient()

		cmd := getLimitedSupportHistory(mock, "cluster-aaa")
		msg := cmd()

		lsMsg, ok := msg.(limitedSupportMsg)
		assert.True(t, ok)
		assert.NoError(t, lsMsg.err)
		assert.Len(t, lsMsg.reasons, 1)
	})
}

func TestClusterInfoMsg_UpdatesCache(t *testing.T) {
	t.Run("clusterInfoMsg stores data in clusterCache", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = make(map[string]*ocm.ClusterInfo)

		msg := clusterInfoMsg{
			clusterID: "cluster-aaa",
			info: &ocm.ClusterInfo{
				ID:          "cluster-aaa",
				DisplayName: "Cached Cluster",
			},
			err: nil,
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.clusterCache, "cluster-aaa")
		assert.Equal(t, "Cached Cluster", updated.clusterCache["cluster-aaa"].DisplayName)
	})
}

func TestEnrichClusters_DispatchesCommands(t *testing.T) {
	t.Run("enrichClusters returns commands for each unique cluster", func(t *testing.T) {
		mock := createMockOCMClient()

		clusterIDs := []string{"cluster-aaa", "cluster-bbb"}
		cmds := enrichClusters(mock, clusterIDs)

		assert.Len(t, cmds, 8, "should return 4 commands per cluster (info, logs, reports, limited support)")
	})

	t.Run("enrichClusters returns nil for empty cluster list", func(t *testing.T) {
		mock := createMockOCMClient()

		cmds := enrichClusters(mock, []string{})
		assert.Empty(t, cmds)
	})

	t.Run("enrichClusters returns nil when client is nil", func(t *testing.T) {
		cmds := enrichClusters(nil, []string{"cluster-aaa"})
		assert.Empty(t, cmds)
	})
}

func TestIncidentClusterMap_PopulatedOnAlerts(t *testing.T) {
	t.Run("gotIncidentAlertsMsg populates incidentClusterMap", func(t *testing.T) {
		m := createTestModel()
		m.incidentClusterMap = make(map[string][]string)
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC001"},
		}
		m.incidentCache = make(map[string]*cachedIncidentData)

		msg := gotIncidentAlertsMsg{
			incidentID: "INC001",
			alerts: []pagerduty.IncidentAlert{
				{
					APIObject: pagerduty.APIObject{ID: "A1"},
					Body: map[string]interface{}{
						"details": map[string]interface{}{
							"cluster_id": "cluster-aaa",
						},
					},
				},
			},
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.incidentClusterMap, "INC001")
		assert.Equal(t, []string{"cluster-aaa"}, updated.incidentClusterMap["INC001"])
	})
}

func TestInitialModelWithConfig_MapsInitialized(t *testing.T) {
	t.Run("all OCM maps are initialized", func(t *testing.T) {
		config := &pd.Config{
			Client: &pd.MockPagerDutyClient{},
		}

		teaModel, _ := InitialModelWithConfig(config, []string{"vi"}, launcher.ClusterLauncher{}, false, nil)
		m := teaModel.(model)

		assert.NotNil(t, m.incidentClusterMap, "incidentClusterMap should be initialized")
		assert.NotNil(t, m.clusterCache, "clusterCache should be initialized")
		assert.NotNil(t, m.serviceLogCache, "serviceLogCache should be initialized")
		assert.NotNil(t, m.clusterReportCache, "clusterReportCache should be initialized")
		assert.NotNil(t, m.limitedSupportCache, "limitedSupportCache should be initialized")
	})
}

func TestRenderClusterTab_WithData(t *testing.T) {
	t.Run("renders cluster info when cached", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = map[string]*ocm.ClusterInfo{
			"cluster-aaa": {
				ID:          "cluster-aaa",
				DisplayName: "Test Cluster",
				Name:        "test",
				ExternalID:  "aaaa-bbbb",
				State:       "ready",
				Region:      "us-east-1",
				Version:     "4.16.5",
			},
		}

		content, err := m.renderClusterTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Test Cluster")
		assert.Contains(t, content, "us-east-1")
		assert.Contains(t, content, "1/1")
	})
}

func TestRenderClusterTab_NoOCM(t *testing.T) {
	t.Run("shows OCM not connected when client is nil", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = make(map[string]*ocm.ClusterInfo)
		m.ocmClient = nil

		content, err := m.renderClusterTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "OCM not connected")
	})
}

func TestRenderServiceLogsTab_WithData(t *testing.T) {
	t.Run("renders service logs when cached", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = map[string]*ocm.ClusterInfo{
			"cluster-aaa": {ID: "cluster-aaa"},
		}
		m.serviceLogCache = map[string][]ocm.ServiceLog{
			"cluster-aaa": {
				{Summary: "Test log", Severity: "Warning", Description: "Details here"},
			},
		}

		content, err := m.renderServiceLogsTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Test log")
		assert.Contains(t, content, "Warning")
		assert.Contains(t, content, "Details here")
	})
}

func TestRenderLimitedSupportTab_WithData(t *testing.T) {
	t.Run("renders limited support reasons when cached", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = map[string]*ocm.ClusterInfo{
			"cluster-aaa": {ID: "cluster-aaa"},
		}
		m.limitedSupportCache = map[string][]ocm.LimitedSupportReason{
			"cluster-aaa": {
				{Summary: "Customer modification", DetectionType: "manual"},
			},
		}

		content, err := m.renderLimitedSupportTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Customer modification")
		assert.Contains(t, content, "manual")
	})
}

func TestRenderClusterReportsTab_WithData(t *testing.T) {
	t.Run("renders reports when cached", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = map[string]*ocm.ClusterInfo{
			"cluster-aaa": {ID: "cluster-aaa"},
		}
		m.clusterReportCache = map[string][]ocm.ClusterReport{
			"cluster-aaa": {
				{Title: "Health Check", Summary: "All good"},
			},
		}

		content, err := m.renderClusterReportsTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Health Check")
		assert.Contains(t, content, "All good")
	})
}

func TestSortedClusterIDs(t *testing.T) {
	t.Run("returns sorted cluster IDs", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = map[string]*ocm.ClusterInfo{
			"zzz": {ID: "zzz"},
			"aaa": {ID: "aaa"},
			"mmm": {ID: "mmm"},
		}

		ids := m.sortedClusterIDs()
		assert.Equal(t, []string{"aaa", "mmm", "zzz"}, ids)
	})
}

func TestCacheCleanup_OnClearSelectedIncident(t *testing.T) {
	t.Run("clearSelectedIncident clears OCM caches", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q123"},
		}
		m.clusterCache = map[string]*ocm.ClusterInfo{
			"cluster-aaa": {ID: "cluster-aaa"},
		}
		m.serviceLogCache = map[string][]ocm.ServiceLog{
			"cluster-aaa": {{Summary: "log"}},
		}
		m.clusterReportCache = map[string][]ocm.ClusterReport{
			"cluster-aaa": {{Title: "report"}},
		}
		m.limitedSupportCache = map[string][]ocm.LimitedSupportReason{
			"cluster-aaa": {{Summary: "reason"}},
		}

		m.clearSelectedIncident("test cleanup")

		assert.Empty(t, m.clusterCache)
		assert.Empty(t, m.serviceLogCache)
		assert.Empty(t, m.clusterReportCache)
		assert.Empty(t, m.limitedSupportCache)
	})
}
