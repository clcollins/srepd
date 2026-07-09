package ai

import (
	"context"
	"time"
)

// defaultRequestTimeout bounds non-streaming provider requests when the caller's
// context carries no deadline. Streaming requests are intentionally not bounded here
// (a whole-request timeout would truncate long token streams); they rely on the
// caller's context.
const defaultRequestTimeout = 60 * time.Second

// ensureTimeout returns ctx unchanged if it already has a deadline; otherwise it
// derives a context bounded by timeout. The returned cancel func must always be
// called by the caller.
func ensureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// Provider defines the interface for LLM API integrations.
// Implementations must be safe for concurrent use.
type Provider interface {
	Query(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
	StreamQuery(ctx context.Context, systemPrompt string, userPrompt string, ch chan<- string) error
	Name() string
}

// HealthChecker is an optional interface providers may implement
// to support connectivity checks.
type HealthChecker interface {
	Healthy(ctx context.Context) error
}

// Config holds the configuration for an LLM API provider.
type Config struct {
	Provider  string `mapstructure:"provider"`
	APIKeyEnv string `mapstructure:"api_key_env"`
	Model     string `mapstructure:"model"`
	Endpoint  string `mapstructure:"endpoint"`
}
