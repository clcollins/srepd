package ocm

import (
	"context"
	"fmt"
)

// MockClient implements OCMClient for testing and dev mode.
type MockClient struct {
	Clusters       map[string]*ClusterInfo
	ServiceLogs    map[string][]ServiceLog
	LimitedSupport map[string][]LimitedSupportReason
}

// NewMockClient creates a MockClient with initialized maps.
func NewMockClient() *MockClient {
	return &MockClient{
		Clusters:       make(map[string]*ClusterInfo),
		ServiceLogs:    make(map[string][]ServiceLog),
		LimitedSupport: make(map[string][]LimitedSupportReason),
	}
}

func (m *MockClient) GetCluster(_ context.Context, clusterID string) (*ClusterInfo, error) {
	info, ok := m.Clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", clusterID)
	}
	return info, nil
}

func (m *MockClient) GetServiceLogs(_ context.Context, clusterID, _ string) ([]ServiceLog, error) {
	logs, ok := m.ServiceLogs[clusterID]
	if !ok {
		return []ServiceLog{}, nil
	}
	return logs, nil
}

func (m *MockClient) GetLimitedSupportHistory(_ context.Context, clusterID string) ([]LimitedSupportReason, error) {
	reasons, ok := m.LimitedSupport[clusterID]
	if !ok {
		return []LimitedSupportReason{}, nil
	}
	return reasons, nil
}

func (m *MockClient) GetAccessToken() (string, error) {
	return "mock-access-token", nil
}

func (m *MockClient) GetBackplaneURL() (string, error) {
	return "https://mock-backplane.example.com", nil
}

func (m *MockClient) Close() {}
