package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/launcher"
)

const (
	claudeTimeout = 60 * time.Second

	claudeSystemPrompt = "You are in read-only investigation mode for SRE PagerDuty incident triage. " +
		"Suggest commands for the user to run if changes are needed. Do not modify cluster state. " +
		"All cluster commands must be read-only (oc get, oc describe, NOT oc delete/patch). " +
		"If a fix requires changes, OUTPUT the commands for the SRE to review and run manually."
)

// claudePromptMsg is sent when the user submits a prompt from the input field
type claudePromptMsg struct {
	prompt string
}

// claudeResponseMsg is the response from a Claude CLI query
type claudeResponseMsg struct {
	response string
	err      error
}

// buildClaudeEnvVars constructs environment variable KEY=VALUE pairs for
// passing PagerDuty incident context to the Claude CLI process.
func buildClaudeEnvVars(incident *pagerduty.Incident, alerts []pagerduty.IncidentAlert) []string {
	var envVars []string

	if incident != nil {
		envVars = append(envVars,
			fmt.Sprintf("PAGERDUTY_INCIDENT_ID=%s", incident.ID),
			fmt.Sprintf("PAGERDUTY_INCIDENT_TITLE=%s", sanitizeEnvValue(incident.Title)),
			fmt.Sprintf("PAGERDUTY_INCIDENT_URL=%s", incident.HTMLURL),
			fmt.Sprintf("PAGERDUTY_INCIDENT_SERVICE=%s", sanitizeEnvValue(incident.Service.Summary)),
			fmt.Sprintf("PAGERDUTY_INCIDENT_URGENCY=%s", incident.Urgency),
			fmt.Sprintf("PAGERDUTY_INCIDENT_STATUS=%s", incident.Status),
		)
	}

	// Extract cluster ID from first alert that has one
	for _, alert := range alerts {
		cluster := getDetailFieldFromAlert("cluster_id", alert)
		if cluster != "" {
			envVars = append(envVars, fmt.Sprintf("PAGERDUTY_CLUSTER_ID=%s", cluster))
			break
		}
	}

	// Collect alert names
	var alertNames []string
	for _, alert := range alerts {
		name := getDetailFieldFromAlert("alert_name", alert)
		if name != "" {
			alertNames = append(alertNames, sanitizeEnvValue(name))
		}
	}
	if len(alertNames) > 0 {
		envVars = append(envVars, fmt.Sprintf("PAGERDUTY_ALERT_NAMES=%s", strings.Join(alertNames, ",")))
	}

	return envVars
}

// claudeQuery dispatches a prompt to the Claude CLI and returns the response.
// The prompt is piped via stdin (no -p flag) for safety.
func claudeQuery(prompt string, incident *pagerduty.Incident, alerts []pagerduty.IncidentAlert) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), claudeTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "claude", "--print")
		cmd.Stdin = strings.NewReader(claudeSystemPrompt + "\n\n" + prompt)

		// Set environment variables with PagerDuty context
		cmd.Env = append(os.Environ(), buildClaudeEnvVars(incident, alerts)...)

		log.Debug("tui.claudeQuery()", "prompt", prompt)

		output, err := cmd.Output()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return claudeResponseMsg{
					response: "",
					err:      fmt.Errorf("query timed out after %s", claudeTimeout),
				}
			}
			return claudeResponseMsg{
				response: "",
				err:      fmt.Errorf("claude error: %w", err),
			}
		}

		response := strings.TrimSpace(string(output))
		return claudeResponseMsg{
			response: response,
			err:      nil,
		}
	}
}

// handleClaudePrompt processes a claudePromptMsg, checking for Claude availability
// and dispatching the query command. The hasClaudeCode parameter allows injecting
// a mock for testing.
func (m model) handleClaudePrompt(msg claudePromptMsg, hasClaudeCode func() bool) (tea.Model, tea.Cmd) {
	if !hasClaudeCode() {
		m.setStatus("Claude Code not installed - install from https://claude.ai/download")
		return m, nil
	}

	m.setStatus(fmt.Sprintf("querying Claude: %s", truncatePrompt(msg.prompt, 40)))
	m.claudeQuerying = true
	m.apiInProgress = true

	return m, tea.Batch(
		m.spinner.Tick,
		claudeQuery(msg.prompt, m.selectedIncident, m.selectedIncidentAlerts),
	)
}

// handleClaudeResponse processes a claudeResponseMsg, rendering the response
// in the incident viewer or displaying an error/empty status.
func (m model) handleClaudeResponse(msg claudeResponseMsg) (tea.Model, tea.Cmd) {
	m.claudeQuerying = false
	m.apiInProgress = false

	if msg.err != nil {
		m.setStatus(fmt.Sprintf("Claude query failed: %s", msg.err))
		return m, nil
	}

	if msg.response == "" {
		m.setStatus("no response from Claude")
		return m, nil
	}

	// Format the response with a Claude header
	content := fmt.Sprintf("# Claude Response\n\n%s", msg.response)

	// Render as markdown if renderer is available
	if m.markdownRenderer != nil {
		rendered, err := m.markdownRenderer.Render(content)
		if err == nil {
			content = rendered
		}
	}

	m.incidentViewer.SetContent(content)
	m.incidentViewer.GotoTop()
	m.viewingIncident = true
	m.setStatus("Claude response received")

	return m, nil
}

// truncatePrompt shortens a prompt string for display in the status bar
func truncatePrompt(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// defaultHasClaudeCode wraps launcher.HasClaudeCode for production use
func defaultHasClaudeCode() bool {
	return launcher.HasClaudeCode()
}
