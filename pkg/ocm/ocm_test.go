package ocm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockClient_GetCluster(t *testing.T) {
	t.Run("returns cluster info for known cluster", func(t *testing.T) {
		mock := NewMockClient()
		mock.Clusters["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"] = &ClusterInfo{
			ID:            "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
			ExternalID:    "00000000-fake-uuid-test-999999999999",
			Name:          "fake-osd-webapp",
			DisplayName:   "fake-osd-webapp.7x9k.p1.example.org",
			State:         "ready",
			Region:        "us-east-1",
			CloudProvider: "aws",
			Version:       "4.16.5",
			Hypershift:    false,
			CCS:           true,
			Organization:  "Fake Aeronautical Ltd",
		}

		info, err := mock.GetCluster(context.Background(), "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")

		assert.NoError(t, err)
		assert.Equal(t, "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", info.ID)
		assert.Equal(t, "fake-osd-webapp.7x9k.p1.example.org", info.DisplayName)
		assert.Equal(t, "us-east-1", info.Region)
		assert.Equal(t, "aws", info.CloudProvider)
		assert.True(t, info.CCS)
	})

	t.Run("returns error for unknown cluster", func(t *testing.T) {
		mock := NewMockClient()

		_, err := mock.GetCluster(context.Background(), "nonexistent")

		assert.Error(t, err)
	})
}

func TestMockClient_GetServiceLogs(t *testing.T) {
	t.Run("returns service logs for known cluster", func(t *testing.T) {
		mock := NewMockClient()
		mock.ServiceLogs["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"] = []ServiceLog{
			{
				Timestamp:   "2026-06-01T10:00:00Z",
				Severity:    "Warning",
				ServiceName: "SREManualAction",
				Summary:     "Cluster entered limited support due to unsupported configuration",
				Description: "Customer replaced default IngressController with custom configuration that removed required SRE annotations.",
				ClusterID:   "1q2w3e4rfakeidtest9o0p1a2s3d4f5g",
				ClusterUUID: "00000000-fake-uuid-test-999999999999",
			},
		}

		logs, err := mock.GetServiceLogs(context.Background(), "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", "00000000-fake-uuid-test-999999999999")

		assert.NoError(t, err)
		assert.Len(t, logs, 1)
		assert.Equal(t, "Cluster entered limited support due to unsupported configuration", logs[0].Summary)
		assert.Equal(t, "Customer replaced default IngressController with custom configuration that removed required SRE annotations.", logs[0].Description)
	})

	t.Run("returns empty list for unknown cluster", func(t *testing.T) {
		mock := NewMockClient()

		logs, err := mock.GetServiceLogs(context.Background(), "nonexistent", "")

		assert.NoError(t, err)
		assert.Empty(t, logs)
	})
}

func TestMockClient_GetLimitedSupportHistory(t *testing.T) {
	t.Run("returns limited support reasons for known cluster", func(t *testing.T) {
		mock := NewMockClient()
		mock.LimitedSupport["1q2w3e4rfakeidtest9o0p1a2s3d4f5g"] = []LimitedSupportReason{
			{
				ID:            "ls-001",
				Summary:       "Customer modification",
				Details:       "Customer modified cluster networking",
				DetectionType: "manual",
				CreatedAt:     "2026-05-15T08:00:00Z",
			},
		}

		reasons, err := mock.GetLimitedSupportHistory(context.Background(), "1q2w3e4rfakeidtest9o0p1a2s3d4f5g")

		assert.NoError(t, err)
		assert.Len(t, reasons, 1)
		assert.Equal(t, "Customer modification", reasons[0].Summary)
	})

	t.Run("returns empty list for unknown cluster", func(t *testing.T) {
		mock := NewMockClient()

		reasons, err := mock.GetLimitedSupportHistory(context.Background(), "nonexistent")

		assert.NoError(t, err)
		assert.Empty(t, reasons)
	})
}

func TestClientInterface(t *testing.T) {
	t.Run("mock implements OCMClient interface", func(t *testing.T) {
		var _ OCMClient = (*MockClient)(nil)
	})
}

func TestMockClient_Close(t *testing.T) {
	t.Run("close is a no-op", func(t *testing.T) {
		mock := NewMockClient()
		mock.Close()
	})
}

func TestSanitizeSearchValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no quotes", "abc123", "abc123"},
		{"single quote", "ab'c", "ab''c"},
		{"multiple quotes", "a'b'c", "a''b''c"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeSearchValue(tt.input))
		})
	}
}

func TestClusterIDPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"uuid format", "e7c5363a-fake-uuid-test-edf99fc3ea25", true},
		{"alphanumeric", "1q2w3e4rfakeidtest9o0p1a2s3d4f5g", true},
		{"with underscore", "cluster_id_123", true},
		{"empty", "", false},
		{"sql injection", "'; DROP TABLE --", false},
		{"spaces", "cluster id", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, clusterIDPattern.MatchString(tt.input))
		})
	}
}

func TestValidClusterID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"uuid format", "e7c5363a-fake-uuid-test-edf99fc3ea25", true},
		{"alphanumeric with underscore", "cluster_id_123", true},
		{"empty", "", false},
		{"argument injection with flag", "abc --evil-flag x", false},
		{"spaces", "cluster id", false},
		{"shell metacharacters", "abc;rm -rf", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, ValidClusterID(tt.input))
		})
	}
}

func TestLoadMockClientFromFixtures(t *testing.T) {
	t.Run("loads all fixture types from directory", func(t *testing.T) {
		mock, err := LoadMockClientFromFixtures("../../testdata/fixtures")

		assert.NoError(t, err)
		assert.NotEmpty(t, mock.Clusters, "should load cluster fixtures")
		assert.NotEmpty(t, mock.ServiceLogs, "should load service log fixtures")
		assert.NotEmpty(t, mock.LimitedSupport, "should load limited support fixtures")
	})

	t.Run("returns empty mock for nonexistent directory", func(t *testing.T) {
		mock, err := LoadMockClientFromFixtures("/nonexistent/path")

		assert.NoError(t, err)
		assert.Empty(t, mock.Clusters)
	})

	t.Run("handles malformed cluster JSON", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "clusters.json"), []byte("{invalid json}"), 0644))
		_, err := LoadMockClientFromFixtures(dir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loading cluster fixtures")
	})

	t.Run("handles malformed service log JSON", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "clusters.json"), []byte("{}"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "servicelogs.json"), []byte("not json"), 0644))
		_, err := LoadMockClientFromFixtures(dir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loading service log fixtures")
	})

	t.Run("handles malformed limited support JSON", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "clusters.json"), []byte("{}"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "servicelogs.json"), []byte("{}"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "limitedsupport.json"), []byte("[bad]"), 0644))
		_, err := LoadMockClientFromFixtures(dir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loading limited support fixtures")
	})

	t.Run("cluster fixture data is correct", func(t *testing.T) {
		mock, err := LoadMockClientFromFixtures("../../testdata/fixtures")
		assert.NoError(t, err)

		info, getErr := mock.GetCluster(context.Background(), "e7c5363a-fake-uuid-test-edf99fc3ea25")
		assert.NoError(t, getErr)
		assert.Equal(t, "fake-osd-webapp.7x9k.p1.example.org", info.DisplayName)
		assert.Equal(t, "us-east-1", info.Region)
		assert.True(t, info.CCS)
	})
}
