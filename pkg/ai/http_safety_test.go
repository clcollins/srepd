package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAICompatProvider_RejectsHTTPWithAPIKey(t *testing.T) {
	_, err := newOpenAICompatProvider(Config{
		Endpoint: "http://remote-server.example.com:8080",
		Model:    "gpt-4",
	}, "sk-secret-key")

	require.Error(t, err,
		"must reject non-localhost http:// endpoint when API key is set")
	assert.Contains(t, err.Error(), "HTTPS",
		"error message should mention HTTPS requirement")
}

func TestNewOpenAICompatProvider_AllowsHTTPLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{"localhost", "http://localhost:8080"},
		{"127.0.0.1", "http://127.0.0.1:11434"},
		{"[::1]", "http://[::1]:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := newOpenAICompatProvider(Config{
				Endpoint: tt.endpoint,
				Model:    "m",
			}, "sk-secret-key")

			assert.NoError(t, err,
				"localhost http:// with API key should be allowed")
			assert.NotNil(t, provider)
		})
	}
}

func TestNewOpenAICompatProvider_AllowsHTTPSWithAPIKey(t *testing.T) {
	provider, err := newOpenAICompatProvider(Config{
		Endpoint: "https://api.openai.com",
		Model:    "gpt-4",
	}, "sk-secret-key")

	assert.NoError(t, err,
		"https:// endpoint with API key should be allowed")
	assert.NotNil(t, provider)
}

func TestNewOpenAICompatProvider_AllowsHTTPWithoutAPIKey(t *testing.T) {
	provider, err := newOpenAICompatProvider(Config{
		Endpoint: "http://remote-server.example.com:8080",
		Model:    "llama",
	}, "")

	assert.NoError(t, err,
		"http:// without API key should be allowed (e.g., ollama)")
	assert.NotNil(t, provider)
}
