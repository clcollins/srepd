package tui

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	tea "github.com/charmbracelet/bubbletea"
	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/stretchr/testify/assert"
)

// mockCommandExecutor records the arguments passed to Execute and returns
// pre-configured results, eliminating all real subprocess calls.
type mockCommandExecutor struct {
	name      string
	args      []string
	stdinData string
	env       []string

	stdout []byte
	stderr string
	err    error
}

func (m *mockCommandExecutor) Execute(_ context.Context, name string, args []string, stdin io.Reader, env []string) ([]byte, string, error) {
	m.name = name
	m.args = args
	m.env = env
	if stdin != nil {
		data, _ := io.ReadAll(stdin)
		m.stdinData = string(data)
	}
	return m.stdout, m.stderr, m.err
}

func TestInputCharLimit_Increased(t *testing.T) {
	// newTextInput should have a CharLimit of 500 (increased from 32)
	// to allow longer prompts for Claude queries
	input := newTextInput()
	assert.Equal(t, 500, input.CharLimit, "CharLimit should be 500 for Claude prompts")
}

func TestClaudePrompt_DispatchesCommand(t *testing.T) {
	// When the user is in input focus mode and presses Enter with :agent prefix,
	// a claudePromptMsg should be dispatched with the query text (prefix stripped),
	// the input should be reset and blurred.

	m := createTestModel()
	m.input = newTextInput()
	m.input.Focus()
	m.input.SetValue(":agent investigate this alert")

	// Simulate pressing Enter in input focus mode
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := switchInputFocusMode(m, enterMsg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.input.Focused(), "Input should blur after :agent dispatch")
	assert.Equal(t, "", updatedModel.input.Value(), "Input value should be cleared after dispatch")

	// A command should be returned
	assert.NotNil(t, cmd, "Enter in input mode should return a command")

	// The command should produce a claudePromptMsg with prefix stripped
	msg := cmd()
	promptMsg, ok := msg.(claudePromptMsg)
	assert.True(t, ok, "Command should produce a claudePromptMsg, got %T", msg)
	assert.Equal(t, "investigate this alert", promptMsg.prompt, "Prompt text should have :agent prefix stripped")
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

func TestClaudeNotFound_ShowsFlash(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.agentCLICommand = "nonexistent-binary --print"

	msg := claudePromptMsg{prompt: "test query"}

	_, cmd := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "", fmt.Errorf("not found: %s", s)
	})

	assert.NotNil(t, cmd, "should return flash notification command")
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

func TestClaudeResponse_RendersInWatcherPane(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.claudeQuerying = true
	m.apiInProgress = true

	msg := claudeResponseMsg{
		response: "Based on the alert, the cluster appears to have high CPU usage.",
		err:      nil,
	}

	result, _ := m.Update(msg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.claudeQuerying, "claudeQuerying should be false after response")
	assert.False(t, updatedModel.apiInProgress, "apiInProgress should be false after response")
	assert.False(t, updatedModel.viewingIncident, "viewingIncident should remain false — response goes to watcher pane")
	assert.True(t, updatedModel.watcherExpanded, "watcher pane should auto-expand on response")
	assert.Equal(t, 1, updatedModel.watcherBuffer.Len(), "response should be appended to watcher buffer")
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

	result, cmd := m.Update(msg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.claudeQuerying, "claudeQuerying should be false after error")
	assert.False(t, updatedModel.apiInProgress, "apiInProgress should be false after error")
	assert.NotNil(t, cmd, "should return flash notification for error")
}

func TestClaudeResponse_EmptyResponse(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.claudeQuerying = true
	m.apiInProgress = true

	msg := claudeResponseMsg{
		response: "",
		err:      nil,
	}

	result, cmd := m.Update(msg)
	updatedModel := result.(model)

	assert.False(t, updatedModel.claudeQuerying, "claudeQuerying should be false after empty response")
	assert.NotNil(t, cmd, "should return flash notification for empty response")
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

func TestDefaultAgentSystemPrompt(t *testing.T) {
	defaultPrompt := pkgconfig.DefaultOptionalKeys["agent_system_prompt"]
	assert.Contains(t, defaultPrompt, "read-only")
	assert.Contains(t, defaultPrompt, "investigation")
}

func TestDefaultWatcherSystemPrompt(t *testing.T) {
	defaultPrompt := pkgconfig.DefaultOptionalKeys["watcher_system_prompt"]
	assert.Contains(t, defaultPrompt, "SRE assistant")
	assert.Contains(t, defaultPrompt, "destructive")
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
	m.input.SetValue(":agent query text")

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

func TestClaudePrompt_AutoExpandsWatcher(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.watcherExpanded = false
	m.selectedIncident = &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "Q123"},
	}

	msg := claudePromptMsg{prompt: "what is wrong with this cluster?"}

	result, cmd := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "/usr/bin/" + s, nil
	})
	updatedModel := result.(model)

	assert.True(t, updatedModel.claudeQuerying, "Should start querying")
	assert.True(t, updatedModel.watcherExpanded, "Watcher pane should auto-expand on query")
	assert.NotNil(t, cmd, "Should dispatch query command")
}

func TestNewTextInputWidth(t *testing.T) {
	// newTextInput width should be set to 120 (will be adjusted by window resize)
	input := newTextInput()
	assert.Equal(t, 120, input.Width, "Input width should be 120")
}

func TestClaudeResponse_AppendsToWatcherBuffer(t *testing.T) {
	m := createTestModel()
	m.incidentCache = make(map[string]*cachedIncidentData)
	m.claudeQuerying = true
	m.apiInProgress = true

	msg := claudeResponseMsg{
		response: "The cluster is experiencing high CPU usage.",
		err:      nil,
	}

	result, cmd := m.Update(msg)
	updatedModel := result.(model)

	assert.Equal(t, 1, updatedModel.watcherBuffer.Len(), "should have placeholder entry for typewriter")
	assert.NotNil(t, updatedModel.typewriter, "typewriter should be active")
	assert.NotNil(t, cmd, "should return typewriter tick command")
}

func TestDefaultLookPath_NoPanic(t *testing.T) {
	t.Run("defaultLookPath does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_, _ = defaultLookPath("nonexistent-binary-12345")
		}, "defaultLookPath should not panic for missing binaries")
	})
}

func TestAgentQuery_EmptyCommand(t *testing.T) {
	mock := &mockCommandExecutor{}
	cmd := agentQuery(mock, "", "test system", "test prompt", "", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.Error(t, resp.err)
	assert.Contains(t, resp.err.Error(), "agent_cli_command is empty")
}

func TestAgentQuery_CommandParsing(t *testing.T) {
	mock := &mockCommandExecutor{stdout: []byte("hello world")}
	cmd := agentQuery(mock, "echo hello world", "", "ignored", "", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)
	assert.Equal(t, "hello world", resp.response)
	assert.Equal(t, "echo", mock.name)
	assert.Equal(t, []string{"hello", "world"}, mock.args)
}

func TestAgentQuery_MultiWordCommand(t *testing.T) {
	mock := &mockCommandExecutor{stdout: []byte("test")}
	cmd := agentQuery(mock, "echo -n test", "", "ignored", "", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)
	assert.Equal(t, "test", resp.response)
	assert.Equal(t, "echo", mock.name)
	assert.Equal(t, []string{"-n", "test"}, mock.args)
}

func TestAgentQuery_CommandNotFound(t *testing.T) {
	mock := &mockCommandExecutor{err: fmt.Errorf("exec: \"nonexistent-binary-99999\": executable file not found in $PATH")}
	cmd := agentQuery(mock, "nonexistent-binary-99999 --flag", "", "test", "", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.Error(t, resp.err)
	assert.Contains(t, resp.err.Error(), "agent error")
}

func TestAgentQuery_StderrCaptured(t *testing.T) {
	mock := &mockCommandExecutor{
		stderr: "oops",
		err:    fmt.Errorf("exit status 1"),
	}
	cmd := agentQuery(mock, "/bin/fail", "", "test", "", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.Error(t, resp.err)
	assert.Contains(t, resp.err.Error(), "oops")
}

func TestAgentQuery_PassesEnvVars(t *testing.T) {
	mock := &mockCommandExecutor{stdout: []byte("ok")}

	incident := &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: "PD999"},
		Title:     "Test",
		Urgency:   "high",
		Status:    "triggered",
		Service:   pagerduty.APIObject{Summary: "svc"},
	}

	cmd := agentQuery(mock, "/bin/agent", "", "test", "", incident, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)

	envMap := make(map[string]string)
	for _, e := range mock.env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}
	assert.Equal(t, "PD999", envMap["PAGERDUTY_INCIDENT_ID"])
}

func TestAgentQuery_PipesStdin(t *testing.T) {
	mock := &mockCommandExecutor{stdout: []byte("ok")}
	cmd := agentQuery(mock, "cat", "test system prompt", "user question here", "", nil, nil)
	msg := cmd()
	resp, ok := msg.(claudeResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, resp.err)
	assert.Contains(t, mock.stdinData, "user question here")
	assert.Contains(t, mock.stdinData, "test system prompt")
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

func TestHandleClaudePrompt_WhitespaceOnlyCommand(t *testing.T) {
	// A whitespace-only agentCLICommand is non-empty, so it skips the "" default,
	// but strings.Fields returns an empty slice — indexing [0] would panic. The
	// handler must guard this (mirroring agentQuery's len==0 check) and flash an
	// error instead of panicking. Library code must never panic.
	m := createTestModel()
	m.agentCLICommand = "   "

	msg := claudePromptMsg{prompt: "test"}
	assert.NotPanics(t, func() {
		result, cmd := m.handleClaudePrompt(msg, func(s string) (string, error) {
			return s, nil
		})
		updated := result.(model)
		assert.False(t, updated.claudeQuerying, "should not start a query on an empty command")
		assert.NotNil(t, cmd, "should flash a notification rather than panic")
	})
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
	assert.True(t, isAgentCommand(":agent investigate this alert"))
}

func TestIsAgentCommand_BareSlashAgent(t *testing.T) {
	assert.True(t, isAgentCommand(":agent"))
}

func TestIsAgentCommand_WithLeadingSpace(t *testing.T) {
	assert.True(t, isAgentCommand("  :agent query"))
}

func TestIsAgentCommand_NotAgent(t *testing.T) {
	assert.False(t, isAgentCommand(":flag cluster abc"))
}

func TestIsAgentCommand_PlainText(t *testing.T) {
	assert.False(t, isAgentCommand("hello world"))
}

func TestParseAgentQuery_ExtractsQuery(t *testing.T) {
	assert.Equal(t, "investigate this", parseAgentQuery(":agent investigate this"))
}

func TestParseAgentQuery_EmptyAfterPrefix(t *testing.T) {
	assert.Equal(t, "", parseAgentQuery(":agent"))
}

func TestParseAgentQuery_PreservesExtraSpaces(t *testing.T) {
	assert.Equal(t, "what happened to cluster abc", parseAgentQuery(":agent what happened to cluster abc"))
}

func TestInputMode_AgentCommand_DispatchesClaude(t *testing.T) {
	m := createInputFocusedModel(":agent what happened")

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.False(t, updated.input.Focused(), "input should blur after :agent dispatch")
	assert.NotNil(t, cmd, ":agent command must dispatch a command")

	msg := cmd()
	promptMsg, ok := msg.(claudePromptMsg)
	assert.True(t, ok, "dispatched message must be claudePromptMsg")
	assert.Equal(t, "what happened", promptMsg.prompt, "query must have :agent prefix stripped")
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
	m := createInputFocusedModel(":agent")

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.keyMsgHandler(keyMsg)
	updated := result.(model)

	assert.Nil(t, cmd, ":agent with no query must not dispatch")
	assert.Contains(t, updated.status, "usage", "status must show usage hint")
}

func TestIsClaudeCLI(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected bool
	}{
		{"bare claude", "claude --print", true},
		{"absolute path", "/usr/bin/claude --print", true},
		{"toolbox wrapper", "toolbox run -c devtools claude --print", true},
		{"flatpak-spawn wrapper", "flatpak-spawn --host claude --print", true},
		{"not claude", "my-agent --print", false},
		{"empty", "", false},
		{"claude-like prefix", "claude-wrapper --print", false},
		{"claude in path segment", "/opt/claude-tools/agent --print", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isClaudeCLI(tt.cmd), "isClaudeCLI(%q)", tt.cmd)
		})
	}
}

func TestParseCLIStreamLine_TextDelta(t *testing.T) {
	line := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}}`
	parsed, err := parseCLIStreamLine([]byte(line))
	assert.NoError(t, err)
	assert.Equal(t, "stream_event", parsed.Type)
	assert.NotNil(t, parsed.Event)
	assert.Equal(t, "content_block_delta", parsed.Event.Type)
	assert.NotNil(t, parsed.Event.Delta)
	assert.Equal(t, "text_delta", parsed.Event.Delta.Type)
	assert.Equal(t, "hello", parsed.Event.Delta.Text)
}

func TestParseCLIStreamLine_Result(t *testing.T) {
	line := `{"type":"result","subtype":"success","result":"final answer"}`
	parsed, err := parseCLIStreamLine([]byte(line))
	assert.NoError(t, err)
	assert.Equal(t, "result", parsed.Type)
	assert.Equal(t, "success", parsed.Subtype)
}

func TestParseCLIStreamLine_SystemEvent(t *testing.T) {
	line := `{"type":"system","subtype":"init","cwd":"/home/user"}`
	parsed, err := parseCLIStreamLine([]byte(line))
	assert.NoError(t, err)
	assert.Equal(t, "system", parsed.Type)
}

func TestBuildStreamingArgs_AppendsFlags(t *testing.T) {
	args := buildStreamingArgs([]string{"claude", "--print"})
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--verbose")
	assert.Contains(t, args, "--include-partial-messages")
}

func TestBuildStreamingArgs_DoesNotDuplicate(t *testing.T) {
	args := buildStreamingArgs([]string{"claude", "--print", "--output-format", "stream-json", "--verbose", "--include-partial-messages"})
	count := 0
	for _, a := range args {
		if a == "--include-partial-messages" {
			count++
		}
	}
	assert.Equal(t, 1, count, "should not duplicate --include-partial-messages")
	verboseCount := 0
	for _, a := range args {
		if a == "--verbose" {
			verboseCount++
		}
	}
	assert.Equal(t, 1, verboseCount, "should not duplicate --verbose")
}

func TestReadAgentStreamCmd_TextChunk(t *testing.T) {
	ch := make(chan streamEvent, 1)
	ch <- streamEvent{text: "hello"}

	cmd := readAgentStreamCmd(ch)
	msg := cmd()
	chunk, ok := msg.(agentStreamChunkMsg)
	assert.True(t, ok, "expected agentStreamChunkMsg, got %T", msg)
	assert.Equal(t, "hello", chunk.text)
}

func TestReadAgentStreamCmd_Done(t *testing.T) {
	ch := make(chan streamEvent, 1)
	ch <- streamEvent{done: true}

	cmd := readAgentStreamCmd(ch)
	msg := cmd()
	_, ok := msg.(agentStreamDoneMsg)
	assert.True(t, ok, "expected agentStreamDoneMsg, got %T", msg)
}

func TestReadAgentStreamCmd_ClosedChannel(t *testing.T) {
	ch := make(chan streamEvent)
	close(ch)

	cmd := readAgentStreamCmd(ch)
	msg := cmd()
	_, ok := msg.(agentStreamDoneMsg)
	assert.True(t, ok, "closed channel should produce agentStreamDoneMsg")
}

func TestHandleClaudePrompt_StreamingDispatch(t *testing.T) {
	m := createTestModel()
	m.agentCLICommand = "claude --print"
	m.streamResponses = true

	msg := claudePromptMsg{prompt: "test query"}
	result, cmd := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "/usr/bin/" + s, nil
	})
	updated := result.(model)

	assert.True(t, updated.claudeQuerying)
	assert.True(t, updated.apiInProgress)
	assert.NotNil(t, cmd, "should dispatch streaming command")
}

func TestHandleClaudePrompt_NonClaudeCLI_NoStreaming(t *testing.T) {
	m := createTestModel()
	m.agentCLICommand = "my-custom-agent --print"
	m.streamResponses = true

	msg := claudePromptMsg{prompt: "test query"}
	result, _ := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "/usr/bin/" + s, nil
	})
	updated := result.(model)

	assert.True(t, updated.claudeQuerying, "non-claude CLI should still query")
}

func TestHandleClaudePrompt_StreamingDisabled_NoStreaming(t *testing.T) {
	m := createTestModel()
	m.agentCLICommand = "claude --print"
	m.streamResponses = false

	msg := claudePromptMsg{prompt: "test query"}
	result, _ := m.handleClaudePrompt(msg, func(s string) (string, error) {
		return "/usr/bin/" + s, nil
	})
	updated := result.(model)

	assert.True(t, updated.claudeQuerying, "should query even with streaming disabled")
}

func TestAgentStreamStartedMsg_SetsState(t *testing.T) {
	m := createTestModel()
	m.claudeQuerying = true
	m.apiInProgress = true

	ch := make(chan streamEvent)
	cancel := func() {}
	msg := agentStreamStartedMsg{ch: ch, cancel: cancel}

	result, cmd := m.Update(msg)
	updated := result.(model)

	assert.NotNil(t, updated.agentStreamCancel)
	assert.Equal(t, "", updated.agentStreamPartial)
	assert.True(t, updated.watcherExpanded)
	assert.NotNil(t, cmd)
}

func TestAgentStreamChunkMsg_AppendsText(t *testing.T) {
	m := createTestModel()
	m.claudeQuerying = true
	m.agentStreamPartial = "hello "
	m.watcherExpanded = true
	m.watcherBuffer.Append(prefixLines(m.agentMarker, "hello "))

	ch := make(chan streamEvent, 1)
	ch <- streamEvent{text: "more"}
	msg := agentStreamChunkMsg{text: "world", ch: ch}

	result, cmd := m.Update(msg)
	updated := result.(model)

	assert.Equal(t, "hello world", updated.agentStreamPartial)
	assert.NotNil(t, cmd, "should continue reading stream")
}

func TestAgentStreamDoneMsg_ClearsState(t *testing.T) {
	m := createTestModel()
	m.claudeQuerying = true
	m.apiInProgress = true
	m.agentStreamCancel = func() {}

	msg := agentStreamDoneMsg{}

	result, _ := m.Update(msg)
	updated := result.(model)

	assert.False(t, updated.claudeQuerying)
	assert.False(t, updated.apiInProgress)
	assert.Nil(t, updated.agentStreamCancel)
}

func TestAgentStreamDoneMsg_WithError(t *testing.T) {
	m := createTestModel()
	m.claudeQuerying = true
	m.apiInProgress = true

	msg := agentStreamDoneMsg{err: fmt.Errorf("stream failed")}

	result, cmd := m.Update(msg)
	updated := result.(model)

	assert.False(t, updated.claudeQuerying)
	assert.NotNil(t, cmd, "should flash error notification")
}
