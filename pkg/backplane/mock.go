package backplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MockClient implements BackplaneClient for testing and dev mode.
type MockClient struct {
	Reports map[string][]ReportSummary
}

// NewMockClient creates a MockClient with initialized maps.
func NewMockClient() *MockClient {
	return &MockClient{
		Reports: make(map[string][]ReportSummary),
	}
}

func (m *MockClient) ListReports(_ context.Context, clusterID string) ([]ReportSummary, error) {
	reports, ok := m.Reports[clusterID]
	if !ok {
		return []ReportSummary{}, nil
	}
	return reports, nil
}

func (m *MockClient) GetReport(_ context.Context, clusterID, reportID string) (*Report, error) {
	reports, ok := m.Reports[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", clusterID)
	}
	for _, r := range reports {
		if r.ReportID == reportID {
			return &Report{
				ReportID:  r.ReportID,
				Summary:   r.Summary,
				CreatedAt: r.CreatedAt,
				Data:      "mock report data",
			}, nil
		}
	}
	return nil, fmt.Errorf("report %q not found for cluster %q", reportID, clusterID)
}

type fixtureReport struct {
	ReportID  string `json:"report_id"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

// LoadMockClientFromFixtures creates a MockClient populated with fixture data.
func LoadMockClientFromFixtures(dir string) (*MockClient, error) {
	mock := NewMockClient()

	path := filepath.Join(dir, "clusterreports.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return mock, nil
		}
		return mock, fmt.Errorf("loading cluster report fixtures: %w", err)
	}

	var reports map[string][]fixtureReport
	if err := json.Unmarshal(data, &reports); err != nil {
		return mock, fmt.Errorf("parsing cluster report fixtures: %w", err)
	}

	for id, frs := range reports {
		var summaries []ReportSummary
		for _, fr := range frs {
			summaries = append(summaries, ReportSummary(fr))
		}
		mock.Reports[id] = summaries
	}

	return mock, nil
}
