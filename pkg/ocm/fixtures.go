package ocm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type fixtureCluster struct {
	ID            string `json:"id"`
	ExternalID    string `json:"external_id"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	State         string `json:"state"`
	Region        string `json:"region"`
	CloudProvider string `json:"cloud_provider"`
	Version       string `json:"version"`
	Hypershift    bool   `json:"hypershift"`
	CCS           bool   `json:"ccs"`
	Organization  string `json:"organization"`
}

type fixtureServiceLog struct {
	Timestamp    string `json:"timestamp"`
	Severity     string `json:"severity"`
	ServiceName  string `json:"service_name"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	ClusterID    string `json:"cluster_id"`
	ClusterUUID  string `json:"cluster_uuid"`
	InternalOnly bool   `json:"internal_only"`
}

type fixtureClusterReport struct {
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Details   string `json:"details"`
	CreatedAt string `json:"created_at"`
}

type fixtureLimitedSupport struct {
	ID            string `json:"id"`
	Summary       string `json:"summary"`
	Details       string `json:"details"`
	DetectionType string `json:"detection_type"`
	CreatedAt     string `json:"created_at"`
}

// LoadMockClientFromFixtures creates a MockClient populated with data from fixture files.
func LoadMockClientFromFixtures(dir string) (*MockClient, error) {
	mock := NewMockClient()

	if err := loadClusterFixtures(filepath.Join(dir, "clusters.json"), mock); err != nil {
		return mock, fmt.Errorf("loading cluster fixtures: %w", err)
	}
	if err := loadServiceLogFixtures(filepath.Join(dir, "servicelogs.json"), mock); err != nil {
		return mock, fmt.Errorf("loading service log fixtures: %w", err)
	}
	if err := loadClusterReportFixtures(filepath.Join(dir, "clusterreports.json"), mock); err != nil {
		return mock, fmt.Errorf("loading cluster report fixtures: %w", err)
	}
	if err := loadLimitedSupportFixtures(filepath.Join(dir, "limitedsupport.json"), mock); err != nil {
		return mock, fmt.Errorf("loading limited support fixtures: %w", err)
	}

	return mock, nil
}

func loadClusterFixtures(path string, mock *MockClient) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var clusters map[string]fixtureCluster
	if err := json.Unmarshal(data, &clusters); err != nil {
		return err
	}
	for id, fc := range clusters {
		mock.Clusters[id] = &ClusterInfo{
			ID:            fc.ID,
			ExternalID:    fc.ExternalID,
			Name:          fc.Name,
			DisplayName:   fc.DisplayName,
			State:         fc.State,
			Region:        fc.Region,
			CloudProvider: fc.CloudProvider,
			Version:       fc.Version,
			Hypershift:    fc.Hypershift,
			CCS:           fc.CCS,
			Organization:  fc.Organization,
		}
	}
	return nil
}

func loadServiceLogFixtures(path string, mock *MockClient) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var logs map[string][]fixtureServiceLog
	if err := json.Unmarshal(data, &logs); err != nil {
		return err
	}
	for id, fls := range logs {
		var serviceLogs []ServiceLog
		for _, fl := range fls {
			serviceLogs = append(serviceLogs, ServiceLog{
				Timestamp:    fl.Timestamp,
				Severity:     fl.Severity,
				ServiceName:  fl.ServiceName,
				Summary:      fl.Summary,
				Description:  fl.Description,
				ClusterID:    fl.ClusterID,
				ClusterUUID:  fl.ClusterUUID,
				InternalOnly: fl.InternalOnly,
			})
		}
		mock.ServiceLogs[id] = serviceLogs
	}
	return nil
}

func loadClusterReportFixtures(path string, mock *MockClient) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var reports map[string][]fixtureClusterReport
	if err := json.Unmarshal(data, &reports); err != nil {
		return err
	}
	for id, frs := range reports {
		var clusterReports []ClusterReport
		for _, fr := range frs {
			clusterReports = append(clusterReports, ClusterReport{
				Title:     fr.Title,
				Summary:   fr.Summary,
				Details:   fr.Details,
				CreatedAt: fr.CreatedAt,
			})
		}
		mock.ClusterReports[id] = clusterReports
	}
	return nil
}

func loadLimitedSupportFixtures(path string, mock *MockClient) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var reasons map[string][]fixtureLimitedSupport
	if err := json.Unmarshal(data, &reasons); err != nil {
		return err
	}
	for id, frs := range reasons {
		var lsReasons []LimitedSupportReason
		for _, fr := range frs {
			lsReasons = append(lsReasons, LimitedSupportReason{
				ID:            fr.ID,
				Summary:       fr.Summary,
				Details:       fr.Details,
				DetectionType: fr.DetectionType,
				CreatedAt:     fr.CreatedAt,
			})
		}
		mock.LimitedSupport[id] = lsReasons
	}
	return nil
}
