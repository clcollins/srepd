package backplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListReports_PathEscapesClusterID(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(listReportsResponse{Reports: []ReportSummary{}})
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	_, err := client.ListReports(context.Background(), "../../admin/endpoint")
	require.NoError(t, err)

	assert.Equal(t, "/backplane/cluster/..%2F..%2Fadmin%2Fendpoint/reports", receivedPath,
		"clusterID with path traversal must be URL-escaped in the request path")
}

func TestClient_GetReport_PathEscapesClusterIDAndReportID(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Report{ReportID: "rpt-1"})
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	_, err := client.GetReport(context.Background(), "../admin", "../../secret")
	require.NoError(t, err)

	assert.Equal(t, "/backplane/cluster/..%2Fadmin/reports/..%2F..%2Fsecret", receivedPath,
		"both clusterID and reportID with path traversal must be URL-escaped")
}

func TestClient_ListReports_NormalClusterIDUnchanged(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(listReportsResponse{Reports: []ReportSummary{}})
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	_, err := client.ListReports(context.Background(), "abc-123-def")
	require.NoError(t, err)

	assert.Equal(t, "/backplane/cluster/abc-123-def/reports", receivedPath,
		"normal clusterID should pass through unchanged")
}
