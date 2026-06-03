package tui

import (
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/clcollins/srepd/pkg/ocm"
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
