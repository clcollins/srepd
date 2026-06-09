package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOllamaQuery_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "test-model", req.Model)
		assert.False(t, req.Stream)
		assert.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "user", req.Messages[1].Role)

		resp := ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: "Test response"},
			Done:    true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newOllamaProvider(Config{
		Endpoint: server.URL,
		Model:    "test-model",
	})
	assert.NoError(t, err)

	result, err := provider.Query(context.Background(), "system prompt", "user prompt")

	assert.NoError(t, err)
	assert.Equal(t, "Test response", result)
}

func TestOllamaQuery_NoSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)

		resp := ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: "OK"},
			Done:    true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newOllamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	result, err := provider.Query(context.Background(), "", "question")

	assert.NoError(t, err)
	assert.Equal(t, "OK", result)
}

func TestOllamaQuery_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	provider, err := newOllamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	_, err = provider.Query(context.Background(), "", "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOllamaStreamQuery_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.True(t, req.Stream)

		flusher, ok := w.(http.Flusher)
		assert.True(t, ok)

		chunks := []ollamaChatResponse{
			{Message: ollamaChatMessage{Content: "Hello"}, Done: false},
			{Message: ollamaChatMessage{Content: " world"}, Done: false},
			{Message: ollamaChatMessage{Content: ""}, Done: true},
		}

		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "%s\n", data)
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider, err := newOllamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	ch := make(chan string, 10)
	err = provider.StreamQuery(context.Background(), "sys", "user", ch)

	assert.NoError(t, err)

	var tokens []string
	for token := range ch {
		tokens = append(tokens, token)
	}
	assert.Equal(t, []string{"Hello", " world"}, tokens)
}

func TestOllamaStreamQuery_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider, err := newOllamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	ch := make(chan string, 10)
	err = provider.StreamQuery(context.Background(), "", "test", ch)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestOllamaHealthy_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	provider, err := newOllamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	err = provider.Healthy(context.Background())
	assert.NoError(t, err)
}

func TestOllamaHealthy_Unreachable(t *testing.T) {
	provider, err := newOllamaProvider(Config{
		Endpoint: "http://localhost:1",
		Model:    "m",
	})
	assert.NoError(t, err)

	err = provider.Healthy(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestOllamaHealthy_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider, err := newOllamaProvider(Config{Endpoint: server.URL, Model: "m"})
	assert.NoError(t, err)

	err = provider.Healthy(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOllamaName(t *testing.T) {
	provider, _ := newOllamaProvider(Config{Model: "m"})
	assert.Equal(t, "ollama", provider.Name())
}

func TestOllamaProvider_ImplementsHealthChecker(t *testing.T) {
	provider, _ := newOllamaProvider(Config{Model: "m"})
	var _ HealthChecker = provider
}
