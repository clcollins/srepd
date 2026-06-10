package backplane

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockClient_ImplementsInterface(t *testing.T) {
	var _ BackplaneClient = &MockClient{}
}

func TestMockClient_ListReports(t *testing.T) {
	mock := NewMockClient()
	mock.Reports["cluster-1"] = []ReportSummary{
		{ReportID: "rpt-1", Summary: "test", CreatedAt: "2026-06-01T00:00:00Z"},
	}

	reports, err := mock.ListReports(context.Background(), "cluster-1")
	require.NoError(t, err)
	assert.Len(t, reports, 1)
	assert.Equal(t, "rpt-1", reports[0].ReportID)

	empty, err := mock.ListReports(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestMockClient_GetReport(t *testing.T) {
	mock := NewMockClient()
	mock.Reports["cluster-1"] = []ReportSummary{
		{ReportID: "rpt-1", Summary: "test", CreatedAt: "2026-06-01T00:00:00Z"},
	}

	report, err := mock.GetReport(context.Background(), "cluster-1", "rpt-1")
	require.NoError(t, err)
	assert.Equal(t, "rpt-1", report.ReportID)

	_, err = mock.GetReport(context.Background(), "cluster-1", "nonexistent")
	assert.Error(t, err)

	_, err = mock.GetReport(context.Background(), "nonexistent", "rpt-1")
	assert.Error(t, err)
}

func TestLoadMockClientFromFixtures(t *testing.T) {
	fixturesDir := filepath.Join("..", "..", "testdata", "fixtures")
	mock, err := LoadMockClientFromFixtures(fixturesDir)
	require.NoError(t, err)

	reports, err := mock.ListReports(context.Background(), "cluster-osd-001")
	require.NoError(t, err)
	assert.Len(t, reports, 2)
	assert.Equal(t, "rpt-cora-001", reports[0].ReportID)
}

func TestLoadMockClientFromFixtures_MissingFile(t *testing.T) {
	mock, err := LoadMockClientFromFixtures(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, mock.Reports)
}
