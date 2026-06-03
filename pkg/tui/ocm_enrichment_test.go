package tui

import (
	"fmt"
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

		cmd := getOCMServiceLogs(mock, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", "00000000-fake-uuid-test-999999999999", "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		msg := cmd()

		logsMsg, ok := msg.(ocmServiceLogsMsg)
		assert.True(t, ok)
		assert.NoError(t, logsMsg.err)
		assert.Len(t, logsMsg.logs, 1)
		assert.Equal(t, "Cluster entered limited support due to unsupported configuration", logsMsg.logs[0].Summary)
	})
}

func TestGetLimitedSupportHistory_Success(t *testing.T) {
	t.Run("returns limitedSupportMsg with reasons", func(t *testing.T) {
		mock := createMockOCMClient()

		cmd := getLimitedSupportHistory(mock, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
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

func TestClusterInfoMsg_ErrorTracksFailure(t *testing.T) {
	t.Run("clusterInfoMsg with error increments failure count", func(t *testing.T) {
		m := createTestModel()
		m.clusterEnrichInFlight = map[string]bool{"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": true}

		msg := clusterInfoMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			err:       fmt.Errorf("cluster not found"),
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Equal(t, 1, updated.clusterEnrichFailed["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"])
		assert.False(t, updated.clusterEnrichInFlight["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"])
	})
}

func TestServiceLogsMsg_UpdatesCache(t *testing.T) {
	t.Run("ocmServiceLogsMsg stores logs in cache", func(t *testing.T) {
		m := createTestModel()
		m.serviceLogCache = make(map[string][]ocm.ServiceLog)

		msg := ocmServiceLogsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			logs:      []ocm.ServiceLog{{Summary: "test log", Severity: "Info"}},
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.serviceLogCache, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.Len(t, updated.serviceLogCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], 1)
	})

	t.Run("ocmServiceLogsMsg with error does not populate cache", func(t *testing.T) {
		m := createTestModel()
		m.serviceLogCache = make(map[string][]ocm.ServiceLog)

		msg := ocmServiceLogsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			err:       fmt.Errorf("fetch failed"),
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Empty(t, updated.serviceLogCache)
	})
}

func TestLimitedSupportMsg_UpdatesCache(t *testing.T) {
	t.Run("limitedSupportMsg stores reasons in cache", func(t *testing.T) {
		m := createTestModel()
		m.limitedSupportCache = make(map[string][]ocm.LimitedSupportReason)

		msg := limitedSupportMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			reasons:   []ocm.LimitedSupportReason{{Summary: "test reason"}},
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.limitedSupportCache, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.Len(t, updated.limitedSupportCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], 1)
	})

	t.Run("limitedSupportMsg with error does not populate cache", func(t *testing.T) {
		m := createTestModel()
		m.limitedSupportCache = make(map[string][]ocm.LimitedSupportReason)

		msg := limitedSupportMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			err:       fmt.Errorf("fetch failed"),
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		assert.Empty(t, updated.limitedSupportCache)
	})
}

func TestClusterInfoMsg_SkipsPhase2WhenCached(t *testing.T) {
	t.Run("phase 2 skipped when service logs already cached", func(t *testing.T) {
		m := createTestModel()
		m.ocmClient = createMockOCMClient()
		m.clusterCache = make(map[string]*ocm.ClusterInfo)
		m.serviceLogCache = map[string][]ocm.ServiceLog{
			"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {{Summary: "already cached"}},
		}
		m.limitedSupportCache = map[string][]ocm.LimitedSupportReason{
			"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": {},
		}
		m.clusterEnrichInFlight = map[string]bool{"1q2w3e4rfakeidtest9o0p1a2s3d4f5g": true}

		msg := clusterInfoMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			info: &ocm.ClusterInfo{
				ID:         "internal-id-fake-0000",
				ExternalID: "00000000-fake-uuid-test-999999999999",
			},
		}

		result, _ := m.Update(msg)
		updated := result.(model)

		// Cache should still have the original service logs
		assert.Equal(t, "already cached", updated.serviceLogCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"][0].Summary)
	})
}

func TestEnrichClusters_DispatchesCommands(t *testing.T) {
	t.Run("enrichClusters returns commands for each unique cluster", func(t *testing.T) {
		mock := createMockOCMClient()

		clusterIDs := []string{"1q2w3e4rfakeidtest9o0p1a2s3d4f5g", "2a3b4c5dfakeidtest0i1j2k3l4m5n6o"}
		cmds := enrichClusters(mock, clusterIDs, false)

		assert.Len(t, cmds, 2, "should return 1 command per cluster (phase 1 GetCluster only)")
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
		m.clusterEnrichInFlight = make(map[string]bool)
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

func TestRenderServiceLogsTab_NoOCM(t *testing.T) {
	t.Run("shows OCM not connected when client is nil", func(t *testing.T) {
		m := createTestModel()
		m.serviceLogCache = make(map[string][]ocm.ServiceLog)
		m.ocmClient = nil

		content, err := m.renderServiceLogsTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "OCM not connected")
	})
}

func TestRenderLimitedSupportTab_NoOCM(t *testing.T) {
	t.Run("shows OCM not connected when client is nil", func(t *testing.T) {
		m := createTestModel()
		m.limitedSupportCache = make(map[string][]ocm.LimitedSupportReason)
		m.ocmClient = nil

		content, err := m.renderLimitedSupportTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "OCM not connected")
	})
}

func TestRenderClusterTab_Loading(t *testing.T) {
	t.Run("shows loading message when client connected but no data", func(t *testing.T) {
		m := createTestModel()
		m.clusterCache = make(map[string]*ocm.ClusterInfo)
		m.ocmClient = createMockOCMClient()

		content, err := m.renderClusterTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Loading cluster info")
	})
}

func TestRenderServiceLogsTab_Loading(t *testing.T) {
	t.Run("shows loading message when client connected but no data", func(t *testing.T) {
		m := createTestModel()
		m.serviceLogCache = make(map[string][]ocm.ServiceLog)
		m.ocmClient = createMockOCMClient()

		content, err := m.renderServiceLogsTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "Loading service logs")
	})
}

func TestRenderLimitedSupportTab_Empty(t *testing.T) {
	t.Run("shows no history when client connected but no data", func(t *testing.T) {
		m := createTestModel()
		m.limitedSupportCache = make(map[string][]ocm.LimitedSupportReason)
		m.ocmClient = createMockOCMClient()

		content, err := m.renderLimitedSupportTab()
		assert.NoError(t, err)
		assert.Contains(t, content, "No limited support history")
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

func TestCacheCleanup_ClearSelectedDoesNotTouchOCM(t *testing.T) {
	t.Run("clearSelectedIncident does NOT clear OCM caches", func(t *testing.T) {
		c1 := "1q2w3e4rfakeidtest9o0p1a2s3d4f5g"
		m := createTestModel()
		m.selectedIncident = &pagerduty.Incident{
			APIObject: pagerduty.APIObject{ID: "INC001"},
		}
		m.incidentClusterMap = map[string][]string{"INC001": {c1}}
		m.clusterCache = map[string]*ocm.ClusterInfo{c1: {ID: c1}}
		m.serviceLogCache = map[string][]ocm.ServiceLog{c1: {{Summary: "log"}}}
		m.limitedSupportCache = map[string][]ocm.LimitedSupportReason{c1: {{Summary: "reason"}}}

		m.clearSelectedIncident("refresh")

		// OCM caches should be PRESERVED on clearSelectedIncident
		assert.Contains(t, m.clusterCache, c1)
		assert.Contains(t, m.serviceLogCache, c1)
		assert.Contains(t, m.limitedSupportCache, c1)
		assert.Contains(t, m.incidentClusterMap, "INC001")
	})
}

func TestCacheCleanup_SharedClusterPreserved(t *testing.T) {
	t.Run("shared cluster preserved when one incident removed", func(t *testing.T) {
		sharedCluster := "shared0fakeidtest000000000000001"
		m := createTestModel()
		m.incidentClusterMap = map[string][]string{
			"INC001": {sharedCluster},
			"INC002": {sharedCluster},
		}
		m.clusterCache = map[string]*ocm.ClusterInfo{
			sharedCluster: {ID: sharedCluster},
		}

		m.clearOCMCacheForIncident("INC001")

		assert.Contains(t, m.clusterCache, sharedCluster, "shared cluster should be preserved")
		assert.NotContains(t, m.incidentClusterMap, "INC001")
		assert.Contains(t, m.incidentClusterMap, "INC002")
	})
}

func TestCacheCleanup_RemovedFromList(t *testing.T) {
	t.Run("OCM caches cleared when incident removed from list", func(t *testing.T) {
		c1 := "1q2w3e4rfakeidtest9o0p1a2s3d4f5g"
		c2 := "2a3b4c5dfakeidtest0i1j2k3l4m5n6o"
		m := createTestModel()
		m.incidentClusterMap = map[string][]string{
			"INC001": {c1},
			"INC002": {c2},
		}
		m.clusterCache = map[string]*ocm.ClusterInfo{
			c1: {ID: c1},
			c2: {ID: c2},
		}
		m.serviceLogCache = map[string][]ocm.ServiceLog{
			c1: {{Summary: "log c1"}},
			c2: {{Summary: "log c2"}},
		}
		m.limitedSupportCache = map[string][]ocm.LimitedSupportReason{
			c1: {{Summary: "reason c1"}},
		}

		// Simulate INC001 being removed from list
		m.clearOCMCacheForIncident("INC001")

		assert.NotContains(t, m.clusterCache, c1)
		assert.NotContains(t, m.serviceLogCache, c1)
		assert.NotContains(t, m.incidentClusterMap, "INC001")

		// INC002 preserved
		assert.Contains(t, m.clusterCache, c2)
		assert.Contains(t, m.incidentClusterMap, "INC002")
	})
}
