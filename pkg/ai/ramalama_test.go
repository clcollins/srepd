package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRamalamaQuery_DelegatesCorrectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)

		resp := openaiChatResponse{
			Choices: []openaiChoice{
				{Message: openaiChatMessage{Content: "ramalama response"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newRamalamaProvider(Config{
		Endpoint: server.URL,
		Model:    "granite-code:8b",
	})
	assert.NoError(t, err)

	result, err := provider.Query(context.Background(), "system", "user")

	assert.NoError(t, err)
	assert.Equal(t, "ramalama response", result)
}

func TestRamalamaDefaults_Endpoint(t *testing.T) {
	provider, err := newRamalamaProvider(Config{
		Model: "test-model",
	})

	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", provider.inner.endpoint)
}

func TestRamalamaDefaults_CustomEndpoint(t *testing.T) {
	provider, err := newRamalamaProvider(Config{
		Endpoint: "http://custom:9090",
		Model:    "m",
	})

	assert.NoError(t, err)
	assert.Equal(t, "http://custom:9090", provider.inner.endpoint)
}

func TestRamalamaName(t *testing.T) {
	provider, _ := newRamalamaProvider(Config{Model: "m"})
	assert.Equal(t, "ramalama", provider.Name())
}

func TestRamalamaHealthy_Delegates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := newRamalamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	err = provider.Healthy(context.Background())
	assert.NoError(t, err)
}

func TestRamalamaProvider_ImplementsHealthChecker(t *testing.T) {
	provider, _ := newRamalamaProvider(Config{Model: "m"})
	var _ HealthChecker = provider
}

func TestRamalamaNoAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))

		resp := openaiChatResponse{
			Choices: []openaiChoice{
				{Message: openaiChatMessage{Content: "OK"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newRamalamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	_, err = provider.Query(context.Background(), "", "test")
	assert.NoError(t, err)
}
