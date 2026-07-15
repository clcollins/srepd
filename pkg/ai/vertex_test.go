package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveVertexRegion_ConfigTakesPrecedence(t *testing.T) {
	cfg := Config{Region: "us-central1"}
	assert.Equal(t, "us-central1", resolveVertexRegion(cfg))
}

func TestResolveVertexRegion_EmptyConfig_FallsToEnv(t *testing.T) {
	t.Setenv("CLOUD_ML_REGION", "europe-west1")
	cfg := Config{}
	assert.Equal(t, "europe-west1", resolveVertexRegion(cfg))
}

func TestResolveVertexRegion_SecondEnvVar(t *testing.T) {
	t.Setenv("CLOUD_ML_REGION", "")
	t.Setenv("VERTEXAI_LOCATION", "asia-east1")
	cfg := Config{}
	assert.Equal(t, "asia-east1", resolveVertexRegion(cfg))
}

func TestResolveVertexRegion_Empty(t *testing.T) {
	t.Setenv("CLOUD_ML_REGION", "")
	t.Setenv("VERTEXAI_LOCATION", "")
	cfg := Config{}
	assert.Equal(t, "", resolveVertexRegion(cfg))
}

func TestResolveVertexProjectID_ConfigTakesPrecedence(t *testing.T) {
	cfg := Config{ProjectID: "my-project"}
	assert.Equal(t, "my-project", resolveVertexProjectID(cfg))
}

func TestResolveVertexProjectID_FallsToEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "env-project")
	cfg := Config{}
	assert.Equal(t, "env-project", resolveVertexProjectID(cfg))
}

func TestResolveVertexProjectID_SecondEnvVar(t *testing.T) {
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
	t.Setenv("VERTEXAI_PROJECT", "vertex-project")
	cfg := Config{}
	assert.Equal(t, "vertex-project", resolveVertexProjectID(cfg))
}

func TestResolveVertexProjectID_ThirdEnvVar(t *testing.T) {
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
	t.Setenv("VERTEXAI_PROJECT", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "gcp-project")
	cfg := Config{}
	assert.Equal(t, "gcp-project", resolveVertexProjectID(cfg))
}

func TestResolveVertexProjectID_Empty(t *testing.T) {
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
	t.Setenv("VERTEXAI_PROJECT", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	cfg := Config{}
	assert.Equal(t, "", resolveVertexProjectID(cfg))
}

func TestNewVertexProvider_MissingRegion(t *testing.T) {
	t.Setenv("CLOUD_ML_REGION", "")
	t.Setenv("VERTEXAI_LOCATION", "")
	cfg := Config{Provider: "anthropic-vertex", ProjectID: "proj"}
	_, err := newVertexProvider(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires region")
}

func TestNewVertexProvider_MissingProjectID(t *testing.T) {
	t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "")
	t.Setenv("VERTEXAI_PROJECT", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	cfg := Config{Provider: "anthropic-vertex", Region: "us-central1"}
	_, err := newVertexProvider(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires project_id")
}

func TestNewVertexProvider_AuthPanicRecovery(t *testing.T) {
	cfg := Config{
		Provider:  "anthropic-vertex",
		Region:    "us-central1",
		ProjectID: "fake-project",
	}
	_, err := newVertexProvider(cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "auth failed")
	}
}
