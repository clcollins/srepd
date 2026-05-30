package ai

import (
	"context"
	"fmt"
	"os"
)

// Provider defines the interface for LLM API integrations.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Query sends a system prompt and user prompt to the LLM and returns
	// the complete response text. The context controls cancellation and
	// timeout.
	Query(ctx context.Context, systemPrompt string, userPrompt string) (string, error)

	// StreamQuery sends a system prompt and user prompt to the LLM and
	// streams response tokens to the provided channel. The channel is
	// closed when streaming completes. Errors are returned after the
	// channel is closed.
	StreamQuery(ctx context.Context, systemPrompt string, userPrompt string, ch chan<- string) error

	// Name returns the provider name (e.g., "anthropic", "openai").
	Name() string
}

// Config holds the configuration for an LLM API provider.
type Config struct {
	// Provider is the LLM provider name (e.g., "anthropic").
	Provider string

	// APIKeyEnv is the name of the environment variable containing the
	// API key. The key itself is never stored in config.
	APIKeyEnv string

	// Model is the model identifier (e.g., "claude-sonnet-4-6").
	Model string

	// Endpoint is an optional custom API endpoint URL. When empty, the
	// provider's default endpoint is used.
	Endpoint string
}

// knownProviders lists the provider names that NewProvider accepts.
var knownProviders = map[string]bool{
	"anthropic": true,
}

// NewProvider creates a Provider implementation based on the given Config.
// It validates the config and returns an error if the provider is unknown
// or the API key environment variable is not set.
func NewProvider(cfg Config) (Provider, error) {
	if cfg.Provider == "" {
		return nil, fmt.Errorf("ai: provider name is required")
	}

	if !knownProviders[cfg.Provider] {
		return nil, fmt.Errorf("ai: unknown provider %q", cfg.Provider)
	}

	if cfg.APIKeyEnv == "" {
		return nil, fmt.Errorf("ai: api_key_env is required")
	}

	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("ai: environment variable %q is not set or empty", cfg.APIKeyEnv)
	}

	switch cfg.Provider {
	case "anthropic":
		return newAnthropicProvider(cfg, apiKey)
	default:
		return nil, fmt.Errorf("ai: unknown provider %q", cfg.Provider)
	}
}

// ValidateConfig checks whether a Config is valid without creating a
// provider. It returns nil if the config is valid, or an error describing
// the first validation failure.
func ValidateConfig(cfg Config) error {
	if cfg.Provider == "" {
		return fmt.Errorf("ai: provider name is required")
	}

	if !knownProviders[cfg.Provider] {
		return fmt.Errorf("ai: unknown provider %q", cfg.Provider)
	}

	if cfg.APIKeyEnv == "" {
		return fmt.Errorf("ai: api_key_env is required")
	}

	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return fmt.Errorf("ai: environment variable %q is not set or empty", cfg.APIKeyEnv)
	}

	return nil
}
