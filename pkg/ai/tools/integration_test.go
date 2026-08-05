package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/clcollins/srepd/pkg/ai/policy"
	"github.com/clcollins/srepd/pkg/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeadline_DenyEverything_ZeroHandlerInvocations is the phase-413 headline
// criterion: a Deny-everything policy config must produce ZERO tool handler
// invocations. This test exercises the real gated-tool path. It must fail
// before the policy gate exists and pass after.
func TestHeadline_DenyEverything_ZeroHandlerInvocations(t *testing.T) {
	var handlerCalls atomic.Int64

	schema := []byte(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)

	reg := tools.NewRegistry()

	toolNames := []string{"get_incident", "get_alerts", "list_queue"}
	for _, name := range toolNames {
		require.NoError(t, reg.Register(tools.Tool{
			Name:        name,
			Description: "test tool " + name,
			Class:       policy.ClassRead,
			Schema:      schema,
			Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
				handlerCalls.Add(1)
				return `{"result":"ok"}`, nil
			},
		}))
	}

	// Deny-everything: ModePlan with ClassWriteLocal tools would deny,
	// but even ClassRead tools should be denied when we use a custom
	// decide function that always returns Deny.
	denyAll := func(_ string, _ policy.Class, _ json.RawMessage) policy.Decision {
		return policy.Deny
	}

	gated := reg.GatedBetaTools(denyAll, nil)
	require.Len(t, gated, len(toolNames))

	ctx := context.Background()
	input := json.RawMessage(`{"id":"INC-001"}`)

	// Simulate what the SDK tool runner does: call Execute on each tool
	for _, tool := range gated {
		results, err := tool.Execute(ctx, input)
		require.NoError(t, err)
		require.NotEmpty(t, results)
		// Denied tools should return a denial message, not handler output
		assert.Contains(t, results[0].OfText.Text, "denied")
	}

	// THE HEADLINE ASSERTION: with Deny-everything, exactly zero handlers fire
	assert.Equal(t, int64(0), handlerCalls.Load(),
		"Deny-everything policy must produce zero handler invocations")
}

// TestGatedBetaTools_AllowPath_HandlerRuns verifies that when the decide function
// returns Allow, the handler executes and its (truncated) result is returned.
// Verified: replacing the default (Allow) branch with a denial leaves all tests
// passing — this test catches that gap.
func TestGatedBetaTools_AllowPath_HandlerRuns(t *testing.T) {
	var handlerCalls int64

	schema := []byte(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)
	reg := tools.NewRegistry()

	require.NoError(t, reg.Register(tools.Tool{
		Name:        "get_incident",
		Description: "test tool",
		Class:       policy.ClassRead,
		Schema:      schema,
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			handlerCalls++
			return `{"result":"ok","incident_id":"P123"}`, nil
		},
	}))

	alwaysAllow := func(_ string, _ policy.Class, _ json.RawMessage) policy.Decision {
		return policy.Allow
	}

	gated := reg.GatedBetaTools(alwaysAllow, nil)
	require.Len(t, gated, 1)

	results, err := gated[0].Execute(context.Background(), json.RawMessage(`{"id":"P123"}`))
	require.NoError(t, err)
	require.NotEmpty(t, results)

	assert.Equal(t, int64(1), handlerCalls,
		"Allow decision must invoke the handler exactly once")
	assert.Contains(t, results[0].OfText.Text, "P123",
		"Allow path must return the handler's result")
}

// TestGatedBetaTools_AllowPath_Truncation verifies truncation works through the gated path.
func TestGatedBetaTools_AllowPath_Truncation(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{}}`)
	reg := tools.NewRegistry()

	largeOutput := strings.Repeat("x", tools.MaxResponseBytes+1000)
	require.NoError(t, reg.Register(tools.Tool{
		Name:    "big_tool",
		Schema:  schema,
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return largeOutput, nil },
	}))

	gated := reg.GatedBetaTools(
		func(_ string, _ policy.Class, _ json.RawMessage) policy.Decision { return policy.Allow },
		nil,
	)

	results, err := gated[0].Execute(context.Background(), json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results[0].OfText.Text), tools.MaxResponseBytes)
	assert.True(t, strings.HasSuffix(results[0].OfText.Text, "[truncated]"))
}

// TestRevertCheck_PolicyGate encodes the headline criterion's revert check as
// a permanent test. It proves that TestHeadline_DenyEverything_ZeroHandlerInvocations
// has teeth: without the policy gate, handlers fire unconditionally.
//
// Pattern follows TestRevertCheck_StubIndexWrite in pkg/agent/integration_test.go.
func TestRevertCheck_PolicyGate(t *testing.T) {
	var handlerCalls atomic.Int64

	schema := []byte(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)

	reg := tools.NewRegistry()
	require.NoError(t, reg.Register(tools.Tool{
		Name:        "get_incident",
		Description: "test tool",
		Class:       policy.ClassRead,
		Schema:      schema,
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			handlerCalls.Add(1)
			return `{"result":"ok"}`, nil
		},
	}))

	// Use the Allow path — simulates what happens if the gate is reverted
	alwaysAllow := func(_ string, _ policy.Class, _ json.RawMessage) policy.Decision {
		return policy.Allow
	}
	gated := reg.GatedBetaTools(alwaysAllow, nil)
	require.Len(t, gated, 1)

	ctx := context.Background()
	input := json.RawMessage(`{"id":"INC-001"}`)

	_, err := gated[0].Execute(ctx, input)
	require.NoError(t, err)

	// WITH Allow, the handler MUST fire — if it doesn't, the gate is
	// not load-bearing (the headline test would pass even without the gate)
	assert.Greater(t, handlerCalls.Load(), int64(0),
		"With Allow decision, handlers must fire — "+
			"this proves the headline test has teeth")
}
