package ocm

import "fmt"

// MockClient implements OCMClient for testing and dev mode.
type MockClient struct {
	Clusters       map[string]*ClusterInfo
	ServiceLogs    map[string][]ServiceLog
	ClusterReports map[string][]ClusterReport
	LimitedSupport map[string][]LimitedSupportReason
}

// NewMockClient creates a MockClient with initialized maps.
func NewMockClient() *MockClient {
	return &MockClient{
		Clusters:       make(map[string]*ClusterInfo),
		ServiceLogs:    make(map[string][]ServiceLog),
		ClusterReports: make(map[string][]ClusterReport),
		LimitedSupport: make(map[string][]LimitedSupportReason),
	}
}

func (m *MockClient) GetCluster(clusterID string) (*ClusterInfo, error) {
	info, ok := m.Clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", clusterID)
	}
	return info, nil
}

func (m *MockClient) GetServiceLogs(clusterID, _ string) ([]ServiceLog, error) {
	logs, ok := m.ServiceLogs[clusterID]
	if !ok {
		return []ServiceLog{}, nil
	}
	return logs, nil
}

func (m *MockClient) GetClusterReports(clusterID string) ([]ClusterReport, error) {
	reports, ok := m.ClusterReports[clusterID]
	if !ok {
		return []ClusterReport{}, nil
	}
	return reports, nil
}

func (m *MockClient) GetLimitedSupportHistory(clusterID string) ([]LimitedSupportReason, error) {
	reasons, ok := m.LimitedSupport[clusterID]
	if !ok {
		return []LimitedSupportReason{}, nil
	}
	return reasons, nil
}

func (m *MockClient) Close() {}
