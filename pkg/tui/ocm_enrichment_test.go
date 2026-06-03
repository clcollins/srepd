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
	mock.Clusters["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"] = &ocm.ClusterInfo{
		ID:            "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
		ExternalID:    "00000000-fake-uuid-test-999999999999",
		Name:          "test-cluster",
		DisplayName:   "fake-osd-webapp.7x9k.p1.example.org",
		State:         "ready",
		Region:        "us-east-1",
		CloudProvider: "aws",
		Version:       "4.16.5",
		CCS:           true,
	}
	mock.ServiceLogs["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"] = []ocm.ServiceLog{
		{
			Timestamp:   "2026-06-01T10:00:00Z",
			Severity:    "Warning",
			ServiceName: "SREManualAction",
			Summary:     "Cluster entered limited support due to unsupported configuration",
			Description: "Customer replaced default IngressController with custom configuration that removed required SRE annotations. Cluster monitoring is degraded.",
			ClusterID:   "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
		},
	}
	mock.ClusterReports["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"] = []ocm.ClusterReport{
		{Title: "Health Check", Summary: "All good", CreatedAt: "2026-06-01T10:00:00Z"},
	}
	mock.LimitedSupport["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"] = []ocm.LimitedSupportReason{
		{ID: "ls-1", Summary: "Customer modification", CreatedAt: "2026-05-15T08:00:00Z"},
	}
	return mock
}

func TestGetClusterInfo_Success(t *testing.T) {
	t.Run("returns clusterInfoMsg with cluster data", func(t *testing.T) {
		mock := createMockOCMClient()

		cmd := getClusterInfo(mock, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		msg := cmd()

		infoMsg, ok := msg.(clusterInfoMsg)
		assert.True(t, ok, "should return clusterInfoMsg")
		assert.NoError(t, infoMsg.err)
		assert.Equal(t, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", infoMsg.clusterID)
		assert.Equal(t, "fake-osd-webapp.7x9k.p1.example.org", infoMsg.info.DisplayName)
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

		cmd := getOCMServiceLogs(mock, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", "00000000-fake-uuid-test-999999999999")
		msg := cmd()

		logsMsg, ok := msg.(ocmServiceLogsMsg)
		assert.True(t, ok)
		assert.NoError(t, logsMsg.err)
		assert.Len(t, logsMsg.logs, 1)
		assert.Equal(t, "Cluster entered limited support due to unsupported configuration", logsMsg.logs[0].Summary)
	})
}

func TestGetClusterReports_Success(t *testing.T) {
	t.Run("returns clusterReportsMsg with reports", func(t *testing.T) {
		mock := createMockOCMClient()

		cmd := getClusterReports(mock, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
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

		cmd := getLimitedSupportHistory(mock, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
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
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			info: &ocm.ClusterInfo{
				ID:          "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
				DisplayName: "fake-staging-api.4m2n.s1.example.org",
			},
			err: nil,
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.clusterCache, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.Equal(t, "fake-staging-api.4m2n.s1.example.org", updated.clusterCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"].DisplayName)
	})
}

func TestEnrichClusters_DispatchesCommands(t *testing.T) {
	t.Run("enrichClusters returns commands for each unique cluster", func(t *testing.T) {
		mock := createMockOCMClient()

		clusterIDs := []string{"1q2w3e4rfakeidtest9o0p1a2s3d4f5g", "2a3b4c5dfakeidtest0i1j2k3l4m5n6o"}
		cmds := enrichClusters(mock, clusterIDs, false)

		assert.Len(t, cmds, 8, "should return 4 commands per cluster (info, logs, reports, limited support)")
	})

	t.Run("enrichClusters returns nil for empty cluster list", func(t *testing.T) {
		mock := createMockOCMClient()

		cmds := enrichClusters(mock, []string{}, false)
		assert.Empty(t, cmds)
	})

	t.Run("enrichClusters returns nil when client is nil", func(t *testing.T) {
		cmds := enrichClusters(nil, []string{"1q2w3e4rfakeidtest9o0p1a2s3d4f5g"}, false)
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
							"cluster_id": "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
						},
					},
				},
			},
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.incidentClusterMap, "INC001")
		assert.Equal(t, []string{"1q2w3e4rfakeidtest9o0p1a2s3d4f5g"}, updated.incidentClusterMap["INC001"])
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

const testClusterID = "1q2w3e4rfakeidtest9o0p1a2s3d4f5g"

func setupModelWithCluster(m *model) {
	m.selectedIncident = &pagerduty.Incident{APIObject: pagerduty.APIObject{ID: "INC001"}}
	m.incidentClusterMap = map[string][]string{"INC001": {testClusterID}}
}

func TestRenderClusterTab_WithData(t *testing.T) {
	t.Run("renders cluster info when cached", func(t *testing.T) {
		m := createTestModel()
		setupModelWithCluster(&m)
		m.clusterCache = map[string]*ocm.ClusterInfo{
			testClusterID: {
				ID:          testClusterID,
				DisplayName: "fake-osd-webapp.7x9k.p1.example.org",
				Name:        "fake-osd-webapp",
				ExternalID:  "00000000-fake-uuid-test-999999999999",
				State:       "ready",
				Region:      "us-east-1",
				Version:     "4.16.5",
			},
		}

		content, err := m.renderClusterTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "fake-osd-webapp.7x9k.p1.example.org")
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
		setupModelWithCluster(&m)
		m.clusterCache = map[string]*ocm.ClusterInfo{
			testClusterID: {ID: testClusterID},
		}
		m.serviceLogCache = map[string][]ocm.ServiceLog{
			testClusterID: {
				{Summary: "Cluster entered limited support due to unsupported configuration", Severity: "Warning", Description: "Details here"},
			},
		}

		content, err := m.renderServiceLogsTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Cluster entered limited support due to unsupported configuration")
		assert.Contains(t, content, "Warning")
		assert.Contains(t, content, "Details here")
	})
}

func TestRenderLimitedSupportTab_WithData(t *testing.T) {
	t.Run("renders limited support reasons when cached", func(t *testing.T) {
		m := createTestModel()
		setupModelWithCluster(&m)
		m.clusterCache = map[string]*ocm.ClusterInfo{
			testClusterID: {ID: testClusterID},
		}
		m.limitedSupportCache = map[string][]ocm.LimitedSupportReason{
			testClusterID: {
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
		setupModelWithCluster(&m)
		m.clusterCache = map[string]*ocm.ClusterInfo{
			testClusterID: {ID: testClusterID},
		}
		m.clusterReportCache = map[string][]ocm.ClusterReport{
			testClusterID: {
				{Title: "Cluster Operator Status", Summary: "3 operators degraded"},
			},
		}

		content, err := m.renderClusterReportsTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Cluster Operator Status")
		assert.Contains(t, content, "3 operators degraded")
	})
}

func TestSortedClusterIDs(t *testing.T) {
	t.Run("returns sorted cluster IDs scoped to incident", func(t *testing.T) {
		m := createTestModel()
		c1 := "1a2b3c4dfakeidtest9i0j"
		c2 := "5m6n7o8pfakeidtest3u4v"
		c3 := "9z8y7x6wfakeidtest1r0q"
		m.selectedIncident = &pagerduty.Incident{APIObject: pagerduty.APIObject{ID: "INC001"}}
		m.incidentClusterMap = map[string][]string{"INC001": {c3, c1, c2}}
		m.clusterCache = map[string]*ocm.ClusterInfo{
			c1: {ID: c1},
			c2: {ID: c2},
			c3: {ID: c3},
		}

		ids := m.sortedClusterIDs()
		assert.Equal(t, []string{c1, c2, c3}, ids)
	})
}

func TestCacheCleanup_OnClearSelectedIncident(t *testing.T) {
	t.Run("clearSelectedIncident clears OCM caches", func(t *testing.T) {
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "Q123"},
		}
		m.clusterCache = map[string]*ocm.ClusterInfo{
			"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {ID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g"},
		}
		m.serviceLogCache = map[string][]ocm.ServiceLog{
			"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {{Summary: "log"}},
		}
		m.clusterReportCache = map[string][]ocm.ClusterReport{
			"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {{Title: "report"}},
		}
		m.limitedSupportCache = map[string][]ocm.LimitedSupportReason{
			"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {{Summary: "reason"}},
		}

		m.clearSelectedIncident("test cleanup")

		assert.Empty(t, m.clusterCache)
		assert.Empty(t, m.serviceLogCache)
		assert.Empty(t, m.clusterReportCache)
		assert.Empty(t, m.limitedSupportCache)
	})
}
