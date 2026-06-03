package ocm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockClient_GetCluster(t *testing.T) {
	t.Run("returns cluster info for known cluster", func(t *testing.T) {
		mock := NewMockClient()
		mock.Clusters["1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h"] = &ClusterInfo{
			ID:            "1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h",
			ExternalID:    "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			Name:          "prod-webapp",
			DisplayName:   "prod-webapp.7x9k.p1.openshiftapps.com",
			State:         "ready",
			Region:        "us-east-1",
			CloudProvider: "aws",
			Version:       "4.16.5",
			Hypershift:    false,
			CCS:           true,
			Organization:  "Acme Industries",
		}

		info, err := mock.GetCluster("1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h")

		assert.NoError(t, err)
		assert.Equal(t, "1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h", info.ID)
		assert.Equal(t, "prod-webapp.7x9k.p1.openshiftapps.com", info.DisplayName)
		assert.Equal(t, "us-east-1", info.Region)
		assert.Equal(t, "aws", info.CloudProvider)
		assert.True(t, info.CCS)
	})

	t.Run("returns error for unknown cluster", func(t *testing.T) {
		mock := NewMockClient()

		_, err := mock.GetCluster("nonexistent")

		assert.Error(t, err)
	})
}

func TestMockClient_GetServiceLogs(t *testing.T) {
	t.Run("returns service logs for known cluster", func(t *testing.T) {
		mock := NewMockClient()
		mock.ServiceLogs["1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h"] = []ServiceLog{
			{
				Timestamp:   "2026-06-01T10:00:00Z",
				Severity:    "Warning",
				ServiceName: "SREManualAction",
				Summary:     "Cluster entered limited support due to unsupported configuration",
				Description: "Customer replaced default IngressController with custom configuration that removed required SRE annotations.",
				ClusterID:   "1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h",
				ClusterUUID: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			},
		}

		logs, err := mock.GetServiceLogs("1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h", "f47ac10b-58cc-4372-a567-0e02b2c3d479")

		assert.NoError(t, err)
		assert.Len(t, logs, 1)
		assert.Equal(t, "Cluster entered limited support due to unsupported configuration", logs[0].Summary)
		assert.Equal(t, "Customer replaced default IngressController with custom configuration that removed required SRE annotations.", logs[0].Description)
	})

	t.Run("returns empty list for unknown cluster", func(t *testing.T) {
		mock := NewMockClient()

		logs, err := mock.GetServiceLogs("nonexistent", "")

		assert.NoError(t, err)
		assert.Empty(t, logs)
	})
}

func TestMockClient_GetClusterReports(t *testing.T) {
	t.Run("returns reports for known cluster", func(t *testing.T) {
		mock := NewMockClient()
		mock.ClusterReports["1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h"] = []ClusterReport{
			{
				Title:     "Cluster Operator Status",
				Summary:   "Cluster is healthy",
				CreatedAt: "2026-06-01T10:00:00Z",
			},
		}

		reports, err := mock.GetClusterReports("1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h")

		assert.NoError(t, err)
		assert.Len(t, reports, 1)
		assert.Equal(t, "Cluster Operator Status", reports[0].Title)
	})

	t.Run("returns empty list for unknown cluster", func(t *testing.T) {
		mock := NewMockClient()

		reports, err := mock.GetClusterReports("nonexistent")

		assert.NoError(t, err)
		assert.Empty(t, reports)
	})
}

func TestMockClient_GetLimitedSupportHistory(t *testing.T) {
	t.Run("returns limited support reasons for known cluster", func(t *testing.T) {
		mock := NewMockClient()
		mock.LimitedSupport["1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h"] = []LimitedSupportReason{
			{
				ID:            "ls-001",
				Summary:       "Customer modification",
				Details:       "Customer modified cluster networking",
				DetectionType: "manual",
				CreatedAt:     "2026-05-15T08:00:00Z",
			},
		}

		reasons, err := mock.GetLimitedSupportHistory("1q2w3e4r5t6y7u8i9o0p1a2s3d4f5g6h")

		assert.NoError(t, err)
		assert.Len(t, reasons, 1)
		assert.Equal(t, "Customer modification", reasons[0].Summary)
	})

	t.Run("returns empty list for unknown cluster", func(t *testing.T) {
		mock := NewMockClient()

		reasons, err := mock.GetLimitedSupportHistory("nonexistent")

		assert.NoError(t, err)
		assert.Empty(t, reasons)
	})
}

func TestClientInterface(t *testing.T) {
	t.Run("mock implements OCMClient interface", func(t *testing.T) {
		var _ OCMClient = (*MockClient)(nil)
	})
}

func TestLoadMockClientFromFixtures(t *testing.T) {
	t.Run("loads all fixture types from directory", func(t *testing.T) {
		mock, err := LoadMockClientFromFixtures("../../testdata/fixtures")

		assert.NoError(t, err)
		assert.NotEmpty(t, mock.Clusters, "should load cluster fixtures")
		assert.NotEmpty(t, mock.ServiceLogs, "should load service log fixtures")
		assert.NotEmpty(t, mock.ClusterReports, "should load cluster report fixtures")
		assert.NotEmpty(t, mock.LimitedSupport, "should load limited support fixtures")
	})

	t.Run("returns empty mock for nonexistent directory", func(t *testing.T) {
		mock, err := LoadMockClientFromFixtures("/nonexistent/path")

		assert.NoError(t, err)
		assert.Empty(t, mock.Clusters)
	})

	t.Run("cluster fixture data is correct", func(t *testing.T) {
		mock, err := LoadMockClientFromFixtures("../../testdata/fixtures")
		assert.NoError(t, err)

		info, getErr := mock.GetCluster("e7c5363a-b69b-47bf-98ff-edf99fc3ea25")
		assert.NoError(t, getErr)
		assert.Equal(t, "Test OSD Cluster", info.DisplayName)
		assert.Equal(t, "us-east-1", info.Region)
		assert.True(t, info.CCS)
	})
}
