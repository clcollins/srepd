package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProvider_Anthropic(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test-key-123")

	provider, err := NewProvider(Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
		Model:     "claude-sonnet-4-6",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "anthropic", provider.Name())
}

func TestNewProvider_AnthropicWithEndpoint(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test-key-123")

	provider, err := NewProvider(Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
		Endpoint:  "https://custom.api.example.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_AnthropicDefaultModel(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test-key-123")

	provider, err := NewProvider(Config{
		Provider:  "anthropic",
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)

	ap, ok := provider.(*anthropicProvider)
	assert.True(t, ok)
	assert.Equal(t, anthropicDefaultModel, ap.model)
}

func TestNewProvider_AnthropicNoAPIKey(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "anthropic",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_Ollama(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "ollama",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "ollama", provider.Name())
}

func TestNewProvider_OllamaDefaults(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "ollama",
	})

	assert.NoError(t, err)
	op, ok := provider.(*ollamaProvider)
	assert.True(t, ok)
	assert.Equal(t, "http://localhost:11434", op.endpoint)
	assert.Equal(t, "llama3.1:8b", op.model)
}

func TestNewProvider_OllamaCustomEndpoint(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "ollama",
		Endpoint: "http://remote-host:11434",
		Model:    "mistral:7b",
	})

	assert.NoError(t, err)
	op, ok := provider.(*ollamaProvider)
	assert.True(t, ok)
	assert.Equal(t, "http://remote-host:11434", op.endpoint)
	assert.Equal(t, "mistral:7b", op.model)
}

func TestNewProvider_OpenAI(t *testing.T) {
	t.Setenv("TEST_OPENAI_KEY", "sk-test-123")

	provider, err := NewProvider(Config{
		Provider:  "openai",
		APIKeyEnv: "TEST_OPENAI_KEY",
		Endpoint:  "https://api.openai.com",
		Model:     "gpt-4o",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "openai", provider.Name())
}

func TestNewProvider_OpenAIRequiresEndpoint(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "openai",
	})

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestNewProvider_OpenAINoAPIKey(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "openai",
		Endpoint: "http://localhost:8080",
		Model:    "local-model",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestNewProvider_Ramalama(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "ramalama",
		Model:    "granite-code:8b",
	})

	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "ramalama", provider.Name())
}

func TestNewProvider_RamalamaDefaults(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "ramalama",
	})

	assert.NoError(t, err)
	rp, ok := provider.(*ramalamaProvider)
	assert.True(t, ok)
	assert.Equal(t, "http://localhost:8080", rp.inner.endpoint)
}

func TestNewProvider_Unknown(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "unknown-provider",
	})

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestNewProvider_EmptyProvider(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "",
	})

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "provider name is required")
}

func TestValidateConfig_VertexValid(t *testing.T) {
	err := ValidateConfig(Config{Provider: "anthropic-vertex"})
	assert.NoError(t, err)
}

func TestValidateConfig_BedrockValid(t *testing.T) {
	err := ValidateConfig(Config{Provider: "anthropic-bedrock"})
	assert.NoError(t, err)
}

func TestValidateConfig_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "anthropic with key",
			cfg:  Config{Provider: "anthropic", APIKeyEnv: "TEST_KEY"},
		},
		{
			name: "anthropic without key",
			cfg:  Config{Provider: "anthropic"},
		},
		{
			name: "anthropic-vertex",
			cfg:  Config{Provider: "anthropic-vertex"},
		},
		{
			name: "anthropic-bedrock",
			cfg:  Config{Provider: "anthropic-bedrock"},
		},
		{
			name: "ollama minimal",
			cfg:  Config{Provider: "ollama"},
		},
		{
			name: "openai with endpoint",
			cfg:  Config{Provider: "openai", Endpoint: "http://localhost:8080"},
		},
		{
			name: "ramalama minimal",
			cfg:  Config{Provider: "ramalama"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			assert.NoError(t, err)
		})
	}
}

func TestValidateConfig_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		errContains string
	}{
		{
			name:        "empty provider",
			cfg:         Config{Provider: ""},
			errContains: "provider name is required",
		},
		{
			name:        "unknown provider",
			cfg:         Config{Provider: "chatgpt"},
			errContains: "unknown provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.cfg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestProviderRegistry_Defaults(t *testing.T) {
	tests := []struct {
		name            string
		provider        string
		defaultEndpoint string
		defaultModel    string
	}{
		{
			name:            "anthropic defaults",
			provider:        "anthropic",
			defaultEndpoint: "",
			defaultModel:    "claude-sonnet-4-6",
		},
		{
			name:            "anthropic-vertex defaults",
			provider:        "anthropic-vertex",
			defaultEndpoint: "",
			defaultModel:    "claude-sonnet-4-6",
		},
		{
			name:            "anthropic-bedrock defaults",
			provider:        "anthropic-bedrock",
			defaultEndpoint: "",
			defaultModel:    "us.anthropic.claude-sonnet-4-6",
		},
		{
			name:            "ollama defaults",
			provider:        "ollama",
			defaultEndpoint: "http://localhost:11434",
			defaultModel:    "llama3.1:8b",
		},
		{
			name:            "openai defaults",
			provider:        "openai",
			defaultEndpoint: "",
			defaultModel:    "",
		},
		{
			name:            "ramalama defaults",
			provider:        "ramalama",
			defaultEndpoint: "http://localhost:8080",
			defaultModel:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := providerRegistry[tt.provider]
			assert.True(t, ok)
			assert.Equal(t, tt.defaultEndpoint, meta.defaultEndpoint)
			assert.Equal(t, tt.defaultModel, meta.defaultModel)
		})
	}
}
