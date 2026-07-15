package ai

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
)

type providerMeta struct {
	defaultEndpoint string
	defaultModel    string
}

var providerRegistry = map[string]providerMeta{
	"anthropic": {
		defaultEndpoint: "",
		defaultModel:    "claude-sonnet-4-6",
	},
	"anthropic-vertex": {
		defaultEndpoint: "",
		defaultModel:    "claude-sonnet-4-6",
	},
	"anthropic-bedrock": {
		defaultEndpoint: "",
		defaultModel:    "anthropic.claude-sonnet-4-6-20250514-v1:0",
	},
	"ollama": {
		defaultEndpoint: "http://localhost:11434",
		defaultModel:    "llama3.1:8b",
	},
	"openai": {
		defaultEndpoint: "",
		defaultModel:    "",
	},
	"ramalama": {
		defaultEndpoint: "http://localhost:8080",
		defaultModel:    "",
	},
}

// NewProvider creates a Provider implementation based on the given Config.
// It validates the config, applies provider-specific defaults, and resolves
// the optional API key from the environment.
func NewProvider(cfg Config) (Provider, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	meta := providerRegistry[cfg.Provider]
	if cfg.Endpoint == "" {
		cfg.Endpoint = meta.defaultEndpoint
		log.Debug("ai.NewProvider", "msg", "using default endpoint", "provider", cfg.Provider, "endpoint", cfg.Endpoint)
	}
	if cfg.Model == "" {
		cfg.Model = meta.defaultModel
		log.Debug("ai.NewProvider", "msg", "using default model", "provider", cfg.Provider, "model", cfg.Model)
	}

	var apiKey string
	if cfg.APIKeyEnv != "" {
		apiKey = os.Getenv(cfg.APIKeyEnv)
		if apiKey == "" {
			log.Warn("ai.NewProvider", "msg", "api_key_env set but environment variable is empty", "env_var", cfg.APIKeyEnv)
		} else {
			log.Debug("ai.NewProvider", "msg", "API key loaded from environment", "env_var", cfg.APIKeyEnv)
		}
	}

	log.Debug("ai.NewProvider", "provider", cfg.Provider, "endpoint", cfg.Endpoint, "model", cfg.Model)

	switch cfg.Provider {
	case "anthropic":
		return newAnthropicProvider(cfg, apiKey)
	case "anthropic-vertex":
		return newVertexProvider(cfg)
	case "anthropic-bedrock":
		return newBedrockProvider(cfg)
	case "ollama":
		return newOllamaProvider(cfg)
	case "openai":
		return newOpenAICompatProvider(cfg, apiKey)
	case "ramalama":
		return newRamalamaProvider(cfg)
	default:
		return nil, fmt.Errorf("ai: unknown provider %q", cfg.Provider)
	}
}

// ValidateConfig checks whether a Config is valid without creating a
// provider. Returns nil if valid, or an error describing the first
// validation failure.
func ValidateConfig(cfg Config) error {
	if cfg.Provider == "" {
		return fmt.Errorf("ai: provider name is required")
	}

	if _, ok := providerRegistry[cfg.Provider]; !ok {
		return fmt.Errorf("ai: unknown provider %q", cfg.Provider)
	}

	return nil
}
