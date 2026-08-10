package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/log"
)

// bedrockDefaultModel is a US cross-region inference profile ID, not a bare
// foundation-model ID. Current Anthropic Claude models on Bedrock are
// inference-profile-only (no ON_DEMAND throughput), so the bare
// foundation-model ID cannot be invoked directly. See docs/llm-providers.md.
const bedrockDefaultModel = "us.anthropic.claude-sonnet-4-6"

func resolveBedrockRegion(cfg Config) string {
	if cfg.Region != "" {
		return cfg.Region
	}
	for _, env := range []string{"AWS_REGION", "AWS_DEFAULT_REGION"} {
		if v := os.Getenv(env); v != "" {
			log.Debug("ai.bedrock", "msg", "region from env", "env", env, "region", v)
			return v
		}
	}
	return ""
}

func newBedrockProvider(cfg Config) (p *anthropicProvider, err error) {
	region := resolveBedrockRegion(cfg)
	if region == "" {
		return nil, fmt.Errorf("ai: anthropic-bedrock requires region (set llm_api.region, AWS_REGION, or AWS_DEFAULT_REGION)")
	}

	defer func() {
		if r := recover(); r != nil {
			p = nil
			err = fmt.Errorf("ai: anthropic-bedrock auth failed: %v", r)
		}
	}()

	opts := []option.RequestOption{
		bedrock.WithLoadDefaultConfig(context.Background()),
	}

	client := anthropic.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = bedrockDefaultModel
	}

	log.Info("ai.bedrock", "msg", "provider initialized", "model", model)

	return &anthropicProvider{
		client: client,
		model:  model,
		name:   "anthropic-bedrock",
	}, nil
}
