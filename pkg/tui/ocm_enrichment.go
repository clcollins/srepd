package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ocm"
)

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

type clusterReportsMsg struct {
	clusterID string
	reports   []ocm.ClusterReport
	err       error
}

type limitedSupportMsg struct {
	clusterID string
	reasons   []ocm.LimitedSupportReason
	err       error
}

func enrichClusters(client ocm.OCMClient, clusterIDs []string, devMode bool) []tea.Cmd {
	if client == nil || len(clusterIDs) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, id := range clusterIDs {
		cid := id
		clusterCmds := []tea.Cmd{
			getClusterInfo(client, cid),
			getOCMServiceLogs(client, cid, ""),
			getClusterReports(client, cid),
			getLimitedSupportHistory(client, cid),
		}
		// In dev mode, delay enrichment by 1s to simulate the real-world
		// async flow where alerts arrive first and OCM data follows.
		// Without this, fixture data enriches instantly and the SRE
		// never sees the service name change or the flash notification.
		if devMode {
			for _, cmd := range clusterCmds {
				delayedCmd := cmd
				cmds = append(cmds, tea.Tick(time.Second, func(time.Time) tea.Msg {
					return delayedCmd()
				}))
			}
		} else {
			cmds = append(cmds, clusterCmds...)
		}
	}
	return cmds
}

func getClusterInfo(client ocm.OCMClient, clusterID string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("ocm.GetCluster", "cluster_id", clusterID)
		info, err := client.GetCluster(clusterID)
		return clusterInfoMsg{clusterID: clusterID, info: info, err: err}
	}
}

func getOCMServiceLogs(client ocm.OCMClient, clusterID, externalID string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("ocm.GetServiceLogs", "cluster_id", clusterID)
		logs, err := client.GetServiceLogs(clusterID, externalID)
		log.Debug("ocm.GetServiceLogs", "cluster_id", clusterID, "count", len(logs))
		return ocmServiceLogsMsg{clusterID: clusterID, logs: logs, err: err}
	}
}

func getClusterReports(client ocm.OCMClient, clusterID string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("ocm.GetClusterReports", "cluster_id", clusterID)
		reports, err := client.GetClusterReports(clusterID)
		log.Debug("ocm.GetClusterReports", "cluster_id", clusterID, "count", len(reports))
		return clusterReportsMsg{clusterID: clusterID, reports: reports, err: err}
	}
}

func getLimitedSupportHistory(client ocm.OCMClient, clusterID string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("ocm.GetLimitedSupportHistory", "cluster_id", clusterID)
		reasons, err := client.GetLimitedSupportHistory(clusterID)
		log.Debug("ocm.GetLimitedSupportHistory", "cluster_id", clusterID, "count", len(reasons))
		return limitedSupportMsg{clusterID: clusterID, reasons: reasons, err: err}
	}
}
