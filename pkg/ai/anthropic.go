package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/log"
)

const (
	anthropicDefaultMaxTokens = 4096
	anthropicDefaultModel     = "claude-sonnet-4-6"
)

type anthropicProvider struct {
	client anthropic.Client
	model  string
	name   string
}

func newAnthropicProvider(cfg Config, apiKey string) (*anthropicProvider, error) {
	opts := []option.RequestOption{}

	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}

	if cfg.Endpoint != "" {
		opts = append(opts, option.WithBaseURL(cfg.Endpoint))
	}

	client := anthropic.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = anthropicDefaultModel
	}

	return &anthropicProvider{
		client: client,
		model:  model,
		name:   "anthropic",
	}, nil
}

func (p *anthropicProvider) Name() string {
	return p.name
}

// SupportsStreaming reports that this provider streams tokens via StreamQuery.
func (p *anthropicProvider) SupportsStreaming() bool { return true }

func (p *anthropicProvider) Query(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	log.Debug("anthropic.Query", "model", p.model)
	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: anthropicDefaultMaxTokens,
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
		if block.Type == "text" {
			result.WriteString(block.AsText().Text)
		}
	}

	return result.String(), nil
}

func (p *anthropicProvider) StreamQuery(ctx context.Context, systemPrompt string, userPrompt string, ch chan<- string) error {
	defer close(ch)
	log.Debug("anthropic.StreamQuery", "model", p.model)

	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: anthropicDefaultMaxTokens,
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
