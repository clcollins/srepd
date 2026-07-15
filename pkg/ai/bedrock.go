package ai

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/log"
)

const bedrockDefaultModel = "anthropic.claude-sonnet-4-6-20250514-v1:0"

func newBedrockProvider(cfg Config) (p *anthropicProvider, err error) {
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
