package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"unicode/utf8"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/clcollins/srepd/pkg/ai/policy"
)

// MaxResponseBytes is the truncation cap for tool handler output.
const MaxResponseBytes = 8192

// Tool defines a tool that can be registered and executed.
type Tool struct {
	Name        string
	Description string
	Class       policy.Class
	Schema      []byte
	Handler     func(ctx context.Context, input json.RawMessage) (string, error)
}

// Registry holds registered tools in insertion order.
type Registry struct {
	mu    sync.RWMutex
	tools []Tool
	names map[string]struct{}
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		names: make(map[string]struct{}),
	}
}

// Register adds a tool. Returns an error if a tool with the same name is already registered.
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.names[t.Name]; exists {
		return fmt.Errorf("tool %q already registered", t.Name)
	}
	r.names[t.Name] = struct{}{}
	r.tools = append(r.tools, t)
	return nil
}

// Tools returns a copy of all registered tools.
func (r *Registry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Tool, len(r.tools))
	copy(result, r.tools)
	return result
}

// BetaTools returns SDK-compatible tool params for the tool runner.
func (r *Registry) BetaTools() []anthropic.BetaToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	params := make([]anthropic.BetaToolUnionParam, 0, len(r.tools))
	for _, t := range r.tools {
		var schema anthropic.BetaToolInputSchemaParam
		if err := schema.UnmarshalJSON(t.Schema); err != nil {
			continue
		}
		desc := t.Description
		params = append(params, anthropic.BetaToolUnionParam{
			OfTool: &anthropic.BetaToolParam{
				Name:        t.Name,
				Description: anthropic.String(desc),
				InputSchema: schema,
			},
		})
	}
	return params
}

// gatedTool wraps a Tool with policy enforcement, implementing anthropic.BetaTool.
type gatedTool struct {
	tool   Tool
	decide func(toolName string, class policy.Class, input json.RawMessage) policy.Decision
	onAsk  func(toolName string, input json.RawMessage)
}

func (g *gatedTool) Name() string        { return g.tool.Name }
func (g *gatedTool) Description() string { return g.tool.Description }
func (g *gatedTool) InputSchema() anthropic.BetaToolInputSchemaParam {
	var schema anthropic.BetaToolInputSchemaParam
	_ = schema.UnmarshalJSON(g.tool.Schema)
	return schema
}

func (g *gatedTool) Execute(ctx context.Context, input json.RawMessage) ([]anthropic.BetaToolResultBlockParamContentUnion, error) {
	decision := g.decide(g.tool.Name, g.tool.Class, input)
	switch decision {
	case policy.Deny:
		return []anthropic.BetaToolResultBlockParamContentUnion{
			{OfText: &anthropic.BetaTextBlockParam{Text: "Permission denied: tool call not allowed by current policy"}},
		}, nil
	case policy.Ask:
		if g.onAsk != nil {
			g.onAsk(g.tool.Name, input)
		}
		return []anthropic.BetaToolResultBlockParamContentUnion{
			{OfText: &anthropic.BetaTextBlockParam{Text: "Awaiting user approval — this action requires confirmation"}},
		}, nil
	default:
		result, err := g.tool.Handler(ctx, input)
		if err != nil {
			return nil, err
		}
		truncated := Truncate(result, MaxResponseBytes)
		return []anthropic.BetaToolResultBlockParamContentUnion{
			{OfText: &anthropic.BetaTextBlockParam{Text: truncated}},
		}, nil
	}
}

// GatedBetaTools wraps each registered tool with policy enforcement.
// The decide function is called before every handler invocation.
// onAsk is called when a tool call requires user approval (may be nil).
func (r *Registry) GatedBetaTools(
	decide func(toolName string, class policy.Class, input json.RawMessage) policy.Decision,
	onAsk func(toolName string, input json.RawMessage),
) []anthropic.BetaTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]anthropic.BetaTool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, &gatedTool{
			tool:   t,
			decide: decide,
			onAsk:  onAsk,
		})
	}
	return result
}

// Truncate shortens s to maxBytes, appending a truncation marker if cut.
// The cut point is backed up to the last valid UTF-8 rune boundary to
// avoid splitting multi-byte characters.
func Truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	const marker = "\n[truncated]"
	if maxBytes <= 0 {
		return ""
	}
	if maxBytes <= len(marker) {
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		return s[:cut]
	}
	cut := maxBytes - len(marker)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + marker
}
