package backplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListReports(t *testing.T) {
	reports := listReportsResponse{
		Reports: []ReportSummary{
			{ReportID: "rpt-1", Summary: "test report", CreatedAt: "2026-06-01T00:00:00Z"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/backplane/cluster/test-cluster/reports", r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("last"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reports)
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	result, err := client.ListReports(context.Background(), "test-cluster")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "rpt-1", result[0].ReportID)
	assert.Equal(t, "test report", result[0].Summary)
}

func TestClient_GetReport(t *testing.T) {
	report := Report{
		ReportID:  "rpt-1",
		Summary:   "test report",
		CreatedAt: "2026-06-01T00:00:00Z",
		Data:      "dGVzdCBkYXRh",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/backplane/cluster/test-cluster/reports/rpt-1", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(report)
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	result, err := client.GetReport(context.Background(), "test-cluster", "rpt-1")
	require.NoError(t, err)
	assert.Equal(t, "rpt-1", result.ReportID)
	assert.Equal(t, "dGVzdCBkYXRh", result.Data)
}

func TestClient_ListReports_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	result, err := client.ListReports(context.Background(), "test-cluster")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestClient_EmptyReports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(listReportsResponse{Reports: []ReportSummary{}})
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	result, err := client.ListReports(context.Background(), "test-cluster")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestClient_TokenError(t *testing.T) {
	cfg := &Config{URL: "https://unused.example.com"}
	client := NewClient(cfg, func() (string, error) {
		return "", fmt.Errorf("token expired")
	})

	result, err := client.ListReports(context.Background(), "test-cluster")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token expired")
}

func TestClient_InvalidJSON_ListReports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	result, err := client.ListReports(context.Background(), "test-cluster")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode reports response")
}

func TestClient_InvalidJSON_GetReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	result, err := client.GetReport(context.Background(), "test-cluster", "rpt-1")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode report")
}

func TestNewClient_WithProxy(t *testing.T) {
	proxyURL := "http://proxy.example.com:3128"
	cfg := &Config{
		URL:      "https://backplane.example.com",
		ProxyURL: &proxyURL,
		Govcloud: false,
	}
	client := NewClient(cfg, func() (string, error) { return "token", nil })
	assert.NotNil(t, client)
}

func TestNewClient_GovcloudSkipsProxy(t *testing.T) {
	proxyURL := "http://proxy.example.com:3128"
	cfg := &Config{
		URL:      "https://backplane.example.com",
		ProxyURL: &proxyURL,
		Govcloud: true,
	}
	client := NewClient(cfg, func() (string, error) { return "token", nil })
	assert.NotNil(t, client)
}

func TestNewClient_NilProxy(t *testing.T) {
	cfg := &Config{
		URL:      "https://backplane.example.com",
		ProxyURL: nil,
	}
	client := NewClient(cfg, func() (string, error) { return "token", nil })
	assert.NotNil(t, client)
}

func TestClient_GetReport_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	cfg := &Config{URL: server.URL}
	client := NewClient(cfg, func() (string, error) { return "test-token", nil })

	result, err := client.GetReport(context.Background(), "test-cluster", "rpt-missing")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 404")
}
