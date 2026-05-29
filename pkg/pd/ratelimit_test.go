package pd

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rateLimitMock is a configurable mock for testing rate limiting and retry behavior.
// It tracks call counts and can return errors for a specified number of calls.
type rateLimitMock struct {
	MockPagerDutyClient
	callCount     atomic.Int32
	failCount     int    // number of initial calls that should fail with 429
	failMessage   string // error message for failures
	returnAfterOK bool   // whether to return success after failCount failures
}

func newRateLimitMock(failCount int) *rateLimitMock {
	return &rateLimitMock{
		failCount:     failCount,
		failMessage:   "HTTP response code: 429",
		returnAfterOK: true,
	}
}

func (m *rateLimitMock) GetIncidentWithContext(ctx context.Context, id string) (*pagerduty.Incident, error) {
	count := int(m.callCount.Add(1))
	if count <= m.failCount {
		return nil, fmt.Errorf("%s", m.failMessage)
	}
	return &pagerduty.Incident{
		APIObject: pagerduty.APIObject{ID: id},
	}, nil
}

func (m *rateLimitMock) ListIncidentsWithContext(ctx context.Context, opts pagerduty.ListIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	count := int(m.callCount.Add(1))
	if count <= m.failCount {
		return nil, fmt.Errorf("%s", m.failMessage)
	}
	return &pagerduty.ListIncidentsResponse{
		Incidents: []pagerduty.Incident{
			{APIObject: pagerduty.APIObject{ID: "test-incident"}},
		},
	}, nil
}

func (m *rateLimitMock) ListIncidentAlertsWithContext(ctx context.Context, id string, opts pagerduty.ListIncidentAlertsOptions) (*pagerduty.ListAlertsResponse, error) {
	count := int(m.callCount.Add(1))
	if count <= m.failCount {
		return nil, fmt.Errorf("%s", m.failMessage)
	}
	return &pagerduty.ListAlertsResponse{
		Alerts: []pagerduty.IncidentAlert{
			{APIObject: pagerduty.APIObject{ID: "test-alert"}},
		},
	}, nil
}

func TestRateLimiter_PassthroughBelowLimit(t *testing.T) {
	t.Run("requests within rate limit pass through without delay", func(t *testing.T) {
		mock := newRateLimitMock(0) // no failures
		client := NewRateLimitedClient(mock)

		start := time.Now()

		// Make several requests that should all pass through quickly
		// since they are within the burst limit (default 20)
		for i := 0; i < 5; i++ {
			incident, err := client.GetIncidentWithContext(context.Background(), "test-id")
			require.NoError(t, err)
			assert.Equal(t, "test-id", incident.ID)
		}

		elapsed := time.Since(start)

		// 5 requests within burst of 20 should complete very quickly
		assert.Less(t, elapsed, 2*time.Second, "requests within burst limit should not be delayed significantly")
		assert.Equal(t, int32(5), mock.callCount.Load(), "all 5 requests should have been made")
	})

	t.Run("multiple method types pass through", func(t *testing.T) {
		mock := newRateLimitMock(0)
		client := NewRateLimitedClient(mock)

		// Test GetIncidentWithContext
		incident, err := client.GetIncidentWithContext(context.Background(), "inc-1")
		require.NoError(t, err)
		assert.Equal(t, "inc-1", incident.ID)

		// Test ListIncidentsWithContext
		resp, err := client.ListIncidentsWithContext(context.Background(), pagerduty.ListIncidentsOptions{})
		require.NoError(t, err)
		assert.Len(t, resp.Incidents, 1)

		// Test ListIncidentAlertsWithContext
		alerts, err := client.ListIncidentAlertsWithContext(context.Background(), "inc-1", pagerduty.ListIncidentAlertsOptions{})
		require.NoError(t, err)
		assert.Len(t, alerts.Alerts, 1)

		assert.Equal(t, int32(3), mock.callCount.Load(), "all 3 requests should have been made")
	})
}

func TestRetry_429Response(t *testing.T) {
	t.Run("retries on 429 response and succeeds", func(t *testing.T) {
		mock := newRateLimitMock(1) // first call fails with 429, second succeeds

		// Use short delays for testing
		client := NewRateLimitedClientWithOptions(mock, RateLimitOptions{
			RequestsPerSecond: 100,
			BurstSize:         100,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			MaxRetries:        3,
		})

		incident, err := client.GetIncidentWithContext(context.Background(), "test-id")
		require.NoError(t, err)
		assert.Equal(t, "test-id", incident.ID)
		assert.Equal(t, int32(2), mock.callCount.Load(), "should have made 2 calls: 1 failure + 1 success")
	})
}

func TestRetry_MaxRetriesExhausted(t *testing.T) {
	t.Run("returns error after max retries exhausted", func(t *testing.T) {
		mock := newRateLimitMock(10) // always fails (more than max retries)

		client := NewRateLimitedClientWithOptions(mock, RateLimitOptions{
			RequestsPerSecond: 100,
			BurstSize:         100,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			MaxRetries:        3,
		})

		_, err := client.GetIncidentWithContext(context.Background(), "test-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "429")
		// Initial call + 3 retries = 4 total attempts
		assert.Equal(t, int32(4), mock.callCount.Load(), "should have made 4 calls: 1 initial + 3 retries")
	})
}

func TestRetry_SuccessAfterRetry(t *testing.T) {
	t.Run("succeeds after transient 429 failures", func(t *testing.T) {
		mock := newRateLimitMock(2) // first 2 calls fail, third succeeds

		client := NewRateLimitedClientWithOptions(mock, RateLimitOptions{
			RequestsPerSecond: 100,
			BurstSize:         100,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			MaxRetries:        3,
		})

		incident, err := client.GetIncidentWithContext(context.Background(), "test-id")
		require.NoError(t, err)
		assert.Equal(t, "test-id", incident.ID)
		assert.Equal(t, int32(3), mock.callCount.Load(), "should have made 3 calls: 2 failures + 1 success")
	})

	t.Run("non-429 errors are not retried", func(t *testing.T) {
		mock := newRateLimitMock(1)
		mock.failMessage = "authentication failed" // not a 429 error

		client := NewRateLimitedClientWithOptions(mock, RateLimitOptions{
			RequestsPerSecond: 100,
			BurstSize:         100,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			MaxRetries:        3,
		})

		_, err := client.GetIncidentWithContext(context.Background(), "test-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
		assert.Equal(t, int32(1), mock.callCount.Load(), "should have made only 1 call since it's not a 429 error")
	})
}

func TestRetry_ContextCancellation(t *testing.T) {
	t.Run("respects context cancellation during retry", func(t *testing.T) {
		mock := newRateLimitMock(10) // always fails

		client := NewRateLimitedClientWithOptions(mock, RateLimitOptions{
			RequestsPerSecond: 100,
			BurstSize:         100,
			InitialDelay:      500 * time.Millisecond, // long delay to allow cancellation
			MaxDelay:          1 * time.Second,
			MaxRetries:        5,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := client.GetIncidentWithContext(ctx, "test-id")
		require.Error(t, err)
		// Should fail quickly due to context cancellation, not exhaust all retries
		assert.Less(t, mock.callCount.Load(), int32(5), "should not exhaust all retries due to context cancellation")
	})
}

// TestRateLimitedWrapper_Delegation tests that each rate-limited wrapper method
// correctly delegates to the inner mock client. Each test verifies:
// 1. The wrapper returns the expected result from the mock
// 2. The inner mock's call count increments (proving delegation happened)
func TestRateLimitedWrapper_CreateIncidentNoteWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	note := pagerduty.IncidentNote{Content: "test note"}
	result, err := client.CreateIncidentNoteWithContext(ctx, "INCIDENT1", note)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test note", result.Content)
	assert.Equal(t, 1, mock.CallCounts["CreateIncidentNoteWithContext"])
}

func TestRateLimitedWrapper_CreateIncidentNoteWithContext_Error(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	note := pagerduty.IncidentNote{Content: "test note"}
	_, err := client.CreateIncidentNoteWithContext(ctx, "err", note)

	assert.Error(t, err)
	assert.Equal(t, 1, mock.CallCounts["CreateIncidentNoteWithContext"])
}

func TestRateLimitedWrapper_GetCurrentUserWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	result, err := client.GetCurrentUserWithContext(ctx, pagerduty.GetCurrentUserOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "MOCK_USER", result.ID)
	assert.Equal(t, "mock@example.com", result.Email)
	assert.Equal(t, 1, mock.CallCounts["GetCurrentUserWithContext"])
}

func TestRateLimitedWrapper_GetEscalationPolicyWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	result, err := client.GetEscalationPolicyWithContext(ctx, "POLICY1", &pagerduty.GetEscalationPolicyOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "POLICY1", result.ID)
	assert.Equal(t, 1, mock.CallCounts["GetEscalationPolicyWithContext"])
}

func TestRateLimitedWrapper_GetEscalationPolicyWithContext_Error(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	_, err := client.GetEscalationPolicyWithContext(ctx, "err", &pagerduty.GetEscalationPolicyOptions{})

	assert.Error(t, err)
	assert.Equal(t, 1, mock.CallCounts["GetEscalationPolicyWithContext"])
}

func TestRateLimitedWrapper_GetTeamWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	result, err := client.GetTeamWithContext(ctx, "TEAM1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "TEAM1", result.Name)
	assert.Equal(t, 1, mock.CallCounts["GetTeamWithContext"])
}

func TestRateLimitedWrapper_ListMembersWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	result, err := client.ListMembersWithContext(ctx, "TEAM1", pagerduty.ListTeamMembersOptions{Limit: 100})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Members, 1)
	assert.Equal(t, 1, mock.CallCounts["ListMembersWithContext"])
}

func TestRateLimitedWrapper_GetUserWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	result, err := client.GetUserWithContext(ctx, "USER1", pagerduty.GetUserOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "USER1", result.ID)
	assert.Equal(t, 1, mock.CallCounts["GetUserWithContext"])
}

func TestRateLimitedWrapper_GetUserWithContext_Error(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	_, err := client.GetUserWithContext(ctx, "err", pagerduty.GetUserOptions{})

	assert.Error(t, err)
	assert.Equal(t, 1, mock.CallCounts["GetUserWithContext"])
}

func TestRateLimitedWrapper_ListIncidentNotesWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	result, err := client.ListIncidentNotesWithContext(ctx, "INCIDENT1")

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "QABCDEFG1234567", result[0].ID)
	assert.Equal(t, 1, mock.CallCounts["ListIncidentNotesWithContext"])
}

func TestRateLimitedWrapper_ListIncidentNotesWithContext_Error(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	_, err := client.ListIncidentNotesWithContext(ctx, "err")

	assert.Error(t, err)
	assert.Equal(t, 1, mock.CallCounts["ListIncidentNotesWithContext"])
}

func TestRateLimitedWrapper_ListOnCallsWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	result, err := client.ListOnCallsWithContext(ctx, pagerduty.ListOnCallOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.OnCalls) // Default mock returns empty on-calls
	assert.Equal(t, 1, mock.CallCounts["ListOnCallsWithContext"])
}

func TestRateLimitedWrapper_ManageIncidentsWithContext(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	opts := []pagerduty.ManageIncidentsOptions{
		{ID: "INCIDENT1", Status: "acknowledged"},
	}
	result, err := client.ManageIncidentsWithContext(ctx, "user@example.com", opts)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Incidents, 2)
	assert.Equal(t, 1, mock.CallCounts["ManageIncidentsWithContext"])
}

func TestRateLimitedWrapper_ManageIncidentsWithContext_Error(t *testing.T) {
	mock := &MockPagerDutyClient{}
	client := NewRateLimitedClient(mock)
	ctx := context.Background()

	opts := []pagerduty.ManageIncidentsOptions{
		{ID: "err"},
	}
	_, err := client.ManageIncidentsWithContext(ctx, "user@example.com", opts)

	assert.Error(t, err)
	assert.Equal(t, 1, mock.CallCounts["ManageIncidentsWithContext"])
}
