package backplane

import "context"

// ReportSummary is a summary of a single cluster report from the backplane API.
type ReportSummary struct {
	ReportID  string `json:"report_id"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

// Report is the full content of a single cluster report.
type Report struct {
	ReportID  string `json:"report_id"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
	Data      string `json:"data"`
}

// BackplaneClient defines the interface for backplane API operations.
type BackplaneClient interface {
	ListReports(ctx context.Context, clusterID string) ([]ReportSummary, error)
	GetReport(ctx context.Context, clusterID, reportID string) (*Report, error)
}
