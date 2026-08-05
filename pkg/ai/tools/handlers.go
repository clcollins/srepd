package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ai/policy"
	"github.com/clcollins/srepd/pkg/ocm"
	"github.com/clcollins/srepd/pkg/pd"
)

const maxServiceLogs = 5

// RegisterPDTools registers all PagerDuty read-only tools.
func RegisterPDTools(reg *Registry, client pd.PagerDutyClientInterface, pdConfig *pd.Config) error {
	tools := []Tool{
		newGetIncidentTool(client),
		newGetAlertsTool(client),
		newGetNotesTool(client),
		newListQueueTool(pdConfig),
	}
	for _, t := range tools {
		if err := reg.Register(t); err != nil {
			return err
		}
	}
	return nil
}

// RegisterOCMTools registers all OCM read-only tools.
func RegisterOCMTools(reg *Registry, client ocm.OCMClient) error {
	tools := []Tool{
		newGetClusterInfoTool(client),
		newGetServiceLogsTool(client),
		newGetLimitedSupportTool(client),
	}
	for _, t := range tools {
		if err := reg.Register(t); err != nil {
			return err
		}
	}
	return nil
}

type idInput struct {
	ID string `json:"id"`
}

type incidentIDInput struct {
	IncidentID string `json:"incident_id"`
}

type clusterIDInput struct {
	ClusterID string `json:"cluster_id"`
}

func newGetIncidentTool(client pd.PagerDutyClientInterface) Tool {
	return Tool{
		Name:        "get_incident",
		Description: "Get details of a PagerDuty incident by ID",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{"id":{"type":"string","description":"The PagerDuty incident ID"}},"required":["id"]}`),
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params idInput
			if err := json.Unmarshal(input, &params); err != nil {
				return formatError("invalid input", err), nil
			}
			incident, err := client.GetIncidentWithContext(ctx, params.ID)
			if err != nil {
				return formatError("incident not found", err), nil
			}
			return marshalResult(incident)
		},
	}
}

func newGetAlertsTool(client pd.PagerDutyClientInterface) Tool {
	return Tool{
		Name:        "get_alerts",
		Description: "Get alerts for a PagerDuty incident",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{"incident_id":{"type":"string","description":"The PagerDuty incident ID"}},"required":["incident_id"]}`),
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params incidentIDInput
			if err := json.Unmarshal(input, &params); err != nil {
				return formatError("invalid input", err), nil
			}
			resp, err := client.ListIncidentAlertsWithContext(ctx, params.IncidentID, pagerduty.ListIncidentAlertsOptions{})
			if err != nil {
				return formatError("alerts not found", err), nil
			}
			return marshalResult(resp.Alerts)
		},
	}
}

func newGetNotesTool(client pd.PagerDutyClientInterface) Tool {
	return Tool{
		Name:        "get_notes",
		Description: "Get notes for a PagerDuty incident",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{"incident_id":{"type":"string","description":"The PagerDuty incident ID"}},"required":["incident_id"]}`),
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params incidentIDInput
			if err := json.Unmarshal(input, &params); err != nil {
				return formatError("invalid input", err), nil
			}
			notes, err := client.ListIncidentNotesWithContext(ctx, params.IncidentID)
			if err != nil {
				return formatError("notes not found", err), nil
			}
			return marshalResult(notes)
		},
	}
}

func newListQueueTool(pdConfig *pd.Config) Tool {
	return Tool{
		Name:        "list_queue",
		Description: "List all incidents in the current PagerDuty queue",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{}}`),
		Handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			if pdConfig == nil || pdConfig.Client == nil {
				return formatError("PagerDuty not configured", nil), nil
			}
			opts := pagerduty.ListIncidentsOptions{
				Statuses: []string{"triggered", "acknowledged"},
			}
			if len(pdConfig.TeamsMemberIDs) > 0 {
				opts.UserIDs = pdConfig.TeamsMemberIDs
			}
			resp, err := pdConfig.Client.ListIncidentsWithContext(ctx, opts)
			if err != nil {
				return formatError("failed to list incidents", err), nil
			}
			incidents := resp.Incidents
			const maxIncidents = 25
			if len(incidents) > maxIncidents {
				incidents = incidents[:maxIncidents]
			}
			return marshalResult(incidents)
		},
	}
}

func newGetClusterInfoTool(client ocm.OCMClient) Tool {
	return Tool{
		Name:        "get_cluster_info",
		Description: "Get OpenShift cluster information by cluster ID",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{"cluster_id":{"type":"string","description":"The OpenShift cluster ID"}},"required":["cluster_id"]}`),
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params clusterIDInput
			if err := json.Unmarshal(input, &params); err != nil {
				return formatError("invalid input", err), nil
			}
			info, err := client.GetCluster(ctx, params.ClusterID)
			if err != nil {
				return formatError("cluster not found", err), nil
			}
			return marshalResult(info)
		},
	}
}

func newGetServiceLogsTool(client ocm.OCMClient) Tool {
	return Tool{
		Name:        "get_service_logs",
		Description: "Get recent service logs for a cluster (up to 5)",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{"cluster_id":{"type":"string","description":"The OpenShift cluster ID"}},"required":["cluster_id"]}`),
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params clusterIDInput
			if err := json.Unmarshal(input, &params); err != nil {
				return formatError("invalid input", err), nil
			}
			logs, err := client.GetServiceLogs(ctx, params.ClusterID, "")
			if err != nil {
				return formatError("service logs not found", err), nil
			}
			if len(logs) > maxServiceLogs {
				logs = logs[:maxServiceLogs]
			}
			return marshalResult(logs)
		},
	}
}

func newGetLimitedSupportTool(client ocm.OCMClient) Tool {
	return Tool{
		Name:        "get_limited_support",
		Description: "Get limited support reasons for a cluster",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{"cluster_id":{"type":"string","description":"The OpenShift cluster ID"}},"required":["cluster_id"]}`),
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params clusterIDInput
			if err := json.Unmarshal(input, &params); err != nil {
				return formatError("invalid input", err), nil
			}
			reasons, err := client.GetLimitedSupportHistory(ctx, params.ClusterID)
			if err != nil {
				return formatError("limited support info not found", err), nil
			}
			return marshalResult(reasons)
		},
	}
}

func marshalResult(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(data), nil
}

func formatError(msg string, err error) string {
	if err != nil {
		log.Debug("tools.handler", "msg", msg, "error", err)
		return fmt.Sprintf("%s (%s)", msg, classifyToolError(err))
	}
	return msg
}

func classifyToolError(err error) string {
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline"):
		return "timeout"
	case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401"):
		return "auth error"
	case strings.Contains(errStr, "not found") || strings.Contains(errStr, "404"):
		return "not found"
	case strings.Contains(errStr, "connection") || strings.Contains(errStr, "network"):
		return "network error"
	default:
		return "request failed"
	}
}
