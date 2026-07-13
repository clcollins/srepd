package pd

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/stretchr/testify/assert"
)

type fakeNetError struct{}

func (fakeNetError) Error() string   { return "dial tcp: connection refused" }
func (fakeNetError) Timeout() bool   { return false }
func (fakeNetError) Temporary() bool { return true }

var _ net.Error = fakeNetError{}

// OB-3: wizard/API errors must be classified into actionable messages, not
// dumped raw. A 401 is a token problem, a timeout is a VPN problem — the
// user should not have to reverse-engineer which.
func TestClassifyAPIError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "",
		},
		{
			name:     "401 unauthorized is a token problem",
			err:      pagerduty.APIError{StatusCode: 401},
			contains: "invalid or expired token",
		},
		{
			name:     "403 forbidden is a permissions problem",
			err:      pagerduty.APIError{StatusCode: 403},
			contains: "permission",
		},
		{
			name:     "429 is rate limiting",
			err:      pagerduty.APIError{StatusCode: 429},
			contains: "rate limited",
		},
		{
			name:     "wrapped API error still classifies",
			err:      fmt.Errorf("fetching teams: %w", pagerduty.APIError{StatusCode: 401}),
			contains: "invalid or expired token",
		},
		{
			name:     "context deadline is a network problem",
			err:      context.DeadlineExceeded,
			contains: "network",
		},
		{
			name:     "wrapped deadline classifies",
			err:      fmt.Errorf("call failed: %w", context.DeadlineExceeded),
			contains: "network",
		},
		{
			name:     "net.Error is a network problem",
			err:      fakeNetError{},
			contains: "network",
		},
		{
			name:     "unknown errors pass through",
			err:      fmt.Errorf("something odd happened"),
			contains: "something odd happened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ClassifyAPIError(tt.err)
			if tt.contains == "" {
				assert.Empty(t, msg)
				return
			}
			assert.Contains(t, msg, tt.contains)
		})
	}
}

// The 401 message must tell the user where to get a token — that help
// currently only exists deep in the wizard.
func TestClassifyAPIError_TokenHelpPath(t *testing.T) {
	msg := ClassifyAPIError(pagerduty.APIError{StatusCode: 401})
	assert.Contains(t, msg, "User Settings")
}
