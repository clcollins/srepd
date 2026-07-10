package ai

import "context"

// MockProvider implements Provider and HealthChecker for testing.
type MockProvider struct {
	QueryResponse string
	QueryErr      error
	StreamTokens  []string
	StreamErr     error
	HealthyErr    error
	ProviderName  string
	Streaming     bool // reported by SupportsStreaming
	CallCounts    map[string]int
}

// SupportsStreaming reports the configured streaming capability.
func (m *MockProvider) SupportsStreaming() bool {
	return m.Streaming
}

// NewMockProvider creates a MockProvider with the given name.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		ProviderName: name,
		CallCounts:   make(map[string]int),
	}
}

func (m *MockProvider) recordCall(method string) {
	if m.CallCounts == nil {
		m.CallCounts = make(map[string]int)
	}
	m.CallCounts[method]++
}

func (m *MockProvider) Query(_ context.Context, _, _ string) (string, error) {
	m.recordCall("Query")
	return m.QueryResponse, m.QueryErr
}

func (m *MockProvider) StreamQuery(_ context.Context, _, _ string, ch chan<- string) error {
	m.recordCall("StreamQuery")
	defer close(ch)
	for _, token := range m.StreamTokens {
		ch <- token
	}
	return m.StreamErr
}

func (m *MockProvider) Name() string {
	return m.ProviderName
}

func (m *MockProvider) Healthy(_ context.Context) error {
	m.recordCall("Healthy")
	return m.HealthyErr
}
