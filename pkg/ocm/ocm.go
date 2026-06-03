package ocm

// ClusterInfo contains enriched cluster data from the OCM API.
type ClusterInfo struct {
	ID            string
	ExternalID    string
	Name          string
	DisplayName   string
	State         string
	Region        string
	CloudProvider string
	Version       string
	Hypershift    bool
	CCS           bool
	Organization  string
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

// ClusterReport represents a cluster report entry.
type ClusterReport struct {
	Title     string
	Summary   string
	Details   string
	CreatedAt string
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
	GetCluster(clusterID string) (*ClusterInfo, error)
	GetServiceLogs(clusterID, externalID string) ([]ServiceLog, error)
	GetClusterReports(clusterID string) ([]ClusterReport, error)
	GetLimitedSupportHistory(clusterID string) ([]LimitedSupportReason, error)
	Close()
}
