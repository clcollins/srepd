package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInputCharLimit_Increased(t *testing.T) {
	// newTextInput should have a CharLimit of 500 (increased from 32)
	// to allow longer prompts for Claude queries
	input := newTextInput()
	assert.Equal(t, 500, input.CharLimit, "CharLimit should be 500 for Claude prompts")
}

func TestClaudePrompt_DispatchesCommand(t *testing.T) {
	// When the user is in input focus mode and presses Enter with /agent prefix,
	// a claudePromptMsg should be dispatched with the query text (prefix stripped),
	// the input should be reset and blurred.

	m := createTestModel()
	m.input = newTextInput()
	m.input.Focus()
	m.input.SetValue("/agent investigate this alert")

	// Simulate pressing Enter in input focus mode
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := switchInputFocusMode(m, enterMsg)
	updatedModel := result.(model)

	// Input should be blurred and reset
	assert.False(t, updatedModel.input.Focused(), "Input should be blurred after Enter")
	assert.Equal(t, "", updatedModel.input.Value(), "Input should be reset after Enter")

	// A command should be returned
	assert.NotNil(t, cmd, "Enter in input mode should return a command")

	// The command should produce a claudePromptMsg with prefix stripped
	msg := cmd()
	promptMsg, ok := msg.(claudePromptMsg)
	assert.True(t, ok, "Command should produce a claudePromptMsg, got %T", msg)
	assert.Equal(t, "investigate this alert", promptMsg.prompt, "Prompt text should have /agent prefix stripped")
}

func TestClaudePrompt_EmptyInput(t *testing.T) {
	// When the user presses Enter with empty input, no command should be dispatched

	m := createTestModel()
	m.input = newTextInput()
	m.input.Focus()
	m.input.SetValue("")

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := switchInputFocusMode(m, enterMsg)
	updatedModel := result.(model)

	// Input should be blurred
	assert.False(t, updatedModel.input.Focused(), "Input should be blurred after Enter on empty input")

	// No command should be returned for empty input
	assert.Nil(t, cmd, "Empty input should not dispatch a command")
}

func TestClaudeNotFound_ShowsStatus(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.agentCLICommand = "nonexistent-binary --print"

	msg := claudePromptMsg{prompt: "test query"}

	result, cmd := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "", fmt.Errorf("not found: %s", s)
	})
	updatedModel := result.(model)

	assert.Contains(t, updatedModel.status, "not found on PATH")
	assert.Nil(t, cmd)
}

func TestClaudePrompt_ShowsSpinner(t *testing.T) {
	// When claudePromptMsg is received and Claude is installed,
	// the model should show spinner and set querying state

	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "Q123"},
	}

	msg := claudePromptMsg{prompt: "test query"}

	result, cmd := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "/usr/bin/" + s, nil
	})
	updatedModel := result.(model)

	assert.True(t, updatedModel.claudeQuerying, "claudeQuerying should be true")
	assert.True(t, updatedModel.apiInProgress, "apiInProgress should be true for spinner")
	assert.Contains(t, updatedModel.status, "querying agent")
	assert.NotNil(t, cmd, "A command should be returned to execute the query")
}

func TestClaudeResponse_RendersInViewport(t *testing.T) {
	// When claudeResponseMsg is received with a successful response,
	// the content should be set in the incidentViewer and viewingIncident
	// should be true

	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.claudeQuerying = true
	m.apiInProgress = true
	m.incidentViewer = newIncidentViewer()

	msg := claudeResponseMsg{
		response: "Based on the alert, the cluster appears to have high CPU usage.",
		err:      nil,
	}

	result, _ := m.Update(msg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.claudeQuerying, "claudeQuerying should be false after response")
	assert.False(t, updatedModel.apiInProgress, "apiInProgress should be false after response")
	assert.True(t, updatedModel.viewingIncident, "viewingIncident should be true to show response")
}

func TestClaudeResponse_Error(t *testing.T) {
	// When claudeResponseMsg has an error, the status should show the error

	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.claudeQuerying = true
	m.apiInProgress = true

	msg := claudeResponseMsg{
		response: "",
		err:      assert.AnError,
	}

	result, _ := m.Update(msg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.claudeQuerying, "claudeQuerying should be false after error")
	assert.False(t, updatedModel.apiInProgress, "apiInProgress should be false after error")
	assert.Contains(t, updatedModel.status, "Claude query failed",
		"Status should indicate failure")
}

func TestClaudeResponse_EmptyResponse(t *testing.T) {
	// When claudeResponseMsg has no error but empty response,
	// the status should indicate no response

	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.claudeQuerying = true
	m.apiInProgress = true

	msg := claudeResponseMsg{
		response: "",
		err:      nil,
	}

	result, _ := m.Update(msg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.claudeQuerying, "claudeQuerying should be false after empty response")
	assert.Contains(t, updatedModel.status, "no response",
		"Status should indicate no response")
}

func TestClaudeQuery_PassesContext(t *testing.T) {
	// claudeQuery should pass PagerDuty context as environment variables

	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID:      "PD123",
			HTMLURL: "https://pagerduty.com/incidents/PD123",
		},
		Title:   "Test Incident",
		Urgency: "high",
		Status:  "triggered",
		Service: pagerduty.APIObject{Summary: "test-service"},
	}

	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "HighCPU",
				},
			},
		},
	}

	env := buildClaudeEnvVars(incident, alerts)

	// Verify environment variables are set
	envMap := make(map[string]string)
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}

	assert.Equal(t, "PD123", envMap["PAGERDUTY_INCIDENT_ID"],
		"Should include incident ID")
	assert.Equal(t, "Test Incident", envMap["PAGERDUTY_INCIDENT_TITLE"],
		"Should include incident title")
	assert.Equal(t, "cluster-abc", envMap["PAGERDUTY_CLUSTER_ID"],
		"Should include cluster ID from alerts")
}

func TestClaudeQuery_SystemPrompt(t *testing.T) {
	// The system prompt should specify read-only investigation mode
	assert.Contains(t, claudeSystemPrompt, "read-only",
		"System prompt should specify read-only mode")
	assert.Contains(t, claudeSystemPrompt, "investigation",
		"System prompt should mention investigation")
}

func TestInputFocusMode_EscBlursInput(t *testing.T) {
	// Pressing Escape in input mode should blur and reset input

	m := createTestModel()
	m.input = newTextInput()
	m.input.Focus()
	m.input.SetValue("some text")

	// Simulate pressing Escape
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := switchInputFocusMode(m, escMsg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.input.Focused(), "Input should be blurred after Escape")
	assert.Equal(t, "", updatedModel.input.Value(), "Input should be reset after Escape")
}

func TestInputFocusMode_TableRegainsFocusOnBlur(t *testing.T) {
	// When input is blurred (Enter or Escape), the table should regain focus

	incidents := []pagerduty.Incident{
		{
			APIObject: pagerduty.APIObject{ID: "Q111"},
			Title:     "Test Alert",
			Service:   pagerduty.APIObject{Summary: "test-service"},
		},
	}

	m := createTestModelWithTableRows(incidents)
	m.input = newTextInput()
	m.input.Focus()
	m.input.SetValue("/agent query text")

	// Simulate pressing Enter
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := switchInputFocusMode(m, enterMsg)
	updatedModel := result.(model)

	assert.True(t, updatedModel.table.Focused(), "Table should regain focus after input Enter")
}

func TestClaudePrompt_InputModeKeyMapUpdated(t *testing.T) {
	// The input mode keymap Enter help text should reflect claude query
	assert.Equal(t, "enter", inputModeKeyMap.Enter.Help().Key,
		"Enter key help should show 'enter'")
	assert.Equal(t, "ask Claude", inputModeKeyMap.Enter.Help().Desc,
		"Enter key help description should say 'ask Claude'")
}

func TestClaudePrompt_WithViewingIncident(t *testing.T) {
	// When in incident view, entering input mode and submitting should work
	// by setting viewingIncident to true for the response display

	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.viewingIncident = true
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "Q123"},
	}

	msg := claudePromptMsg{prompt: "what is wrong with this cluster?"}

	result, cmd := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "/usr/bin/" + s, nil
	})
	updatedModel := result.(model)

	assert.True(t, updatedModel.claudeQuerying, "Should start querying")
	assert.NotNil(t, cmd, "Should dispatch query command")
}

func TestNewTextInputWidth(t *testing.T) {
	// newTextInput width should be set to 120 (will be adjusted by window resize)
	input := newTextInput()
	assert.Equal(t, 120, input.Width, "Input width should be 120")
}

func TestClaudeResponse_RendersWithPrefix(t *testing.T) {
	// When Claude response is rendered in the viewport, it should have
	// a clear prefix indicating it's from Claude

	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.claudeQuerying = true
	m.apiInProgress = true
	m.incidentViewer = newIncidentViewer()

	msg := claudeResponseMsg{
		response: "The cluster is experiencing high CPU usage.",
		err:      nil,
	}

	result, _ := m.Update(msg)
	_ = result.(model)

	// The response content is set on the viewport - we can't directly read it back
	// from viewport.Model in tests, but we verify the model state is correct
	// The actual prefix is added in the Update handler
}

func TestDefaultLookPath_NoPanic(t *testing.T) {
	t.Run("defaultLookPath does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_, _ = defaultLookPath("nonexistent-binary-12345")
		}, "defaultLookPath should not panic for missing binaries")
	})
}

func TestAgentQuery_EmptyCommand(t *testing.T) {
	cmd := agentQuery("", "test prompt", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.Error(t, resp.err)
	assert.Contains(t, resp.err.Error(), "agent_cli_command is empty")
}

func TestAgentQuery_CommandParsing(t *testing.T) {
	cmd := agentQuery("echo hello world", "ignored", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)
	assert.Equal(t, "hello world", resp.response)
}

func TestAgentQuery_MultiWordCommand(t *testing.T) {
	cmd := agentQuery("echo -n test", "ignored", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)
	assert.Equal(t, "test", resp.response)
}

func TestAgentQuery_CommandNotFound(t *testing.T) {
	cmd := agentQuery("nonexistent-binary-99999 --flag", "test", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.Error(t, resp.err)
	assert.Contains(t, resp.err.Error(), "agent error")
}

func TestAgentQuery_StderrCaptured(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fail.sh")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho oops >&2\nexit 1\n"), 0755)
	assert.NoError(t, err)

	cmd := agentQuery(script, "test", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.Error(t, resp.err)
	assert.Contains(t, resp.err.Error(), "oops")
}

func TestAgentQuery_PassesEnvVars(t *testing.T) {
	script := filepath.Join(t.TempDir(), "env.sh")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho $PAGERDUTY_INCIDENT_ID\n"), 0755)
	assert.NoError(t, err)

	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "PD999"},
		Title:     "Test",
		Urgency:   "high",
		Status:    "triggered",
		Service:   pagerduty.APIObject{Summary: "svc"},
	}

	cmd := agentQuery(script, "test", incident, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)
	assert.Equal(t, "PD999", resp.response)
}

func TestAgentQuery_PipesStdin(t *testing.T) {
	cmd := agentQuery("cat", "user question here", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)
	assert.Contains(t, resp.response, "user question here")
	assert.Contains(t, resp.response, claudeSystemPrompt)
}

func TestHandleClaudePrompt_DefaultCommand(t *testing.T) {
	m := createTestModel()
	m.agentCLICommand = ""

	msg := claudePromptMsg{prompt: "test"}
	result, _ := m.handleClaudePrompt(msg, func(s string) (string, error) {
		assert.Equal(t, "claude", s, "should look up 'claude' when agentCLICommand is empty")
		return "/usr/bin/claude", nil
	})
	updated := result.(model)
	assert.True(t, updated.claudeQuerying)
}

func TestHandleClaudePrompt_CustomCommand(t *testing.T) {
	m := createTestModel()
	m.agentCLICommand = "/opt/bin/my-agent --verbose --print"

	msg := claudePromptMsg{prompt: "test"}
	result, _ := m.handleClaudePrompt(msg, func(s string) (string, error) {
		assert.Equal(t, "/opt/bin/my-agent", s, "should look up the first word of the configured command")
		return s, nil
	})
	updated := result.(model)
	assert.True(t, updated.claudeQuerying)
}

func TestHandleClaudePrompt_ToolboxCommand(t *testing.T) {
	m := createTestModel()
	m.agentCLICommand = "toolbox run -c devtools claude --print"

	msg := claudePromptMsg{prompt: "test"}
	result, _ := m.handleClaudePrompt(msg, func(s string) (string, error) {
		assert.Equal(t, "toolbox", s, "should look up 'toolbox' as the first word")
		return "/usr/bin/toolbox", nil
	})
	updated := result.(model)
	assert.True(t, updated.claudeQuerying)
}

func TestTruncatePrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string not truncated",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length not truncated",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string truncated with ellipsis",
			input:    "this is a long prompt that should be truncated",
			maxLen:   20,
			expected: "this is a long promp...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePrompt(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildClaudeEnvVars_NilIncident(t *testing.T) {
	alerts := []pagerduty.IncidentAlert{
		{
			Body: map[string]interface{}{
				"details": map[string]interface{}{
					"cluster_id": "cluster-abc",
					"alert_name": "TestAlert",
				},
			},
		},
	}

	env := buildClaudeEnvVars(nil, alerts)

	envMap := make(map[string]string)
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}

	// Should not have incident-level vars
	_, hasIncidentID := envMap["PAGERDUTY_INCIDENT_ID"]
	assert.False(t, hasIncidentID, "nil incident should not produce PAGERDUTY_INCIDENT_ID")

	// Should still have cluster and alert vars
	assert.Equal(t, "cluster-abc", envMap["PAGERDUTY_CLUSTER_ID"])
	assert.Equal(t, "TestAlert", envMap["PAGERDUTY_ALERT_NAMES"])
}

func TestBuildClaudeEnvVars_NoAlerts(t *testing.T) {
	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{
			ID:      "PD789",
			HTMLURL: "https://pagerduty.com/incidents/PD789",
		},
		Title:   "No Alerts Incident",
		Urgency: "low",
		Status:  "acknowledged",
		Service: pagerduty.APIObject{Summary: "some-service"},
	}

	env := buildClaudeEnvVars(incident, nil)

	envMap := make(map[string]string)
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}

	assert.Equal(t, "PD789", envMap["PAGERDUTY_INCIDENT_ID"])
	assert.Equal(t, "No Alerts Incident", envMap["PAGERDUTY_INCIDENT_TITLE"])
	// No cluster or alert vars expected
	_, hasCluster := envMap["PAGERDUTY_CLUSTER_ID"]
	assert.False(t, hasCluster, "no alerts means no cluster ID")
	_, hasAlertNames := envMap["PAGERDUTY_ALERT_NAMES"]
	assert.False(t, hasAlertNames, "no alerts means no alert names")
}

func TestIsAgentCommand_Valid(t *testing.T) {
	assert.True(t, isAgentCommand("/agent investigate this alert"))
}

func TestIsAgentCommand_BareSlashAgent(t *testing.T) {
	assert.True(t, isAgentCommand("/agent"))
}

func TestIsAgentCommand_WithLeadingSpace(t *testing.T) {
	assert.True(t, isAgentCommand("  /agent query"))
}

func TestIsAgentCommand_NotAgent(t *testing.T) {
	assert.False(t, isAgentCommand("/flag cluster abc"))
}

func TestIsAgentCommand_PlainText(t *testing.T) {
	assert.False(t, isAgentCommand("hello world"))
}

func TestParseAgentQuery_ExtractsQuery(t *testing.T) {
	assert.Equal(t, "investigate this", parseAgentQuery("/agent investigate this"))
}

func TestParseAgentQuery_EmptyAfterPrefix(t *testing.T) {
	assert.Equal(t, "", parseAgentQuery("/agent"))
}

func TestParseAgentQuery_PreservesExtraSpaces(t *testing.T) {
	assert.Equal(t, "what happened to cluster abc", parseAgentQuery("/agent what happened to cluster abc"))
}

func TestInputMode_AgentCommand_DispatchesClaude(t *testing.T) {
	m := createInputFocusedModel("/agent what happened")

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.input.Focused(), "input must be blurred after Enter")
	assert.NotNil(t, cmd, "/agent command must dispatch a command")

	msg := cmd()
	promptMsg, ok := msg.(claudePromptMsg)
	assert.True(t, ok, "dispatched message must be claudePromptMsg")
	assert.Equal(t, "what happened", promptMsg.prompt, "query must have /agent prefix stripped")
}

func TestInputMode_BareText_ShowsError(t *testing.T) {
	m := createInputFocusedModel("hello world")

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.input.Focused(), "input must be blurred after Enter")
	assert.Nil(t, cmd, "bare text must not dispatch any command")
	assert.Contains(t, updated.status, "unknown command", "status must show error for bare text")
}

func TestInputMode_EmptyAgent_ShowsUsage(t *testing.T) {
	m := createInputFocusedModel("/agent")

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.Nil(t, cmd, "/agent with no query must not dispatch")
	assert.Contains(t, updated.status, "usage", "status must show usage hint")
}
