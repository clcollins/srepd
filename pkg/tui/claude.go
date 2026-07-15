package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/clcollins/srepd/pkg/ai"
)

// CommandExecutor abstracts command execution so tests can inject a mock
// instead of spawning real subprocesses.
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args []string, stdin io.Reader, env []string) (stdout []byte, stderr string, err error)
}

// execCommandExecutor is the default implementation that uses os/exec.
type execCommandExecutor struct{}

func (e *execCommandExecutor) Execute(ctx context.Context, name string, args []string, stdin io.Reader, env []string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Env = env

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	output, err := cmd.Output()
	return output, strings.TrimSpace(stderrBuf.String()), err
}

const (
	claudeTimeout = 60 * time.Second
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
// The prompt is piped via stdin for safety. The executor parameter abstracts
// command execution for testability.
func agentQuery(executor CommandExecutor, agentCLICommand string, systemPrompt string, prompt string, incidentContext string, incident *pagerduty.Incident, alerts []pagerduty.IncidentAlert) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), claudeTimeout)
		defer cancel()

		args := strings.Fields(agentCLICommand)
		if len(args) == 0 {
			return claudeResponseMsg{err: fmt.Errorf("agent_cli_command is empty")}
		}

		fullPrompt := systemPrompt + "\n\n" + prompt
		if incidentContext != "" {
			fullPrompt += "\n\nContext:\n" + incidentContext
		}

		env := append(os.Environ(), buildClaudeEnvVars(incident, alerts)...)

		log.Debug("tui.agentQuery()", "command", agentCLICommand, "prompt", prompt)

		output, stderrStr, err := executor.Execute(ctx, args[0], args[1:], strings.NewReader(fullPrompt), env)
		if err != nil {
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

	// Guard against a whitespace-only command: strings.Fields returns an empty
	// slice, so indexing [0] would panic (mirrors agentQuery's len==0 check).
	fields := strings.Fields(agentCmd)
	if len(fields) == 0 {
		return m, m.flashNotification("agent_cli_command is empty")
	}
	binary := fields[0]
	if _, err := lookPath(binary); err != nil {
		return m, m.flashNotification(fmt.Sprintf("agent CLI %q not found on PATH", binary))
	}

	incidentID := ""
	if m.selectedIncident != nil {
		incidentID = m.selectedIncident.ID
	}
	log.Info("agent query initiated", "command", agentCmd, "incident_id", incidentID, "prompt", truncatePrompt(msg.prompt, 80))
	m.setStatus(fmt.Sprintf("querying agent: %s", truncatePrompt(msg.prompt, 40)))
	m.claudeQuerying = true
	m.apiInProgress = true
	m.watcherQueryStart = time.Now()
	m.watcherQueryTimeout = claudeTimeout

	if !m.watcherExpanded {
		m.watcherExpanded = true
		m.recomputeLayout()
	}

	incidentContext := buildWatcherContext(&m)

	if m.streamResponses && isClaudeCLI(agentCmd) {
		m.agentStreamPartial = ""
		return m, tea.Batch(
			m.spinner.Tick,
			streamAgentCmd(agentCmd, m.agentSystemPrompt, msg.prompt, incidentContext, m.selectedIncident, m.selectedIncidentAlerts, claudeTimeout),
		)
	}

	return m, tea.Batch(
		m.spinner.Tick,
		agentQuery(m.cmdExecutor, agentCmd, m.agentSystemPrompt, msg.prompt, incidentContext, m.selectedIncident, m.selectedIncidentAlerts),
	)
}

// handleClaudeResponse processes a claudeResponseMsg, rendering the response
// in the incident viewer or displaying an error/empty status.
func (m model) handleClaudeResponse(msg claudeResponseMsg) (tea.Model, tea.Cmd) {
	m.claudeQuerying = false
	m.apiInProgress = false

	if msg.err != nil {
		classified := ai.ClassifyProviderError(msg.err)
		return m, func() tea.Msg {
			return errMsg{fmt.Errorf("agent query failed: %s", classified)}
		}
	}

	if msg.response == "" {
		return m, m.flashNotification("agent returned empty response")
	}

	if !m.watcherExpanded {
		m.watcherExpanded = true
		m.recomputeLayout()
	}

	log.Info("claude response received")
	m.setStatus("Claude response received")

	m.watcherBuffer.Append("")
	return m, m.startTypewriter(m.agentMarker, msg.response)
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
	return strings.HasPrefix(trimmed, ":agent ") || trimmed == ":agent"
}

func parseAgentQuery(input string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), ":agent"))
}

// isClaudeCLI reports whether the agent CLI command invokes the Claude Code CLI.
// It checks if any whitespace-delimited token in the command has a basename of "claude".
func isClaudeCLI(cmd string) bool {
	for _, field := range strings.Fields(cmd) {
		if filepath.Base(field) == "claude" {
			return true
		}
	}
	return false
}

// cliStreamLine is the top-level JSON object in Claude CLI stream-json output.
type cliStreamLine struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Event   *cliStreamEvent `json:"event,omitempty"`
}

type cliStreamEvent struct {
	Type  string         `json:"type"`
	Delta *cliEventDelta `json:"delta,omitempty"`
}

type cliEventDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func parseCLIStreamLine(data []byte) (cliStreamLine, error) {
	var line cliStreamLine
	err := json.Unmarshal(data, &line)
	return line, err
}

// buildStreamingArgs appends --output-format stream-json, --verbose, and
// --include-partial-messages to the argument list if not already present.
// Claude Code requires --verbose when using --output-format=stream-json
// with --print.
func buildStreamingArgs(args []string) []string {
	hasOutputFormat := false
	hasVerbose := false
	hasPartial := false
	for _, a := range args {
		if a == "--output-format" {
			hasOutputFormat = true
		}
		if a == "--verbose" {
			hasVerbose = true
		}
		if a == "--include-partial-messages" {
			hasPartial = true
		}
	}
	if !hasOutputFormat {
		args = append(args, "--output-format", "stream-json")
	}
	if !hasVerbose {
		args = append(args, "--verbose")
	}
	if !hasPartial {
		args = append(args, "--include-partial-messages")
	}
	return args
}

type agentStreamStartedMsg struct {
	ch     <-chan streamEvent
	cancel context.CancelFunc
}

type agentStreamChunkMsg struct {
	text string
	ch   <-chan streamEvent
}

type agentStreamDoneMsg struct {
	err error
}

// readAgentStreamCmd drains one event from the stream channel and returns it
// as an agent-specific message (mirrors readStreamCmd for the watcher).
func readAgentStreamCmd(ch <-chan streamEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return agentStreamDoneMsg{}
		}
		if ev.done {
			return agentStreamDoneMsg{err: ev.err}
		}
		return agentStreamChunkMsg{text: ev.text, ch: ch}
	}
}

// streamAgentCmd spawns the Claude CLI with streaming flags and feeds text
// deltas into a streamEvent channel for token-by-token rendering.
//
// startupTimeout guards against the CLI hanging before producing any output.
// Once the first text delta arrives, the watchdog is disarmed and the stream
// runs to completion no matter how long it takes.
func streamAgentCmd(agentCLICommand string, systemPrompt string, prompt string, incidentContext string, incident *pagerduty.Incident, alerts []pagerduty.IncidentAlert, startupTimeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		args := strings.Fields(agentCLICommand)
		if len(args) == 0 {
			return agentStreamDoneMsg{err: fmt.Errorf("agent_cli_command is empty")}
		}

		args = buildStreamingArgs(args)

		fullPrompt := systemPrompt + "\n\n" + prompt
		if incidentContext != "" {
			fullPrompt += "\n\nContext:\n" + incidentContext
		}

		// Cancelable only — the startup watchdog below kills a CLI that never
		// responds; after the first token only the consumer can cancel.
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		// Grandchildren can inherit the stdout pipe and keep the scanner
		// blocked after the direct child is killed; force-close the pipes
		// shortly after cancellation so the stream terminates promptly.
		cmd.WaitDelay = 2 * time.Second
		cmd.Stdin = strings.NewReader(fullPrompt)
		cmd.Env = append(os.Environ(), buildClaudeEnvVars(incident, alerts)...)

		var stderrBuf strings.Builder
		cmd.Stderr = &stderrBuf

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			cancel()
			return agentStreamDoneMsg{err: fmt.Errorf("agent stream pipe: %w", err)}
		}

		if err := cmd.Start(); err != nil {
			cancel()
			return agentStreamDoneMsg{err: fmt.Errorf("agent stream start: %w", err)}
		}

		ch := make(chan streamEvent, 64)

		go func() {
			// Deferred order (LIFO): close the channel for the consumer, kill
			// the process via cancel, then Wait to reap it — early returns in
			// the scan loop would otherwise leak a zombie. The extra Wait after
			// the explicit one below returns an error that is safely ignored.
			defer func() { _ = cmd.Wait() }()
			defer cancel()
			defer close(ch)

			// Startup watchdog: kill the CLI if it produces no text before the
			// deadline — a silent process blocks the scanner forever, so a
			// between-lines context check can never fire. Disarmed by the
			// first token.
			var timedOut atomic.Bool
			watchdog := time.AfterFunc(startupTimeout, func() {
				timedOut.Store(true)
				cancel()
			})
			defer watchdog.Stop()

			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 256*1024), 256*1024)

			receivedFirstToken := false

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
				}

				line, parseErr := parseCLIStreamLine(scanner.Bytes())
				if parseErr != nil {
					continue
				}

				if line.Type == "stream_event" && line.Event != nil &&
					line.Event.Type == "content_block_delta" && line.Event.Delta != nil &&
					line.Event.Delta.Type == "text_delta" && line.Event.Delta.Text != "" {
					if !receivedFirstToken {
						receivedFirstToken = true
						watchdog.Stop()
						log.Debug("agent.stream", "msg", "first token received, startup watchdog disarmed")
					}
					select {
					case ch <- streamEvent{text: line.Event.Delta.Text}:
					case <-ctx.Done():
						return
					}
				}

				if line.Type == "result" {
					if line.Subtype != "success" {
						ch <- streamEvent{done: true, err: fmt.Errorf("agent CLI result: %s", line.Subtype)}
					} else {
						ch <- streamEvent{done: true}
					}
					return
				}
			}

			if err := cmd.Wait(); err != nil {
				if timedOut.Load() {
					ch <- streamEvent{done: true, err: fmt.Errorf("no response from agent CLI after %s", startupTimeout)}
					return
				}
				stderrStr := strings.TrimSpace(stderrBuf.String())
				if stderrStr != "" {
					log.Warn("agent.stream", "stderr", stderrStr)
				}
				ch <- streamEvent{done: true, err: fmt.Errorf("agent stream: %w", err)}
				return
			}

			ch <- streamEvent{done: true}
		}()

		return agentStreamStartedMsg{ch: ch, cancel: cancel}
	}
}
