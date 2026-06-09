package ai

import "context"

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
