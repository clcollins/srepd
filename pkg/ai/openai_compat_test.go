package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOpenAIQuery_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req openaiChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "test-model", req.Model)
		assert.False(t, req.Stream)
		assert.Len(t, req.Messages, 2)

		resp := openaiChatResponse{
			Choices: []openaiChoice{
				{Message: openaiChatMessage{Role: "assistant", Content: "Test response"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{
		Endpoint: server.URL,
		Model:    "test-model",
	}, "")
	assert.NoError(t, err)

	result, err := provider.Query(context.Background(), "system prompt", "user prompt")

	assert.NoError(t, err)
	assert.Equal(t, "Test response", result)
}

func TestOpenAIQuery_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		resp := openaiChatResponse{
			Choices: []openaiChoice{
				{Message: openaiChatMessage{Content: "OK"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{
		Endpoint: server.URL,
		Model:    "m",
	}, "test-api-key")
	assert.NoError(t, err)

	_, err = provider.Query(context.Background(), "", "test")
	assert.NoError(t, err)
}

func TestOpenAIQuery_WithoutAPIKey(t *testing.T) {
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

	provider, err := newOpenAICompatProvider(Config{
		Endpoint: server.URL,
		Model:    "m",
	}, "")
	assert.NoError(t, err)

	_, err = provider.Query(context.Background(), "", "test")
	assert.NoError(t, err)
}

func TestOpenAIQuery_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openaiChatResponse{Choices: []openaiChoice{}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "")
	assert.NoError(t, err)

	result, err := provider.Query(context.Background(), "", "test")

	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestOpenAIQuery_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "")
	assert.NoError(t, err)

	_, err = provider.Query(context.Background(), "", "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// TestOpenAIQuery_AppliesDefaultTimeout verifies that a non-streaming Query called
// with a context that has no deadline still gets bounded by the provider's default
// request timeout, so a hung server cannot block forever. The shared http.Client has
// no Timeout (that would truncate streams), so this defense lives in the request
// context.
func TestOpenAIQuery_AppliesDefaultTimeout(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-block // hang until the test releases it
	}))
	defer server.Close()
	defer close(block)

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "")
	assert.NoError(t, err)
	// Shorten the default so the test is fast.
	provider.requestTimeout = 100 * time.Millisecond

	start := time.Now()
	_, err = provider.Query(context.Background(), "", "test")
	elapsed := time.Since(start)

	assert.Error(t, err, "a hung server with no caller deadline should time out")
	assert.Less(t, elapsed, 5*time.Second, "should return promptly via the default timeout")
}

// TestOpenAIQuery_DoesNotLeakTokenInError guards against a proxy/gateway that echoes
// the Authorization header back in its error body: the returned error must contain
// the status code but never the API key.
func TestOpenAIQuery_DoesNotLeakTokenInError(t *testing.T) {
	const secret = "sk-super-secret-token-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a gateway that reflects the Authorization header in the body.
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("denied for " + r.Header.Get("Authorization")))
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, secret)
	assert.NoError(t, err)

	_, err = provider.Query(context.Background(), "", "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401", "error should carry the status code")
	assert.NotContains(t, err.Error(), secret, "error must not leak the API token")
}

func TestOpenAIStreamQuery_DoesNotLeakTokenInError(t *testing.T) {
	const secret = "sk-super-secret-token-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("denied for " + r.Header.Get("Authorization")))
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, secret)
	assert.NoError(t, err)

	ch := make(chan string, 4)
	err = provider.StreamQuery(context.Background(), "", "test", ch)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.NotContains(t, err.Error(), secret, "stream error must not leak the API token")
}

func TestOpenAIQuery_NoSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openaiChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)

		resp := openaiChatResponse{
			Choices: []openaiChoice{
				{Message: openaiChatMessage{Content: "OK"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "")
	assert.NoError(t, err)

	_, err = provider.Query(context.Background(), "", "test")
	assert.NoError(t, err)
}

func TestOpenAIStreamQuery_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openaiChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.True(t, req.Stream)

		flusher, ok := w.(http.Flusher)
		assert.True(t, ok)

		w.Header().Set("Content-Type", "text/event-stream")

		chunks := []openaiChatResponse{
			{Choices: []openaiChoice{{Delta: openaiChatMessage{Content: "Hello"}}}},
			{Choices: []openaiChoice{{Delta: openaiChatMessage{Content: " world"}}}},
		}

		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "")
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

func TestOpenAIStreamQuery_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "")
	assert.NoError(t, err)

	ch := make(chan string, 10)
	err = provider.StreamQuery(context.Background(), "", "test", ch)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestOpenAIHealthy_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "")
	assert.NoError(t, err)

	err = provider.Healthy(context.Background())
	assert.NoError(t, err)
}

func TestOpenAIHealthy_Unreachable(t *testing.T) {
	provider, err := newOpenAICompatProvider(Config{
		Endpoint: "http://localhost:1",
		Model:    "m",
	}, "")
	assert.NoError(t, err)

	err = provider.Healthy(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestOpenAIHealthy_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider, err := newOpenAICompatProvider(Config{Endpoint: server.URL, Model: "m"}, "test-key")
	assert.NoError(t, err)

	err = provider.Healthy(context.Background())
	assert.NoError(t, err)
}

func TestOpenAIName(t *testing.T) {
	provider, _ := newOpenAICompatProvider(Config{Endpoint: "http://localhost", Model: "m"}, "")
	assert.Equal(t, "openai", provider.Name())
}

func TestOpenAIProvider_RequiresEndpoint(t *testing.T) {
	_, err := newOpenAICompatProvider(Config{Model: "m"}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestOpenAIProvider_ImplementsHealthChecker(t *testing.T) {
	provider, _ := newOpenAICompatProvider(Config{Endpoint: "http://localhost", Model: "m"}, "")
	var _ HealthChecker = provider
}
