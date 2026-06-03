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

// GetCluster resolves a cluster key (internal ID, external UUID, name, or
// subscription ID) to a full ClusterInfo. Uses the same two-phase lookup
// as ocm-container: subscriptions API first, then direct cluster search.
func (c *Client) GetCluster(key string) (*ClusterInfo, error) {
	log.Debug("ocm.GetCluster", "cluster_id", key)

	subsResource := c.conn.AccountsMgmt().V1().Subscriptions()
	clustersResource := c.conn.ClustersMgmt().V1().Clusters()

	// Phase 1: Search subscriptions
	subsSearch := fmt.Sprintf(
		"(display_name = '%s' or cluster_id = '%s' or external_cluster_id = '%s') and "+
			"status in ('Reserved', 'Active')",
		key, key, key,
	)
	subsResponse, err := subsResource.List().
		Search(subsSearch).
		Size(1).
		Send()
	if err != nil {
		log.Debug("ocm.GetCluster", "msg", "subscription search failed, trying cluster search", "error", err)
	}

	if err == nil && subsResponse.Total() == 1 {
		internalID, ok := subsResponse.Items().Slice()[0].GetClusterID()
		if ok {
			clusterResponse, clusterErr := clustersResource.Cluster(internalID).Get().Send()
			if clusterErr == nil {
				return clusterFromResponse(clusterResponse.Body()), nil
			}
			log.Debug("ocm.GetCluster", "msg", "cluster get by sub ID failed", "error", clusterErr)
		}
	}

	// Phase 2: Direct cluster search fallback
	clustersSearch := fmt.Sprintf(
		"id = '%s' or name = '%s' or external_id = '%s'",
		key, key, key,
	)
	clustersResponse, err := clustersResource.List().
		Search(clustersSearch).
		Size(1).
		Send()
	if err != nil {
		return nil, fmt.Errorf("cluster search failed: %w", err)
	}

	if clustersResponse.Total() == 0 {
		return nil, fmt.Errorf("cluster %q not found", key)
	}

	return clusterFromResponse(clustersResponse.Items().Slice()[0]), nil
}

func clusterFromResponse(cluster *cmv1.Cluster) *ClusterInfo {
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

	return info
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
	return []ClusterReport{}, nil
}

// GetLimitedSupportHistory retrieves limited support reasons using the
// internal OCM cluster ID (not the external UUID).
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
