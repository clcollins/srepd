package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/openshift-online/srepd/pkg/ai/policy"
	"github.com/openshift-online/srepd/pkg/ai/tools"
	"github.com/openshift-online/srepd/pkg/delta"
	"github.com/openshift-online/srepd/pkg/ocm"
	"github.com/openshift-online/srepd/pkg/pd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIncident_HappyPath(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_incident")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"P123ABC"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "P123ABC")
}

func TestGetIncident_NotFound(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_incident")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"err"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "incident not found")
	assert.NotContains(t, result, "pd.Mock()")
}

func TestGetAlerts_HappyPath(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_alerts")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"incident_id":"P123ABC"}`))
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestGetAlerts_NotFound(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_alerts")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"incident_id":"err"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "alerts not found")
}

func TestGetNotes_HappyPath(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_notes")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"incident_id":"P123ABC"}`))
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestGetNotes_NotFound(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_notes")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"incident_id":"err"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "notes not found")
}

func TestListQueue_HappyPath(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	pdConfig := &pd.Config{Client: mock}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, pdConfig))

	tool := findTool(t, reg, "list_queue")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestGetClusterInfo_HappyPath(t *testing.T) {
	mock := ocm.NewMockClient()
	mock.Clusters["cluster-123"] = &ocm.ClusterInfo{
		ID:            "cluster-123",
		Name:          "test-cluster",
		State:         "ready",
		Region:        "us-east-1",
		CloudProvider: "aws",
		Version:       "4.15.0",
	}

	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterOCMTools(reg, mock))

	tool := findTool(t, reg, "get_cluster_info")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"cluster_id":"cluster-123"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "test-cluster")
	assert.Contains(t, result, "us-east-1")
}

func TestGetClusterInfo_NotFound(t *testing.T) {
	mock := ocm.NewMockClient()
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterOCMTools(reg, mock))

	tool := findTool(t, reg, "get_cluster_info")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"cluster_id":"nonexistent"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "cluster not found")
}

func TestGetServiceLogs_HappyPath(t *testing.T) {
	mock := ocm.NewMockClient()
	mock.ServiceLogs["cluster-123"] = []ocm.ServiceLog{
		{Timestamp: "2026-07-01T00:00:00Z", Summary: "Log 1"},
		{Timestamp: "2026-07-02T00:00:00Z", Summary: "Log 2"},
	}

	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterOCMTools(reg, mock))

	tool := findTool(t, reg, "get_service_logs")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"cluster_id":"cluster-123"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Log 1")
}

func TestGetServiceLogs_Truncation(t *testing.T) {
	mock := ocm.NewMockClient()
	logs := make([]ocm.ServiceLog, 10)
	for i := range logs {
		logs[i] = ocm.ServiceLog{Summary: "Log entry"}
	}
	mock.ServiceLogs["cluster-123"] = logs

	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterOCMTools(reg, mock))

	tool := findTool(t, reg, "get_service_logs")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"cluster_id":"cluster-123"}`))
	require.NoError(t, err)

	var parsed []ocm.ServiceLog
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.LessOrEqual(t, len(parsed), 5, "service logs capped at 5")
}

func TestGetLimitedSupport_HappyPath(t *testing.T) {
	mock := ocm.NewMockClient()
	mock.LimitedSupport["cluster-123"] = []ocm.LimitedSupportReason{
		{ID: "ls-1", Summary: "Node not ready"},
	}

	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterOCMTools(reg, mock))

	tool := findTool(t, reg, "get_limited_support")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"cluster_id":"cluster-123"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Node not ready")
}

func TestGetLimitedSupport_NotFound(t *testing.T) {
	mock := ocm.NewMockClient()
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterOCMTools(reg, mock))

	tool := findTool(t, reg, "get_limited_support")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"cluster_id":"nonexistent"}`))
	require.NoError(t, err)
	// OCM mock returns empty slice for unknown clusters (not an error)
	assert.Equal(t, "[]", result)
}

func TestHandler_InvalidJSON(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_incident")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{invalid}`))
	require.NoError(t, err)
	assert.Contains(t, result, "invalid input")
}

func TestHandler_TruncationMarker(t *testing.T) {
	result := tools.Truncate(strings.Repeat("x", 9000), tools.MaxResponseBytes)
	assert.LessOrEqual(t, len(result), tools.MaxResponseBytes)
	assert.True(t, strings.HasSuffix(result, "[truncated]"))
}

func TestHandler_NoTruncationSmallOutput(t *testing.T) {
	result := tools.Truncate("small output", tools.MaxResponseBytes)
	assert.Equal(t, "small output", result)
	assert.NotContains(t, result, "[truncated]")
}

func TestHandler_ErrorDoesNotLeakInternals(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_incident")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"err"}`))
	require.NoError(t, err)
	assert.NotContains(t, result, "goroutine")
	assert.NotContains(t, result, "panic")
}

func TestTruncate_SmallMaxBytes_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		result := tools.Truncate("hello world", 5)
		assert.LessOrEqual(t, len(result), 5)
	})
	assert.NotPanics(t, func() {
		result := tools.Truncate("hello world", 0)
		assert.Equal(t, "", result)
	})
	assert.NotPanics(t, func() {
		result := tools.Truncate("hello world", 1)
		assert.LessOrEqual(t, len(result), 1)
	})
}

func TestHandler_FormatErrorIncludesClass(t *testing.T) {
	mock := &pd.MockPagerDutyClient{}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterPDTools(reg, mock, nil))

	tool := findTool(t, reg, "get_incident")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"id":"err"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "incident not found")
	assert.Contains(t, result, "(", "error string should include a parenthesized error class")
}

func TestGetRecentEvents_HappyPath(t *testing.T) {
	changes := []delta.Change{
		{Kind: delta.IncidentNew, IncidentID: "P1", Summary: "New incident: Alert A"},
		{Kind: delta.StatusChanged, IncidentID: "P2", Summary: "Status changed: triggered → acknowledged"},
	}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterDeltaTools(reg, func() []delta.Change { return changes }))

	tool := findTool(t, reg, "get_recent_events")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Contains(t, result, "P1")
	assert.Contains(t, result, "P2")
	assert.Contains(t, result, "new")
	assert.Contains(t, result, "status_changed")
}

func TestGetRecentEvents_EmptyChanges(t *testing.T) {
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterDeltaTools(reg, func() []delta.Change { return nil }))

	tool := findTool(t, reg, "get_recent_events")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "[]", result)
}

func TestGetRecentEvents_WithLimit(t *testing.T) {
	var changes []delta.Change
	for i := 0; i < 10; i++ {
		changes = append(changes, delta.Change{
			Kind:       delta.IncidentNew,
			IncidentID: fmt.Sprintf("P%d", i),
			Summary:    fmt.Sprintf("Event %d", i),
		})
	}
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterDeltaTools(reg, func() []delta.Change { return changes }))

	tool := findTool(t, reg, "get_recent_events")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"limit":3}`))
	require.NoError(t, err)

	var parsed []map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Len(t, parsed, 3, "limit must cap returned events")
	assert.Equal(t, "P7", parsed[0]["incident_id"], "must return most recent events")
}

func TestGetRecentEvents_InvalidInput(t *testing.T) {
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterDeltaTools(reg, func() []delta.Change { return nil }))

	tool := findTool(t, reg, "get_recent_events")
	result, err := tool.Handler(context.Background(), json.RawMessage(`{invalid}`))
	require.NoError(t, err)
	assert.Contains(t, result, "invalid input")
}

func TestGetRecentEvents_IsClassRead(t *testing.T) {
	reg := tools.NewRegistry()
	require.NoError(t, tools.RegisterDeltaTools(reg, func() []delta.Change { return nil }))
	tool := findTool(t, reg, "get_recent_events")
	assert.Equal(t, policy.ClassRead, tool.Class)
}

// findTool finds a tool by name in the registry.
func findTool(t *testing.T, reg *tools.Registry, name string) tools.Tool {
	t.Helper()
	for _, tool := range reg.Tools() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found in registry", name)
	return tools.Tool{}
}
