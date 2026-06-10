package backplane

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
)

const defaultTimeout = 30 * time.Second

type listReportsResponse struct {
	Reports []ReportSummary `json:"reports"`
}

// Client is the HTTP-based backplane client.
type Client struct {
	config     *Config
	tokenFunc  func() (string, error)
	httpClient *http.Client
}

// NewClient creates a BackplaneClient from config and a token provider function.
func NewClient(cfg *Config, tokenFunc func() (string, error)) BackplaneClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if cfg.ProxyURL != nil && *cfg.ProxyURL != "" && !cfg.Govcloud {
		proxyURL, err := url.Parse(*cfg.ProxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			log.Debug("backplane.NewClient", "proxy", "configured")
		} else {
			log.Warn("backplane.NewClient", "msg", "invalid proxy URL, proceeding without proxy", "error", err)
		}
	}

	log.Debug("backplane.NewClient", "url", cfg.URL, "govcloud", cfg.Govcloud)

	return &Client{
		config:    cfg,
		tokenFunc: tokenFunc,
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
	}
}

func (c *Client) ListReports(ctx context.Context, clusterID string) ([]ReportSummary, error) {
	endpoint := fmt.Sprintf("%s/backplane/cluster/%s/reports?last=10", c.config.URL, clusterID)
	log.Debug("backplane.ListReports", "cluster_id", clusterID)

	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		log.Warn("backplane.ListReports failed", "cluster_id", clusterID, "error", err)
		return nil, fmt.Errorf("list reports for %s: %w", clusterID, err)
	}

	var resp listReportsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Warn("backplane.ListReports decode failed", "cluster_id", clusterID, "error", err)
		return nil, fmt.Errorf("decode reports response for %s: %w", clusterID, err)
	}

	log.Debug("backplane.ListReports", "cluster_id", clusterID, "count", len(resp.Reports))
	return resp.Reports, nil
}

func (c *Client) GetReport(ctx context.Context, clusterID, reportID string) (*Report, error) {
	endpoint := fmt.Sprintf("%s/backplane/cluster/%s/reports/%s", c.config.URL, clusterID, reportID)
	log.Debug("backplane.GetReport", "cluster_id", clusterID, "report_id", reportID)

	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		log.Warn("backplane.GetReport failed", "cluster_id", clusterID, "report_id", reportID, "error", err)
		return nil, fmt.Errorf("get report %s for %s: %w", reportID, clusterID, err)
	}

	var report Report
	if err := json.Unmarshal(body, &report); err != nil {
		log.Warn("backplane.GetReport decode failed", "cluster_id", clusterID, "report_id", reportID, "error", err)
		return nil, fmt.Errorf("decode report %s: %w", reportID, err)
	}

	log.Debug("backplane.GetReport", "cluster_id", clusterID, "report_id", reportID, "summary", report.Summary)
	return &report, nil
}

func (c *Client) doRequest(ctx context.Context, endpoint string) ([]byte, error) {
	token, err := c.tokenFunc()
	if err != nil {
		log.Warn("backplane.doRequest", "msg", "token acquisition failed", "error", err)
		return nil, fmt.Errorf("get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	log.Debug("backplane.doRequest", "endpoint", endpoint)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Warn("backplane.doRequest", "msg", "request failed", "endpoint", endpoint, "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	log.Debug("backplane.doRequest", "endpoint", endpoint, "status", resp.StatusCode, "body_bytes", len(body))

	if resp.StatusCode != http.StatusOK {
		log.Warn("backplane.doRequest", "msg", "unexpected status", "endpoint", endpoint, "status", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
