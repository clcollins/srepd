package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/clcollins/srepd/pkg/ai"
	"github.com/clcollins/srepd/pkg/ai/policy"
	"github.com/clcollins/srepd/pkg/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWatcherInvestigateCmd_DenyEverything_ProductionPath is the production-path
// headline test. It drives through watcherInvestigateCmd with:
// - A real tool registry built the same way production does
// - policy.Decide (not a hand-rolled closure) via the investigationConfig
// - A fake HTTP backend returning a tool_use response
// - A Deny-everything config (ModePlan + ClassWriteLocal tools)
//
// The test asserts zero handler invocations under the deny config.
func TestWatcherInvestigateCmd_DenyEverything_ProductionPath(t *testing.T) {
	var handlerCalls atomic.Int64
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			resp := map[string]interface{}{
				"id": "msg_test_001", "type": "message", "role": "assistant",
				"model": "claude-sonnet-4-6",
				"content": []map[string]interface{}{
					{"type": "tool_use", "id": "toolu_test_001", "name": "write_note",
						"input": map[string]string{"id": "INC-001", "content": "test note"}},
				},
				"stop_reason": "tool_use",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			resp := map[string]interface{}{
				"id": "msg_test_002", "type": "message", "role": "assistant",
				"model": "claude-sonnet-4-6",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Investigation complete.\n```json\n{\"tier\": \"noteworthy\", \"summary\": \"test observation\"}\n```"},
				},
				"stop_reason": "end_turn",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)
	factory := &client.Beta.Messages

	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(tools.Tool{
		Name:        "write_note",
		Description: "Write a note to an incident",
		Class:       policy.ClassWriteLocal,
		Schema:      []byte(`{"type":"object","properties":{"id":{"type":"string"},"content":{"type":"string"}},"required":["id"]}`),
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			handlerCalls.Add(1)
			return `{"ok":true}`, nil
		},
	}))

	cfg := investigationConfig{
		maxToolTurns: 6,
		timeout:      5 * time.Second,
		policyConfig: policy.Config{Mode: policy.ModePlan},
	}

	cmd := watcherInvestigateCmd(
		factory,
		reg,
		cfg,
		"You are a test system prompt.",
		"Test observation",
		"Test context",
		"claude-sonnet-4-6",
		nil,
	)

	msg := cmd()
	result, ok := msg.(investigationMsg)
	require.True(t, ok, "expected investigationMsg")
	assert.NoError(t, result.err)

	assert.Equal(t, int64(0), handlerCalls.Load(),
		"Deny-everything policy via production path must produce zero handler invocations")
}

// TestWatcherInvestigateCmd_AllowPath_HandlerRuns verifies that when the policy
// config allows the tool class, the handler executes through the production path.
func TestWatcherInvestigateCmd_AllowPath_HandlerRuns(t *testing.T) {
	var handlerCalls atomic.Int64
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			resp := map[string]interface{}{
				"id": "msg_test_001", "type": "message", "role": "assistant",
				"model": "claude-sonnet-4-6",
				"content": []map[string]interface{}{
					{"type": "tool_use", "id": "toolu_test_001", "name": "get_incident",
						"input": map[string]string{"id": "INC-001"}},
				},
				"stop_reason": "tool_use",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			resp := map[string]interface{}{
				"id": "msg_test_002", "type": "message", "role": "assistant",
				"model": "claude-sonnet-4-6",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Done.\n```json\n{\"tier\": \"noteworthy\", \"summary\": \"test\"}\n```"},
				},
				"stop_reason": "end_turn",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)

	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(tools.Tool{
		Name:        "get_incident",
		Description: "Get incident details",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			handlerCalls.Add(1)
			return `{"id":"INC-001","title":"Test"}`, nil
		},
	}))

	cfg := investigationConfig{
		maxToolTurns: 6,
		timeout:      5 * time.Second,
		policyConfig: policy.Config{Mode: policy.ModeInteractive},
	}

	cmd := watcherInvestigateCmd(
		&client.Beta.Messages,
		reg,
		cfg,
		"Test prompt",
		"Test observation",
		"Test context",
		"claude-sonnet-4-6",
		nil,
	)

	msg := cmd()
	result, ok := msg.(investigationMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)

	assert.Greater(t, handlerCalls.Load(), int64(0),
		"Allow policy via production path must invoke the handler")
}

// capturingFactory wraps a ToolRunnerFactory and records the params passed to
// NewToolRunner. This lets tests assert the model and other params without
// relying on HTTP interception.
type capturingFactory struct {
	inner          ToolRunnerFactory
	capturedParams anthropic.BetaToolRunnerParams
}

func (f *capturingFactory) NewToolRunner(betaTools []anthropic.BetaTool, params anthropic.BetaToolRunnerParams, opts ...option.RequestOption) *anthropic.BetaToolRunner {
	f.capturedParams = params
	return f.inner.NewToolRunner(betaTools, params, opts...)
}

func TestWatcherInvestigateCmd_ModelPlumbing_ExplicitModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id": "msg_test_model", "type": "message", "role": "assistant",
			"model": "claude-sonnet-4-6",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Done.\n```json\n{\"tier\": \"silent\", \"summary\": \"ok\"}\n```"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)
	factory := &capturingFactory{inner: &client.Beta.Messages}

	reg := tools.NewRegistry()
	cfg := investigationConfig{
		maxToolTurns: 2,
		timeout:      5 * time.Second,
		policyConfig: policy.Config{Mode: policy.ModeInteractive},
	}

	configuredModel := "us.anthropic.claude-sonnet-4-6"
	cmd := watcherInvestigateCmd(factory, reg, cfg, "test prompt", "obs", "ctx", configuredModel, nil)
	msg := cmd()
	result, ok := msg.(investigationMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)

	assert.Equal(t, anthropic.Model(configuredModel), factory.capturedParams.Model,
		"runner must receive the explicitly configured model, not a hardcoded default")
}

func TestWatcherInvestigateCmd_ModelPlumbing_EmptyModelReturnsError(t *testing.T) {
	reg := tools.NewRegistry()
	cfg := investigationConfig{
		maxToolTurns: 2,
		timeout:      5 * time.Second,
		policyConfig: policy.Config{Mode: policy.ModeInteractive},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called when model is empty")
	}))
	defer server.Close()

	client := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)

	cmd := watcherInvestigateCmd(&client.Beta.Messages, reg, cfg, "prompt", "obs", "ctx", "", nil)
	msg := cmd()
	result, ok := msg.(investigationMsg)
	require.True(t, ok)
	assert.Error(t, result.err, "empty model must return an error, not silently substitute a default")
	assert.Contains(t, result.err.Error(), "model not configured")
}

func TestWatcherInvestigateCmd_AskDedup_SameToolAndInput(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount <= 3 {
			resp := map[string]interface{}{
				"id": "msg_dedup", "type": "message", "role": "assistant",
				"model": "claude-sonnet-4-6",
				"content": []map[string]interface{}{
					{"type": "tool_use", "id": fmt.Sprintf("toolu_dedup_%d", callCount),
						"name":  "write_note",
						"input": map[string]string{"id": "INC-001", "content": "note"}},
				},
				"stop_reason": "tool_use",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			resp := map[string]interface{}{
				"id": "msg_dedup_end", "type": "message", "role": "assistant",
				"model": "claude-sonnet-4-6",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Done.\n```json\n{\"tier\": \"silent\", \"summary\": \"ok\"}\n```"},
				},
				"stop_reason": "end_turn",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)

	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(tools.Tool{
		Name:        "write_note",
		Description: "Write a note",
		Class:       policy.ClassWriteLocal,
		Schema:      []byte(`{"type":"object","properties":{"id":{"type":"string"},"content":{"type":"string"}},"required":["id"]}`),
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
	}))

	cfg := investigationConfig{
		maxToolTurns: 6,
		timeout:      5 * time.Second,
		policyConfig: policy.Config{Mode: policy.ModeInteractive},
	}

	cmd := watcherInvestigateCmd(
		&client.Beta.Messages, reg, cfg,
		"test prompt", "obs", "ctx", "claude-sonnet-4-6", nil,
	)

	msg := cmd()
	result, ok := msg.(investigationMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)

	assert.Equal(t, 1, len(result.toolAsks),
		"same tool+input asked three times must be deduped to one entry")
	assert.Equal(t, "write_note", result.toolAsks[0].toolName)
}

func TestIsAnthropicFamily(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"anthropic", true},
		{"anthropic-bedrock", true},
		{"anthropic-vertex", true},
		{"ollama", false},
		{"openai", false},
		{"ramalama", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isAnthropicFamily(tt.name))
		})
	}
}

func TestDefaultInvestigationConfig(t *testing.T) {
	cfg := defaultInvestigationConfig()
	assert.Equal(t, 6, cfg.maxToolTurns)
	assert.Equal(t, 90*time.Second, cfg.timeout)
	assert.Equal(t, policy.ModeInteractive, cfg.policyConfig.Mode)
}

func TestExtractToolRunnerFactory_NilProvider(t *testing.T) {
	assert.Nil(t, extractToolRunnerFactory(nil))
}

func TestExtractToolRunnerFactory_NonAnthropicProvider(t *testing.T) {
	mock := &ai.MockProvider{ProviderName: "ollama"}
	assert.Nil(t, extractToolRunnerFactory(mock),
		"non-Anthropic provider must return nil factory")
}

type mockBetaMessagesProvider struct {
	ai.MockProvider
}

func (m *mockBetaMessagesProvider) BetaMessages() *anthropic.BetaMessageService {
	return nil
}

func TestExtractToolRunnerFactory_WithBetaMessages(t *testing.T) {
	mock := &mockBetaMessagesProvider{}
	result := extractToolRunnerFactory(mock)
	assert.Nil(t, result,
		"BetaMessages() returns nil service so factory should be nil")
}

type realBetaMessagesProvider struct {
	ai.MockProvider
	svc *anthropic.BetaMessageService
}

func (r *realBetaMessagesProvider) BetaMessages() *anthropic.BetaMessageService {
	return r.svc
}

func TestExtractToolRunnerFactory_NonNilService_ReturnsUsableFactory(t *testing.T) {
	svc := anthropic.NewBetaMessageService()
	mock := &realBetaMessagesProvider{svc: &svc}
	factory := extractToolRunnerFactory(mock)
	require.NotNil(t, factory,
		"anthropic-family provider with non-nil BetaMessages must yield a non-nil factory")

	runner := factory.NewToolRunner(nil, anthropic.BetaToolRunnerParams{
		BetaMessageNewParams: anthropic.BetaMessageNewParams{
			MaxTokens: 1,
		},
	})
	assert.NotNil(t, runner,
		"factory must be able to construct a BetaToolRunner")
}
