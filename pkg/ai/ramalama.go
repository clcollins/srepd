package ai

import (
	"context"
	"fmt"
)

type ramalamaProvider struct {
	inner *openaiCompatProvider
}

func newRamalamaProvider(cfg Config) (*ramalamaProvider, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:8080"
	}

	inner, err := newOpenAICompatProvider(cfg, "")
	if err != nil {
		return nil, fmt.Errorf("ramalama: %w", err)
	}

	return &ramalamaProvider{inner: inner}, nil
}

func (p *ramalamaProvider) Name() string {
	return "ramalama"
}

// SupportsStreaming reports that this provider streams tokens via StreamQuery.
func (p *ramalamaProvider) SupportsStreaming() bool { return true }

func (p *ramalamaProvider) Query(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return p.inner.Query(ctx, systemPrompt, userPrompt)
}

func (p *ramalamaProvider) StreamQuery(ctx context.Context, systemPrompt string, userPrompt string, ch chan<- string) error {
	return p.inner.StreamQuery(ctx, systemPrompt, userPrompt, ch)
}

func (p *ramalamaProvider) Healthy(ctx context.Context) error {
	return p.inner.Healthy(ctx)
}
