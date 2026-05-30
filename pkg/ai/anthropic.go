package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	// defaultMaxTokens is the default maximum number of tokens in the
	// response from the Anthropic API.
	defaultMaxTokens = 4096

	// defaultModel is the default model used when none is specified in
	// the config.
	defaultModel = "claude-sonnet-4-6"
)

// anthropicProvider implements the Provider interface using the Anthropic
// Messages API via the official Go SDK.
type anthropicProvider struct {
	client anthropic.Client
	model  string
}

// newAnthropicProvider creates a new Anthropic provider with the given
// config and resolved API key.
func newAnthropicProvider(cfg Config, apiKey string) (*anthropicProvider, error) {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if cfg.Endpoint != "" {
		opts = append(opts, option.WithBaseURL(cfg.Endpoint))
	}

	client := anthropic.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = defaultModel
	}

	return &anthropicProvider{
		client: client,
		model:  model,
	}, nil
}

// Name returns the provider name.
func (p *anthropicProvider) Name() string {
	return "anthropic"
}

// Query sends a system prompt and user prompt to the Anthropic Messages
// API and returns the complete response text. It concatenates all text
// blocks from the response.
func (p *anthropicProvider) Query(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: defaultMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(userPrompt),
			),
		},
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	message, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("anthropic query failed: %w", err)
	}

	var result strings.Builder
	for _, block := range message.Content {
		switch block.Type {
		case "text":
			textBlock := block.AsText()
			result.WriteString(textBlock.Text)
		}
	}

	return result.String(), nil
}

// StreamQuery sends a system prompt and user prompt to the Anthropic
// Messages API and streams response tokens to the provided channel. The
// channel is closed when streaming completes.
func (p *anthropicProvider) StreamQuery(ctx context.Context, systemPrompt string, userPrompt string, ch chan<- string) error {
	defer close(ch)

	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: defaultMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(userPrompt),
			),
		},
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	for stream.Next() {
		event := stream.Current()
		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				ch <- deltaVariant.Text
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("anthropic stream failed: %w", err)
	}

	return nil
}
