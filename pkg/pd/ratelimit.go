package pd

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/charmbracelet/log"
	"golang.org/x/time/rate"
)

const (
	defaultRequestsPerSecond = 10
	defaultBurstSize         = 20
	defaultInitialDelay      = 1 * time.Second
	defaultMaxDelay          = 30 * time.Second
	defaultMaxRetries        = 3
)

// RateLimitOptions configures the rate limiter and retry behavior.
type RateLimitOptions struct {
	RequestsPerSecond float64
	BurstSize         int
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	MaxRetries        int
}

// RateLimitedClient wraps a PagerDutyClientInterface with rate limiting
// and retry with exponential backoff for 429 (rate limited) responses.
type RateLimitedClient struct {
	inner   PagerDutyClientInterface
	limiter *rate.Limiter
	opts    RateLimitOptions
}

// NewRateLimitedClient creates a new RateLimitedClient with default options:
// 10 requests/second, burst of 20, 1s initial delay, 30s max delay, 3 retries.
func NewRateLimitedClient(client PagerDutyClientInterface) *RateLimitedClient {
	return NewRateLimitedClientWithOptions(client, RateLimitOptions{
		RequestsPerSecond: defaultRequestsPerSecond,
		BurstSize:         defaultBurstSize,
		InitialDelay:      defaultInitialDelay,
		MaxDelay:          defaultMaxDelay,
		MaxRetries:        defaultMaxRetries,
	})
}

// NewRateLimitedClientWithOptions creates a new RateLimitedClient with the
// specified options for rate limiting and retry behavior.
func NewRateLimitedClientWithOptions(client PagerDutyClientInterface, opts RateLimitOptions) *RateLimitedClient {
	limiter := rate.NewLimiter(rate.Limit(opts.RequestsPerSecond), opts.BurstSize)
	return &RateLimitedClient{
		inner:   client,
		limiter: limiter,
		opts:    opts,
	}
}

// isRateLimitError returns true if the error indicates a PagerDuty 429 rate limit response.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") || strings.Contains(strings.ToLower(msg), "rate limit")
}

// withRetry executes fn with rate limiting and retries on 429 responses using
// exponential backoff. It respects the provided context for cancellation.
func (c *RateLimitedClient) withRetry(ctx context.Context, fn func() error) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter wait: %w", err)
	}

	err := fn()
	if err == nil {
		return nil
	}

	if !isRateLimitError(err) {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < c.opts.MaxRetries; attempt++ {
		delay := time.Duration(float64(c.opts.InitialDelay) * math.Pow(2, float64(attempt)))
		if delay > c.opts.MaxDelay {
			delay = c.opts.MaxDelay
		}

		log.Debug("pd.RateLimitedClient: retrying after 429",
			"attempt", attempt+1,
			"max_retries", c.opts.MaxRetries,
			"delay", delay,
		)

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
		case <-time.After(delay):
		}

		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter wait during retry: %w", err)
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isRateLimitError(lastErr) {
			return lastErr
		}
	}

	return lastErr
}

// CreateIncidentNoteWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) CreateIncidentNoteWithContext(ctx context.Context, id string, note pagerduty.IncidentNote) (*pagerduty.IncidentNote, error) {
	var result *pagerduty.IncidentNote
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.CreateIncidentNoteWithContext(ctx, id, note)
		return innerErr
	})
	return result, err
}

// GetCurrentUserWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) GetCurrentUserWithContext(ctx context.Context, opts pagerduty.GetCurrentUserOptions) (*pagerduty.User, error) {
	var result *pagerduty.User
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.GetCurrentUserWithContext(ctx, opts)
		return innerErr
	})
	return result, err
}

// GetEscalationPolicyWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) GetEscalationPolicyWithContext(ctx context.Context, id string, opts *pagerduty.GetEscalationPolicyOptions) (*pagerduty.EscalationPolicy, error) {
	var result *pagerduty.EscalationPolicy
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.GetEscalationPolicyWithContext(ctx, id, opts)
		return innerErr
	})
	return result, err
}

// GetIncidentWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) GetIncidentWithContext(ctx context.Context, id string) (*pagerduty.Incident, error) {
	var result *pagerduty.Incident
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.GetIncidentWithContext(ctx, id)
		return innerErr
	})
	return result, err
}

// GetTeamWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) GetTeamWithContext(ctx context.Context, id string) (*pagerduty.Team, error) {
	var result *pagerduty.Team
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.GetTeamWithContext(ctx, id)
		return innerErr
	})
	return result, err
}

// ListMembersWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) ListMembersWithContext(ctx context.Context, id string, opts pagerduty.ListTeamMembersOptions) (*pagerduty.ListTeamMembersResponse, error) {
	var result *pagerduty.ListTeamMembersResponse
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.ListMembersWithContext(ctx, id, opts)
		return innerErr
	})
	return result, err
}

// GetUserWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) GetUserWithContext(ctx context.Context, id string, opts pagerduty.GetUserOptions) (*pagerduty.User, error) {
	var result *pagerduty.User
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.GetUserWithContext(ctx, id, opts)
		return innerErr
	})
	return result, err
}

// ListIncidentAlertsWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) ListIncidentAlertsWithContext(ctx context.Context, id string, opts pagerduty.ListIncidentAlertsOptions) (*pagerduty.ListAlertsResponse, error) {
	var result *pagerduty.ListAlertsResponse
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.ListIncidentAlertsWithContext(ctx, id, opts)
		return innerErr
	})
	return result, err
}

// ListIncidentsWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) ListIncidentsWithContext(ctx context.Context, opts pagerduty.ListIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	var result *pagerduty.ListIncidentsResponse
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.ListIncidentsWithContext(ctx, opts)
		return innerErr
	})
	return result, err
}

// ListIncidentNotesWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) ListIncidentNotesWithContext(ctx context.Context, id string) ([]pagerduty.IncidentNote, error) {
	var result []pagerduty.IncidentNote
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.ListIncidentNotesWithContext(ctx, id)
		return innerErr
	})
	return result, err
}

// ListOnCallsWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) ListOnCallsWithContext(ctx context.Context, opts pagerduty.ListOnCallOptions) (*pagerduty.ListOnCallsResponse, error) {
	var result *pagerduty.ListOnCallsResponse
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.ListOnCallsWithContext(ctx, opts)
		return innerErr
	})
	return result, err
}

// ManageIncidentsWithContext wraps the inner client with rate limiting and retry.
func (c *RateLimitedClient) ManageIncidentsWithContext(ctx context.Context, email string, opts []pagerduty.ManageIncidentsOptions) (*pagerduty.ListIncidentsResponse, error) {
	var result *pagerduty.ListIncidentsResponse
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.ManageIncidentsWithContext(ctx, email, opts)
		return innerErr
	})
	return result, err
}

func (c *RateLimitedClient) MergeIncidentsWithContext(ctx context.Context, from string, id string, o []pagerduty.MergeIncidentsOptions) (*pagerduty.Incident, error) {
	var result *pagerduty.Incident
	err := c.withRetry(ctx, func() error {
		var innerErr error
		result, innerErr = c.inner.MergeIncidentsWithContext(ctx, from, id, o)
		return innerErr
	})
	return result, err
}
