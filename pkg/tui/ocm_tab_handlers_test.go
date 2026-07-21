package tui

import (
	"errors"
	"testing"

	"github.com/clcollins/srepd/pkg/backplane"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/stretchr/testify/assert"
)

// --- ocmServiceLogsMsg (Update handler, supplements ocm_enrichment_test.go) ---

func TestOcmServiceLogsMsg_InitializesNilCache(t *testing.T) {
	t.Run("initializes serviceLogCache map when nil", func(t *testing.T) {
		m := createTestModel()
		m.serviceLogCache = nil

		msg := ocmServiceLogsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			logs:      []ocm.ServiceLog{{Summary: "new log"}},
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.NotNil(t, updated.serviceLogCache, "should initialize nil map")
		assert.Len(t, updated.serviceLogCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], 1)
		assert.NotNil(t, cmd, "should return re-render cmd")
	})
}

func TestOcmServiceLogsMsg_ErrorStoresError(t *testing.T) {
	t.Run("error stores in error map and returns re-render cmd", func(t *testing.T) {
		m := createTestModel()
		m.serviceLogCache = make(map[string][]ocm.ServiceLog)

		msg := ocmServiceLogsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			err:       errors.New("service log fetch failed"),
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Empty(t, updated.serviceLogCache, "cache should remain empty on error")
		assert.Contains(t, updated.serviceLogErrors, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.EqualError(t, updated.serviceLogErrors["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], "service log fetch failed")
		assert.NotNil(t, cmd, "should return re-render cmd on error")
	})
}

func TestOcmServiceLogsMsg_StoresEmptySlice(t *testing.T) {
	t.Run("stores empty slice in cache when no logs found", func(t *testing.T) {
		m := createTestModel()
		m.serviceLogCache = make(map[string][]ocm.ServiceLog)

		msg := ocmServiceLogsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			logs:      []ocm.ServiceLog{},
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.serviceLogCache, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.Empty(t, updated.serviceLogCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"])
		assert.NotNil(t, cmd, "should return re-render cmd")
	})
}

// --- limitedSupportMsg (Update handler, supplements ocm_enrichment_test.go) ---

func TestLimitedSupportMsg_InitializesNilCache(t *testing.T) {
	t.Run("initializes limitedSupportCache map when nil", func(t *testing.T) {
		m := createTestModel()
		m.limitedSupportCache = nil

		msg := limitedSupportMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			reasons:   []ocm.LimitedSupportReason{{Summary: "test reason"}},
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.NotNil(t, updated.limitedSupportCache, "should initialize nil map")
		assert.Len(t, updated.limitedSupportCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], 1)
		assert.NotNil(t, cmd, "should return re-render cmd")
	})
}

func TestLimitedSupportMsg_ErrorStoresError(t *testing.T) {
	t.Run("error stores in error map and returns re-render cmd", func(t *testing.T) {
		m := createTestModel()
		m.limitedSupportCache = make(map[string][]ocm.LimitedSupportReason)

		msg := limitedSupportMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			err:       errors.New("limited support fetch failed"),
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Empty(t, updated.limitedSupportCache, "cache should remain empty on error")
		assert.Contains(t, updated.limitedSupportErrors, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.EqualError(t, updated.limitedSupportErrors["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], "limited support fetch failed")
		assert.NotNil(t, cmd, "should return re-render cmd on error")
	})
}

// --- clusterReportsMsg ---

func TestClusterReportsMsg_StoresInCache(t *testing.T) {
	t.Run("stores reports in clusterReportCache", func(t *testing.T) {
		m := createTestModel()
		m.clusterReportCache = make(map[string][]backplane.Report)

		msg := clusterReportsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			reports: []backplane.Report{
				{ReportID: "R001", Summary: "test report"},
				{ReportID: "R002", Summary: "another report"},
			},
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.clusterReportCache, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.Len(t, updated.clusterReportCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], 2)
		assert.NotNil(t, cmd, "should return re-render cmd")
	})
}

func TestClusterReportsMsg_ErrorDoesNotPopulateCache(t *testing.T) {
	t.Run("error does not populate cache but stores in error map", func(t *testing.T) {
		m := createTestModel()
		m.clusterReportCache = make(map[string][]backplane.Report)

		msg := clusterReportsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			err:       errors.New("backplane reports fetch failed"),
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Empty(t, updated.clusterReportCache, "cache should remain empty on error")
		assert.Contains(t, updated.clusterReportErrors, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.EqualError(t, updated.clusterReportErrors["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], "backplane reports fetch failed")
		assert.NotNil(t, cmd, "should return re-render cmd on error")
	})
}

func TestClusterReportsMsg_InitializesNilCache(t *testing.T) {
	t.Run("initializes clusterReportCache map when nil", func(t *testing.T) {
		m := createTestModel()
		m.clusterReportCache = nil

		msg := clusterReportsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			reports:   []backplane.Report{{ReportID: "R001", Summary: "report"}},
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.NotNil(t, updated.clusterReportCache, "should initialize nil map")
		assert.Len(t, updated.clusterReportCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"], 1)
		assert.NotNil(t, cmd, "should return re-render cmd")
	})
}

func TestClusterReportsMsg_StoresEmptySlice(t *testing.T) {
	t.Run("stores empty slice when no reports found", func(t *testing.T) {
		m := createTestModel()
		m.clusterReportCache = make(map[string][]backplane.Report)

		msg := clusterReportsMsg{
			clusterID: "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			reports:   []backplane.Report{},
		}

		result, cmd := m.Update(msg)
		updated := result.(model)

		assert.Contains(t, updated.clusterReportCache, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")
		assert.Empty(t, updated.clusterReportCache["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"])
		assert.NotNil(t, cmd, "should return re-render cmd")
	})
}
