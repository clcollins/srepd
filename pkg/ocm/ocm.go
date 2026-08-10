package ocm

import "context"

// ClusterInfo contains enriched cluster data from the OCM API.
type ClusterInfo struct {
	ID             string
	ExternalID     string
	Name           string
	DisplayName    string
	State          string
	Region         string
	CloudProvider  string
	Version        string
	Hypershift     bool
	CCS            bool
	Organization   string
	OrganizationID string
}

// ServiceLog represents a single service log entry.
type ServiceLog struct {
	Timestamp    string
	Severity     string
	ServiceName  string
	Summary      string
	Description  string
	ClusterID    string
	ClusterUUID  string
	InternalOnly bool
}

// LimitedSupportReason represents a limited support reason entry.
type LimitedSupportReason struct {
	ID            string
	Summary       string
	Details       string
	DetectionType string
	CreatedAt     string
}

// OCMClient defines the interface for OCM API operations.
type OCMClient interface {
	GetCluster(ctx context.Context, clusterID string) (*ClusterInfo, error)
	GetServiceLogs(ctx context.Context, clusterID, externalID string) ([]ServiceLog, error)
	GetLimitedSupportHistory(ctx context.Context, clusterID string) ([]LimitedSupportReason, error)
	GetAccessToken() (string, error)
	GetBackplaneURL() (string, error)
	Close()
}
