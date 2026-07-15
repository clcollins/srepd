package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/vertex"
	"github.com/charmbracelet/log"
)

func resolveVertexRegion(cfg Config) string {
	if cfg.Region != "" {
		return cfg.Region
	}
	for _, env := range []string{"CLOUD_ML_REGION", "VERTEXAI_LOCATION"} {
		if v := os.Getenv(env); v != "" {
			log.Debug("ai.vertex", "msg", "region from env", "env", env, "region", v)
			return v
		}
	}
	return ""
}

func resolveVertexProjectID(cfg Config) string {
	if cfg.ProjectID != "" {
		return cfg.ProjectID
	}
	for _, env := range []string{"ANTHROPIC_VERTEX_PROJECT_ID", "VERTEXAI_PROJECT", "GOOGLE_CLOUD_PROJECT"} {
		if v := os.Getenv(env); v != "" {
			log.Debug("ai.vertex", "msg", "project_id from env", "env", env, "project_id", v)
			return v
		}
	}
	return ""
}

func newVertexProvider(cfg Config) (p *anthropicProvider, err error) {
	region := resolveVertexRegion(cfg)
	if region == "" {
		return nil, fmt.Errorf("ai: anthropic-vertex requires region (set llm_api.region, CLOUD_ML_REGION, or VERTEXAI_LOCATION)")
	}

	projectID := resolveVertexProjectID(cfg)
	if projectID == "" {
		return nil, fmt.Errorf("ai: anthropic-vertex requires project_id (set llm_api.project_id, ANTHROPIC_VERTEX_PROJECT_ID, VERTEXAI_PROJECT, or GOOGLE_CLOUD_PROJECT)")
	}

	defer func() {
		if r := recover(); r != nil {
			p = nil
			err = fmt.Errorf("ai: anthropic-vertex auth failed: %v", r)
		}
	}()

	opts := []option.RequestOption{
		vertex.WithGoogleAuth(context.Background(), region, projectID),
	}

	client := anthropic.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = anthropicDefaultModel
	}

	log.Info("ai.vertex", "msg", "provider initialized", "region", region, "project_id", projectID, "model", model)

	return &anthropicProvider{
		client: client,
		model:  model,
		name:   "anthropic-vertex",
	}, nil
}
