package ai

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockProvider_ImplementsProvider(t *testing.T) {
	mock := NewMockProvider("test")
	var _ Provider = mock
}

func TestMockProvider_ImplementsHealthChecker(t *testing.T) {
	mock := NewMockProvider("test")
	var _ HealthChecker = mock
}

func TestMockProvider_Query(t *testing.T) {
	mock := NewMockProvider("test")
	mock.QueryResponse = "mock response"

	result, err := mock.Query(context.Background(), "sys", "user")

	assert.NoError(t, err)
	assert.Equal(t, "mock response", result)
	assert.Equal(t, 1, mock.CallCounts["Query"])
}

func TestMockProvider_QueryError(t *testing.T) {
	mock := NewMockProvider("test")
	mock.QueryErr = fmt.Errorf("mock error")

	_, err := mock.Query(context.Background(), "sys", "user")

	assert.Error(t, err)
	assert.Equal(t, "mock error", err.Error())
}

func TestMockProvider_StreamQuery(t *testing.T) {
	mock := NewMockProvider("test")
	mock.StreamTokens = []string{"Hello", " ", "world"}

	ch := make(chan string, 10)
	err := mock.StreamQuery(context.Background(), "sys", "user", ch)

	assert.NoError(t, err)

	var tokens []string
	for token := range ch {
		tokens = append(tokens, token)
	}
	assert.Equal(t, []string{"Hello", " ", "world"}, tokens)
	assert.Equal(t, 1, mock.CallCounts["StreamQuery"])
}

func TestMockProvider_Name(t *testing.T) {
	mock := NewMockProvider("custom-name")
	assert.Equal(t, "custom-name", mock.Name())
}

func TestMockProvider_Healthy(t *testing.T) {
	mock := NewMockProvider("test")

	err := mock.Healthy(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, mock.CallCounts["Healthy"])
}

func TestMockProvider_HealthyError(t *testing.T) {
	mock := NewMockProvider("test")
	mock.HealthyErr = fmt.Errorf("connection refused")

	err := mock.Healthy(context.Background())

	assert.Error(t, err)
	assert.Equal(t, "connection refused", err.Error())
}

func TestMockProvider_CallCounts(t *testing.T) {
	mock := NewMockProvider("test")

	_, _ = mock.Query(context.Background(), "", "")
	_, _ = mock.Query(context.Background(), "", "")
	ch := make(chan string, 1)
	_ = mock.StreamQuery(context.Background(), "", "", ch)
	// drain channel
	for range ch {
	}
	_ = mock.Healthy(context.Background())

	assert.Equal(t, 2, mock.CallCounts["Query"])
	assert.Equal(t, 1, mock.CallCounts["StreamQuery"])
	assert.Equal(t, 1, mock.CallCounts["Healthy"])
}
