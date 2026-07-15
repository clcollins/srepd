package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
)

// newTestAPIError builds an *anthropic.Error carrying a status code and raw
// response body — the two fields ClassifyProviderError consumes. Unmarshaling
// the body through the SDK's UnmarshalJSON populates RawJSON(), matching how
// the SDK constructs these errors from live responses.
func newTestAPIError(t *testing.T, status int, body string) *anthropic.Error {
	t.Helper()
	apiErr := &anthropic.Error{}
	if body != "" {
		assert.NoError(t, json.Unmarshal([]byte(body), apiErr))
	}
	apiErr.StatusCode = status
	return apiErr
}

const googleVertexErrorBody = `[{
  "error": {
    "code": 403,
    "message": "Agent Platform API has not been used in project my-gcp-project before or it is disabled. Enable it by visiting https://console.developers.google.com/apis/api/aiplatform.googleapis.com/overview?project=my-gcp-project then retry.",
    "status": "PERMISSION_DENIED",
    "details": [{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "SERVICE_DISABLED"}]
  }
}]`

const anthropicErrorBody = `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`

const awsErrorBody = `{"message":"The security token included in the request is invalid."}`

func TestExtractAPIErrorMessage_GoogleArrayShape(t *testing.T) {
	msg := extractAPIErrorMessage(googleVertexErrorBody)
	assert.Contains(t, msg, "Agent Platform API has not been used in project my-gcp-project")
	assert.NotContains(t, msg, "{", "extracted message must not contain raw JSON")
}

func TestExtractAPIErrorMessage_AnthropicShape(t *testing.T) {
	msg := extractAPIErrorMessage(anthropicErrorBody)
	assert.Equal(t, "invalid x-api-key", msg)
}

func TestExtractAPIErrorMessage_AWSShape(t *testing.T) {
	msg := extractAPIErrorMessage(awsErrorBody)
	assert.Equal(t, "The security token included in the request is invalid.", msg)
}

func TestExtractAPIErrorMessage_Empty(t *testing.T) {
	assert.Equal(t, "", extractAPIErrorMessage(""))
}

func TestExtractAPIErrorMessage_InvalidJSON(t *testing.T) {
	assert.Equal(t, "", extractAPIErrorMessage("<html>502 Bad Gateway</html>"))
}

func TestClassifyProviderError_Nil(t *testing.T) {
	assert.Equal(t, "", ClassifyProviderError(nil))
}

func TestClassifyProviderError_Timeout(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", context.DeadlineExceeded)
	msg := ClassifyProviderError(err)
	assert.Contains(t, msg, "timeout")
}

func TestClassifyProviderError_PassthroughUnknown(t *testing.T) {
	err := fmt.Errorf("some CLI error: exit status 1")
	assert.Equal(t, "some CLI error: exit status 1", ClassifyProviderError(err))
}

func TestClassifyProviderError_APIStatusMessage(t *testing.T) {
	// Simulates the SDK error string wrapping without a typed anthropic.Error:
	// falls back to passthrough. The typed path is covered by
	// TestClassifyProviderError_TypedAPIError below.
	err := fmt.Errorf("anthropic stream failed: POST \"https://aiplatform.googleapis.com/v1/messages\": 403 Forbidden %s", googleVertexErrorBody)
	msg := ClassifyProviderError(err)
	assert.NotEmpty(t, msg)
}

func TestClassifyProviderError_TypedAPIError(t *testing.T) {
	apiErr := newTestAPIError(t, 403, googleVertexErrorBody)
	err := fmt.Errorf("anthropic stream failed: %w", apiErr)

	msg := ClassifyProviderError(err)

	assert.Contains(t, msg, "403")
	assert.Contains(t, msg, "Agent Platform API has not been used in project my-gcp-project")
	assert.NotContains(t, msg, "aiplatform.googleapis.com/v1/messages", "URL noise must be stripped")
	assert.NotContains(t, msg, `"@type"`, "raw JSON details must be stripped")
}

func TestClassifyProviderError_TypedAPIError_NoBody(t *testing.T) {
	apiErr := newTestAPIError(t, 429, "")
	err := fmt.Errorf("anthropic query failed: %w", apiErr)

	msg := ClassifyProviderError(err)

	assert.Contains(t, msg, "429")
	assert.Contains(t, msg, "rate limited")
}
