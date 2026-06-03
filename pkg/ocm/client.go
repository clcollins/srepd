package ocm

import (
	"fmt"

	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	slv1 "github.com/openshift-online/ocm-sdk-go/servicelogs/v1"

	ocmconfig "github.com/openshift-online/ocm-common/pkg/ocm/config"
	ocmconn "github.com/openshift-online/ocm-common/pkg/ocm/connection-builder"

	"github.com/charmbracelet/log"
)

// Client wraps the OCM SDK connection for cluster enrichment.
type Client struct {
	conn *sdk.Connection
}

// NewClient creates a real OCM client using the standard config file.
// Returns nil client and nil error if OCM is not configured.
func NewClient(agentVersion string) (*Client, error) {
	cfg, err := ocmconfig.Load()
	if err != nil {
		log.Debug("ocm.NewClient", "msg", "OCM config not found", "error", err)
		return nil, nil
	}

	if cfg == nil {
		log.Debug("ocm.NewClient", "msg", "OCM config is nil")
		return nil, nil
	}

	conn, err := ocmconn.NewConnection().
		Config(cfg).
		AsAgent("srepd/" + agentVersion).
		Build()
	if err != nil {
		return nil, fmt.Errorf("ocm connection failed: %w", err)
	}

	log.Debug("ocm.NewClient", "msg", "connected to OCM")
	return &Client{conn: conn}, nil
}

func (c *Client) GetCluster(clusterID string) (*ClusterInfo, error) {
	log.Debug("ocm.GetCluster", "cluster_id", clusterID)

	response, err := c.conn.ClustersMgmt().V1().Clusters().List().
		Search(fmt.Sprintf("id = '%s' or external_id = '%s' or name = '%s'", clusterID, clusterID, clusterID)).
		Size(1).
		Send()
	if err != nil {
		return nil, fmt.Errorf("cluster search failed: %w", err)
	}

	if response.Total() == 0 {
		return nil, fmt.Errorf("cluster %q not found", clusterID)
	}

	cluster := response.Items().Get(0)
	info := &ClusterInfo{
		ID:            cluster.ID(),
		ExternalID:    cluster.ExternalID(),
		Name:          cluster.Name(),
		DisplayName:   cluster.Name(),
		State:         string(cluster.State()),
		CloudProvider: cluster.CloudProvider().ID(),
		Version:       cluster.OpenshiftVersion(),
	}

	if cluster.Region() != nil {
		info.Region = cluster.Region().ID()
	}
	if cluster.Hypershift() != nil {
		info.Hypershift = cluster.Hypershift().Enabled()
	}
	if cluster.CCS() != nil {
		info.CCS = cluster.CCS().Enabled()
	}
	if cluster.Subscription() != nil {
		info.Organization = cluster.Subscription().ID()
	}

	return info, nil
}

func (c *Client) GetServiceLogs(clusterID, externalID string) ([]ServiceLog, error) {
	log.Debug("ocm.GetServiceLogs", "cluster_id", clusterID)

	searchQuery := fmt.Sprintf("cluster_uuid = '%s' or cluster_id = '%s'", externalID, clusterID)
	if externalID == "" {
		searchQuery = fmt.Sprintf("cluster_id = '%s'", clusterID)
	}

	response, err := c.conn.ServiceLogs().V1().Clusters().ClusterLogs().List().
		Search(searchQuery).
		Parameter("orderBy", "timestamp desc").
		Size(50).
		Send()
	if err != nil {
		return nil, fmt.Errorf("service log fetch failed: %w", err)
	}

	var logs []ServiceLog
	response.Items().Each(func(entry *slv1.LogEntry) bool {
		logs = append(logs, ServiceLog{
			Timestamp:    entry.Timestamp().String(),
			Severity:     string(entry.Severity()),
			ServiceName:  entry.ServiceName(),
			Summary:      entry.Summary(),
			Description:  entry.Description(),
			ClusterID:    entry.ClusterID(),
			ClusterUUID:  entry.ClusterUUID(),
			InternalOnly: entry.InternalOnly(),
		})
		return true
	})

	log.Debug("ocm.GetServiceLogs", "cluster_id", clusterID, "count", len(logs))
	return logs, nil
}

func (c *Client) GetClusterReports(clusterID string) ([]ClusterReport, error) {
	log.Debug("ocm.GetClusterReports", "cluster_id", clusterID)
	// Cluster reports are not directly available as a standalone API in the OCM SDK.
	// This would need to call the cluster-context pattern from osdctl.
	// For now, return empty — can be implemented when the API is identified.
	return []ClusterReport{}, nil
}

func (c *Client) GetLimitedSupportHistory(clusterID string) ([]LimitedSupportReason, error) {
	log.Debug("ocm.GetLimitedSupportHistory", "cluster_id", clusterID)

	response, err := c.conn.ClustersMgmt().V1().Clusters().Cluster(clusterID).
		LimitedSupportReasons().
		List().
		Send()
	if err != nil {
		return nil, fmt.Errorf("limited support fetch failed: %w", err)
	}

	var reasons []LimitedSupportReason
	response.Items().Each(func(reason *cmv1.LimitedSupportReason) bool {
		reasons = append(reasons, LimitedSupportReason{
			ID:            reason.ID(),
			Summary:       reason.Summary(),
			Details:       reason.Details(),
			DetectionType: string(reason.DetectionType()),
			CreatedAt:     reason.CreationTimestamp().String(),
		})
		return true
	})

	log.Debug("ocm.GetLimitedSupportHistory", "cluster_id", clusterID, "count", len(reasons))
	return reasons, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close() //nolint:errcheck
	}
}
