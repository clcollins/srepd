package ocm

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	sdk "github.com/openshift-online/ocm-sdk-go"
	auth "github.com/openshift-online/ocm-sdk-go/authentication"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	slv1 "github.com/openshift-online/ocm-sdk-go/servicelogs/v1"

	ocmconfig "github.com/openshift-online/ocm-common/pkg/ocm/config"
	ocmconn "github.com/openshift-online/ocm-common/pkg/ocm/connection-builder"

	"github.com/charmbracelet/log"
)

const (
	productionURL = "https://api.openshift.com"
	clientID      = "ocm-cli"
)

var clusterIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Client wraps the OCM SDK connection for cluster enrichment.
type Client struct {
	conn *sdk.Connection
}

func sanitizeSearchValue(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// CheckTokens loads the OCM config and checks if valid tokens exist.
// Returns the config (with defaults applied), whether tokens are valid, and any error.
// This is a fast, non-blocking operation.
func CheckTokens() (*ocmconfig.Config, bool, error) {
	cfg, err := ocmconfig.Load()
	if err != nil {
		log.Debug("ocm.CheckTokens", "msg", "OCM config not found, creating new", "error", err)
	}
	if cfg == nil {
		cfg = new(ocmconfig.Config)
	}

	applyConfigDefaults(cfg)

	armed, reason, err := cfg.Armed()
	if err != nil {
		return cfg, false, fmt.Errorf("ocm config check failed: %w", err)
	}

	if !armed {
		log.Debug("ocm.CheckTokens", "msg", "tokens not valid", "reason", reason)
	}

	return cfg, armed, nil
}

// AuthenticateAsync performs browser-based auth code login.
// This call blocks until the user completes authentication in the browser.
// Returns the raw auth token string on success.
func AuthenticateAsync(cfg *ocmconfig.Config) (string, error) {
	cid := cfg.ClientID
	if cid == "" {
		cid = clientID
	}
	token, err := auth.InitiateAuthCode(cid)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ApplyAuthToken classifies a raw auth token and sets the appropriate
// field on the OCM config (RefreshToken or AccessToken).
func ApplyAuthToken(cfg *ocmconfig.Config, token string) {
	if ocmconfig.IsEncryptedToken(token) {
		cfg.AccessToken = ""
		cfg.RefreshToken = token
		return
	}
	parsedToken, parseErr := ocmconfig.ParseToken(token)
	if parseErr == nil {
		typ, typErr := ocmconfig.TokenType(parsedToken)
		if typErr == nil && (typ == "Refresh" || typ == "Offline") {
			cfg.RefreshToken = token
			cfg.AccessToken = ""
			return
		}
	}
	cfg.AccessToken = token
}

// NewClientFromConfig builds an OCM client from a pre-loaded config.
// The config must already have valid tokens set.
func NewClientFromConfig(cfg *ocmconfig.Config, agentVersion string) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("ocm config is nil")
	}

	conn, err := ocmconn.NewConnection().
		Config(cfg).
		AsAgent("srepd/" + agentVersion).
		WithApiUrl(productionURL).
		Build()
	if err != nil {
		return nil, fmt.Errorf("ocm connection failed: %w", err)
	}

	accessToken, refreshToken, err := conn.Tokens()
	if err == nil {
		cfg.AccessToken = accessToken
		cfg.RefreshToken = refreshToken
		if saveErr := ocmconfig.Save(cfg); saveErr != nil {
			log.Debug("ocm.NewClientFromConfig", "msg", "failed to save OCM tokens", "error", saveErr)
		}
	}

	log.Debug("ocm.NewClientFromConfig", "msg", "connected to OCM", "url", productionURL)
	return &Client{conn: conn}, nil
}

func applyConfigDefaults(cfg *ocmconfig.Config) {
	cfg.URL = productionURL
	if cfg.ClientID == "" {
		cfg.ClientID = clientID
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = sdk.DefaultTokenURL
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid"}
	}
}

// NewClient creates a real OCM client using the standard config file.
// Always connects to production OCM. If no valid tokens exist, initiates
// browser-based auth code login (same flow as ocm-container).
func NewClient(agentVersion string) (*Client, error) {
	cfg, armed, err := CheckTokens()
	if err != nil {
		return nil, err
	}

	if !armed {
		log.Debug("ocm.NewClient", "msg", "not logged into OCM, initiating browser auth")
		fmt.Fprintln(os.Stderr, "OCM tokens expired — opening browser for authentication...")
		token, authErr := AuthenticateAsync(cfg)
		if authErr != nil {
			log.Debug("ocm.NewClient", "msg", "browser auth failed or cancelled", "error", authErr)
			return nil, nil
		}
		fmt.Fprintln(os.Stderr, "OCM authentication successful.")
		ApplyAuthToken(cfg, token)
	}

	return NewClientFromConfig(cfg, agentVersion)
}

func (c *Client) GetCluster(ctx context.Context, key string) (*ClusterInfo, error) {
	log.Debug("ocm.GetCluster", "cluster_id", key)

	if !clusterIDPattern.MatchString(key) {
		return nil, fmt.Errorf("invalid cluster ID format: %q", key)
	}

	safeKey := sanitizeSearchValue(key)
	subsResource := c.conn.AccountsMgmt().V1().Subscriptions()
	clustersResource := c.conn.ClustersMgmt().V1().Clusters()

	subsSearch := fmt.Sprintf(
		"(display_name = '%s' or cluster_id = '%s' or external_cluster_id = '%s') and "+
			"status in ('Reserved', 'Active')",
		safeKey, safeKey, safeKey,
	)
	subsResponse, err := subsResource.List().
		Search(subsSearch).
		Size(1).
		SendContext(ctx)
	if err != nil {
		log.Debug("ocm.GetCluster", "msg", "subscription search failed, trying cluster search", "error", err)
	}

	if err == nil {
		log.Debug("ocm.GetCluster", "msg", "subscription search completed", "total", subsResponse.Total())
		if subsResponse.Total() == 1 && len(subsResponse.Items().Slice()) > 0 {
			internalID, ok := subsResponse.Items().Slice()[0].GetClusterID()
			log.Debug("ocm.GetCluster", "msg", "subscription found", "has_cluster_id", ok)
			if ok {
				clusterResponse, clusterErr := clustersResource.Cluster(internalID).Get().SendContext(ctx)
				if clusterErr == nil {
					return clusterFromResponse(clusterResponse.Body()), nil
				}
				log.Debug("ocm.GetCluster", "msg", "cluster get by sub ID failed", "error", clusterErr)
			}
		}
	}

	clustersSearch := fmt.Sprintf(
		"id = '%s' or name = '%s' or external_id = '%s'",
		safeKey, safeKey, safeKey,
	)
	clustersResponse, err := clustersResource.List().
		Search(clustersSearch).
		Size(1).
		SendContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("cluster search failed: %w", err)
	}

	log.Debug("ocm.GetCluster", "msg", "cluster search completed", "total", clustersResponse.Total())
	if clustersResponse.Total() == 0 || len(clustersResponse.Items().Slice()) == 0 {
		return nil, fmt.Errorf("cluster %q not found via subscription or cluster search", key)
	}

	return clusterFromResponse(clustersResponse.Items().Slice()[0]), nil
}

func clusterFromResponse(cluster *cmv1.Cluster) *ClusterInfo {
	displayName := cluster.Name()
	if cluster.DomainPrefix() != "" && cluster.DNS() != nil && cluster.DNS().BaseDomain() != "" {
		displayName = fmt.Sprintf("%s.%s", cluster.DomainPrefix(), cluster.DNS().BaseDomain())
	}

	info := &ClusterInfo{
		ID:            cluster.ID(),
		ExternalID:    cluster.ExternalID(),
		Name:          cluster.Name(),
		DisplayName:   displayName,
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

func (c *Client) GetServiceLogs(ctx context.Context, clusterID, externalID string) ([]ServiceLog, error) {
	log.Debug("ocm.GetServiceLogs", "cluster_id", clusterID, "external_id", externalID)

	request := c.conn.ServiceLogs().V1().Clusters().ClusterLogs().List().
		ClusterID(clusterID).
		ClusterUUID(externalID).
		Order("timestamp desc").
		Size(50)

	response, err := request.SendContext(ctx)
	if err != nil {
		log.Debug("ocm.GetServiceLogs error detail", "cluster_id", clusterID, "error", err.Error())
		return nil, fmt.Errorf("service log fetch failed for %s: %w", clusterID, err)
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

func (c *Client) GetLimitedSupportHistory(ctx context.Context, clusterID string) ([]LimitedSupportReason, error) {
	log.Debug("ocm.GetLimitedSupportHistory", "cluster_id", clusterID)

	response, err := c.conn.ClustersMgmt().V1().Clusters().Cluster(clusterID).
		LimitedSupportReasons().
		List().
		SendContext(ctx)
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
