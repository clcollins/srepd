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

// agentQuery dispatches a prompt to the configured CLI agent and returns the
// response. The command is parsed from the agentCLICommand config string.
// The prompt is piped via stdin for safety.
func agentQuery(agentCLICommand string, prompt string, incident *pagerduty.Incident, alerts []pagerduty.IncidentAlert) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), claudeTimeout)
		defer cancel()

		args := strings.Fields(agentCLICommand)
		if len(args) == 0 {
			return claudeResponseMsg{err: fmt.Errorf("agent_cli_command is empty")}
		}

		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(claudeSystemPrompt + "\n\n" + prompt)
		cmd.Env = append(os.Environ(), buildClaudeEnvVars(incident, alerts)...)

		var stderr strings.Builder
		cmd.Stderr = &stderr

		log.Debug("tui.agentQuery()", "command", agentCLICommand, "prompt", prompt)

		output, err := cmd.Output()
		if err != nil {
			stderrStr := strings.TrimSpace(stderr.String())
			if stderrStr != "" {
				log.Warn("tui.agentQuery()", "stderr", stderrStr)
			}
			if ctx.Err() == context.DeadlineExceeded {
				return claudeResponseMsg{
					response: "",
					err:      fmt.Errorf("query timed out after %s", claudeTimeout),
				}
			}
			if stderrStr != "" {
				return claudeResponseMsg{
					response: "",
					err:      fmt.Errorf("agent error: %w: %s", err, stderrStr),
				}
			}
			return claudeResponseMsg{
				response: "",
				err:      fmt.Errorf("agent error: %w", err),
			}
		}

		response := strings.TrimSpace(string(output))
		return claudeResponseMsg{
			response: response,
			err:      nil,
		}
	}
}

// handleClaudePrompt processes a claudePromptMsg, checking for agent CLI
// availability and dispatching the query command. The lookPath parameter
// allows injecting a mock for testing.
func (m model) handleClaudePrompt(msg claudePromptMsg, lookPath func(string) (string, error)) (tea.Model, tea.Cmd) {
	agentCmd := m.agentCLICommand
	if agentCmd == "" {
		agentCmd = "claude --print"
	}

	binary := strings.Fields(agentCmd)[0]
	if _, err := lookPath(binary); err != nil {
		m.setStatus(fmt.Sprintf("agent CLI %q not found on PATH", binary))
		return m, nil
	}

	incidentID := ""
	if m.selectedIncident != nil {
		incidentID = m.selectedIncident.ID
	}
	log.Info("agent query initiated", "command", agentCmd, "incident_id", incidentID, "prompt", truncatePrompt(msg.prompt, 80))
	m.setStatus(fmt.Sprintf("querying agent: %s", truncatePrompt(msg.prompt, 40)))
	m.claudeQuerying = true
	m.apiInProgress = true

	if !m.watcherExpanded {
		m.watcherExpanded = true
		m.recomputeLayout()
	}

	return m, tea.Batch(
		m.spinner.Tick,
		agentQuery(agentCmd, msg.prompt, m.selectedIncident, m.selectedIncidentAlerts),
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

	m.watcherBuffer.Append(prefixLines(m.agentMarker, msg.response))
	m.updateWatcherViewport()

	if !m.watcherExpanded {
		m.watcherExpanded = true
		m.recomputeLayout()
	}

	log.Info("claude response received")
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

// defaultLookPath wraps exec.LookPath for production use
func defaultLookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func isAgentCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "/agent ") || trimmed == "/agent"
}

func parseAgentQuery(input string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "/agent"))
}
