package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// nonStreamingProvider implements Provider but NOT StreamingProvider, and reports
// no streaming capability by omission.
type nonStreamingProvider struct{}

func (nonStreamingProvider) Query(context.Context, string, string) (string, error) {
	return "", nil
}
func (nonStreamingProvider) StreamQuery(context.Context, string, string, chan<- string) error {
	return nil
}
func (nonStreamingProvider) Name() string { return "nonstreaming" }

// optOutProvider implements StreamingProvider but returns false.
type optOutProvider struct{ nonStreamingProvider }

func (optOutProvider) SupportsStreaming() bool { return false }

func TestSupportsStreaming(t *testing.T) {
	t.Run("provider that does not implement StreamingProvider is not streamable", func(t *testing.T) {
		assert.False(t, SupportsStreaming(nonStreamingProvider{}))
	})

	t.Run("provider that implements StreamingProvider returning false is not streamable", func(t *testing.T) {
		assert.False(t, SupportsStreaming(optOutProvider{}))
	})

	t.Run("provider that implements StreamingProvider returning true is streamable", func(t *testing.T) {
		p := NewMockProvider("mock")
		p.Streaming = true
		assert.True(t, SupportsStreaming(p))
	})

	t.Run("nil provider is not streamable", func(t *testing.T) {
		assert.False(t, SupportsStreaming(nil))
	})
}

func TestSupportsHealthCheck(t *testing.T) {
	t.Run("provider without HealthChecker cannot be health-checked", func(t *testing.T) {
		assert.False(t, SupportsHealthCheck(nonStreamingProvider{}))
	})

	t.Run("provider with HealthChecker can be health-checked", func(t *testing.T) {
		assert.True(t, SupportsHealthCheck(NewMockProvider("mock")))
	})

	t.Run("nil provider cannot be health-checked", func(t *testing.T) {
		assert.False(t, SupportsHealthCheck(nil))
	})

	t.Run("anthropic-family providers have no probe endpoint", func(t *testing.T) {
		p, err := newAnthropicProvider(Config{}, "test-key")
		assert.NoError(t, err)
		assert.False(t, SupportsHealthCheck(p),
			"anthropic providers must not pretend to support health checks")
	})
}

func TestRealProviders_SupportStreaming(t *testing.T) {
	// All shipped providers implement real streaming; they must advertise it so the
	// TUI turns streaming on for them.
	openai, err := newOpenAICompatProvider(Config{Endpoint: "http://localhost", Model: "m"}, "")
	assert.NoError(t, err)
	assert.True(t, SupportsStreaming(openai))

	ollama, err := newOllamaProvider(Config{Endpoint: "http://localhost", Model: "m"})
	assert.NoError(t, err)
	assert.True(t, SupportsStreaming(ollama))

	rama, err := newRamalamaProvider(Config{Endpoint: "http://localhost", Model: "m"})
	assert.NoError(t, err)
	assert.True(t, SupportsStreaming(rama))

	anthro, err := newAnthropicProvider(Config{Model: "m"}, "fake-key")
	assert.NoError(t, err)
	assert.True(t, SupportsStreaming(anthro))
}
