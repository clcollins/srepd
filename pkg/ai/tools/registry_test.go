package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/clcollins/srepd/pkg/ai/policy"
	"github.com/clcollins/srepd/pkg/ai/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_DuplicateRejection(t *testing.T) {
	reg := tools.NewRegistry()
	schema := []byte(`{"type":"object","properties":{}}`)

	err := reg.Register(tools.Tool{
		Name:    "test_tool",
		Schema:  schema,
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil },
	})
	require.NoError(t, err)

	err = reg.Register(tools.Tool{
		Name:    "test_tool",
		Schema:  schema,
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil },
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_BetaToolsShape(t *testing.T) {
	reg := tools.NewRegistry()
	schema := []byte(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)

	require.NoError(t, reg.Register(tools.Tool{
		Name:        "get_incident",
		Description: "Get a PagerDuty incident",
		Class:       policy.ClassRead,
		Schema:      schema,
		Handler:     func(_ context.Context, _ json.RawMessage) (string, error) { return "{}", nil },
	}))
	require.NoError(t, reg.Register(tools.Tool{
		Name:        "list_queue",
		Description: "List incident queue",
		Class:       policy.ClassRead,
		Schema:      []byte(`{"type":"object","properties":{}}`),
		Handler:     func(_ context.Context, _ json.RawMessage) (string, error) { return "[]", nil },
	}))

	params := reg.BetaTools()
	require.Len(t, params, 2)

	assert.Equal(t, "get_incident", params[0].OfTool.Name)
	desc := params[0].GetDescription()
	require.NotNil(t, desc)
	assert.Equal(t, "Get a PagerDuty incident", *desc)
	assert.Equal(t, "list_queue", params[1].OfTool.Name)

	data, err := json.Marshal(params)
	require.NoError(t, err)
	assert.Contains(t, string(data), "get_incident")
	assert.Contains(t, string(data), "list_queue")
}

func TestRegistry_ToolsReturnsACopy(t *testing.T) {
	reg := tools.NewRegistry()
	schema := []byte(`{"type":"object","properties":{}}`)
	require.NoError(t, reg.Register(tools.Tool{
		Name:    "test",
		Schema:  schema,
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil },
	}))

	copy1 := reg.Tools()
	copy2 := reg.Tools()
	assert.Equal(t, len(copy1), len(copy2))
	copy1[0].Name = "mutated"
	assert.NotEqual(t, copy1[0].Name, copy2[0].Name)
}

func TestGatedBetaTools_AskCallsCallback(t *testing.T) {
	reg := tools.NewRegistry()
	schema := []byte(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`)

	require.NoError(t, reg.Register(tools.Tool{
		Name:        "write_note",
		Description: "Write a note",
		Class:       policy.ClassWriteLocal,
		Schema:      schema,
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
	}))

	var askCalled bool
	var askedTool string
	gated := reg.GatedBetaTools(
		func(toolName string, _ policy.Class, _ json.RawMessage) policy.Decision {
			return policy.Ask
		},
		func(toolName string, _ json.RawMessage) {
			askCalled = true
			askedTool = toolName
		},
	)

	results, err := gated[0].Execute(context.Background(), json.RawMessage(`{"id":"INC-1"}`))
	require.NoError(t, err)
	assert.True(t, askCalled)
	assert.Equal(t, "write_note", askedTool)
	assert.Contains(t, results[0].OfText.Text, "approval")
}
