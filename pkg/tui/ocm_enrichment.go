package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/backplane"
	"github.com/clcollins/srepd/pkg/ocm"
)

const ocmAPITimeout = 30 * time.Second

type clusterInfoMsg struct {
	clusterID string
	info      *ocm.ClusterInfo
	err       error
}

type ocmServiceLogsMsg struct {
	clusterID string
	logs      []ocm.ServiceLog
	err       error
}

type limitedSupportMsg struct {
	clusterID string
	reasons   []ocm.LimitedSupportReason
	err       error
}

type clusterReportsMsg struct {
	clusterID string
	reports   []backplane.Report
	err       error
}

// enrichClusters dispatches phase 1 (GetCluster) for each cluster ID.
// Phase 2 (service logs, reports, limited support) is dispatched from
// the clusterInfoMsg handler after the internal OCM ID is resolved.
func enrichClusters(client ocm.OCMClient, clusterIDs []string, devMode bool) []tea.Cmd {
	if client == nil || len(clusterIDs) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, id := range clusterIDs {
		cid := id
		cmd := getClusterInfo(client, cid)
		// In dev mode, delay enrichment by 1s to simulate the real-world
		// async flow where alerts arrive first and OCM data follows.
		// Without this, fixture data enriches instantly and the SRE
		// never sees the service name change or the flash notification.
		if devMode {
			delayedCmd := cmd
			cmds = append(cmds, tea.Tick(time.Second, func(time.Time) tea.Msg {
				return delayedCmd()
			}))
		} else {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func getClusterInfo(client ocm.OCMClient, clusterID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ocmAPITimeout)
		defer cancel()
		log.Debug("ocm.GetCluster", "cluster_id", clusterID)
		info, err := client.GetCluster(ctx, clusterID)
		return clusterInfoMsg{clusterID: clusterID, info: info, err: err}
	}
}

func getOCMServiceLogs(client ocm.OCMClient, clusterID, externalID, cacheKey string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ocmAPITimeout)
		defer cancel()
		log.Debug("ocm.GetServiceLogs", "cluster_id", clusterID)
		logs, err := client.GetServiceLogs(ctx, clusterID, externalID)
		log.Debug("ocm.GetServiceLogs", "cluster_id", clusterID, "count", len(logs))
		return ocmServiceLogsMsg{clusterID: cacheKey, logs: logs, err: err}
	}
}

func getLimitedSupportHistory(client ocm.OCMClient, clusterID, cacheKey string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ocmAPITimeout)
		defer cancel()
		log.Debug("ocm.GetLimitedSupportHistory", "cluster_id", clusterID)
		reasons, err := client.GetLimitedSupportHistory(ctx, clusterID)
		log.Debug("ocm.GetLimitedSupportHistory", "cluster_id", clusterID, "count", len(reasons))
		return limitedSupportMsg{clusterID: cacheKey, reasons: reasons, err: err}
	}
}

func getClusterReports(client backplane.BackplaneClient, clusterID, cacheKey string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		log.Debug("backplane.ListReports", "cluster_id", clusterID)
		summaries, err := client.ListReports(ctx, clusterID)
		if err != nil {
			log.Debug("backplane.ListReports failed", "cluster_id", clusterID, "error", err)
			return clusterReportsMsg{clusterID: cacheKey, err: err}
		}
		log.Debug("backplane.ListReports", "cluster_id", clusterID, "count", len(summaries))

		var reports []backplane.Report
		for _, s := range summaries {
			report, err := client.GetReport(ctx, clusterID, s.ReportID)
			if err != nil {
				log.Debug("backplane.GetReport failed", "cluster_id", clusterID, "report_id", s.ReportID, "error", err)
				reports = append(reports, backplane.Report{
					ReportID:  s.ReportID,
					Summary:   s.Summary,
					CreatedAt: s.CreatedAt,
				})
				continue
			}
			reports = append(reports, *report)
		}
		log.Debug("backplane.GetReports done", "cluster_id", clusterID, "count", len(reports))
		return clusterReportsMsg{clusterID: cacheKey, reports: reports, err: nil}
	}
}
