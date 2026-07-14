package tui

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/clcollins/srepd/pkg/backplane"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/stretchr/testify/assert"
)

// These are deterministic, fully-mocked View-layer render tests. They drive the
// renderClusterReportsTab pure function across all of its branches with model state
// only — no PagerDuty/OCM/backplane API calls, no host tools, no filesystem. They
// raise coverage of a previously-untested (0%) render path and lock in its output.

func TestRenderClusterReportsTab_BackplaneNoConfig(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = nil
	m.backplaneConfig = nil

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "Backplane not enabled")
}

func TestRenderClusterReportsTab_BackplaneOCMAuthPending(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = nil
	m.backplaneConfig = &backplane.Config{}
	m.ocmAuthPending = true

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "Backplane initializing")
	assert.Contains(t, content, "OCM authentication")
}

func TestRenderClusterReportsTab_BackplaneWaitingForOCM(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = nil
	m.backplaneConfig = &backplane.Config{}
	m.ocmClient = nil
	m.ocmAuthPending = false

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "Backplane initializing")
	assert.Contains(t, content, "OCM connection")
}

func TestRenderClusterReportsTab_BackplaneInitError(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = nil
	m.backplaneConfig = &backplane.Config{}
	m.ocmClient = createMockOCMClient()
	m.backplaneInitErr = fmt.Errorf("no backplane URL in config or from OCM")

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "Backplane not available")
	assert.Contains(t, content, "no backplane URL in config or from OCM")
}

func TestRenderClusterReportsTab_BackplaneInitNoStoredError(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = nil
	m.backplaneConfig = &backplane.Config{}
	m.ocmClient = createMockOCMClient()
	m.backplaneInitErr = nil

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "Backplane not available")
	assert.Contains(t, content, "check logs")
}

func TestRenderClusterReportsTab_OCMNotConnected(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = backplane.NewMockClient()
	m.clusterReportCache = map[string][]backplane.Report{}
	m.ocmClient = nil
	m.ocmAuthPending = false

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "OCM not connected")
}

func TestRenderClusterReportsTab_OCMAuthPending(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = backplane.NewMockClient()
	m.clusterReportCache = map[string][]backplane.Report{}
	m.ocmClient = nil
	m.ocmAuthPending = true

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "OCM authenticating")
}

func TestRenderClusterReportsTab_Loading(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = backplane.NewMockClient()
	m.clusterReportCache = map[string][]backplane.Report{}
	m.ocmClient = createMockOCMClient()

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "Loading cluster reports")
}

func TestRenderClusterReportsTab_EmptyReports(t *testing.T) {
	m := createTestModel()
	m.backplaneClient = backplane.NewMockClient()
	// Cache present for a cluster but with an empty report slice.
	m.clusterReportCache = map[string][]backplane.Report{
		"fake-cluster-123": {},
	}

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "No cluster reports")
}

func TestRenderClusterReportsTab_WithData(t *testing.T) {
	m := createTestModel()
	setupModelWithCluster(&m) // selectedIncident + incidentClusterMap for sortedClusterIDs
	m.clusterCache = map[string]*ocm.ClusterInfo{testClusterID: {ID: testClusterID}}
	m.backplaneClient = backplane.NewMockClient()
	encoded := base64.StdEncoding.EncodeToString([]byte("decoded report body"))
	m.clusterReportCache = map[string][]backplane.Report{
		testClusterID: {
			{ReportID: "r1", Summary: "newer report", CreatedAt: "2026-06-02T00:00:00Z", Data: encoded},
			{ReportID: "r2", Summary: "older report", CreatedAt: "2026-06-01T00:00:00Z", Data: ""},
		},
	}

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	assert.Contains(t, content, "Report 1/2")
	assert.Contains(t, content, "Report 2/2")
	assert.Contains(t, content, "newer report")
	assert.Contains(t, content, "older report")
	// base64 Data is decoded and included.
	assert.Contains(t, content, "decoded report body")
	// Reports are sorted newest-first by CreatedAt, so the newer one renders first.
	assert.Less(t, indexOf(content, "newer report"), indexOf(content, "older report"),
		"reports should be sorted newest-first")
}

func TestRenderClusterReportsTab_InvalidBase64FallsBackToRaw(t *testing.T) {
	m := createTestModel()
	setupModelWithCluster(&m)
	m.clusterCache = map[string]*ocm.ClusterInfo{testClusterID: {ID: testClusterID}}
	m.backplaneClient = backplane.NewMockClient()
	m.clusterReportCache = map[string][]backplane.Report{
		testClusterID: {
			{ReportID: "r1", Summary: "s", CreatedAt: "2026-06-01T00:00:00Z", Data: "not-valid-base64!!!"},
		},
	}

	content, err := m.renderClusterReportsTab()

	assert.NoError(t, err)
	// On decode failure the raw Data is written verbatim.
	assert.Contains(t, content, "not-valid-base64!!!")
}

// indexOf is a small helper returning the index of sub in s, or -1.
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
