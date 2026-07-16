package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBedrockProvider_AuthPanicRecovery(t *testing.T) {
	cfg := Config{
		Provider: "anthropic-bedrock",
	}
	_, err := newBedrockProvider(cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "auth failed")
	}
}

func TestNewBedrockProvider_DefaultModel(t *testing.T) {
	assert.Equal(t, "us.anthropic.claude-sonnet-4-6", bedrockDefaultModel)
}
