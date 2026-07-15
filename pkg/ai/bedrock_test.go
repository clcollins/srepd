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
	assert.Equal(t, "anthropic.claude-sonnet-4-6-20250514-v1:0", bedrockDefaultModel)
}
