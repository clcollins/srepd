package pd

import (
	"context"
	"errors"
	"net"

	"github.com/PagerDuty/go-pagerduty"
)

// tokenHelpPath tells the user where to create a PagerDuty User Token; it is
// surfaced whenever an auth failure is classified.
const tokenHelpPath = "PagerDuty web → My Profile → User Settings → API Access → Create New API User Token"

// ClassifyAPIError maps an error from a PagerDuty API call to a short,
// actionable message for the UI: auth failures point at token acquisition,
// rate limits say to wait, network failures say to check connectivity/VPN.
// Unrecognized errors pass through unchanged. Returns "" for nil.
func ClassifyAPIError(err error) string {
	if err == nil {
		return ""
	}

	var apiErr pagerduty.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 401:
			return "invalid or expired token — create one at " + tokenHelpPath
		case 403:
			return "token lacks permission — ensure it is a User Token created at " + tokenHelpPath
		case 429:
			return "rate limited by PagerDuty — wait a moment and try again"
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "network timeout — check connectivity and VPN"
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "network problem — check connectivity and VPN (" + err.Error() + ")"
	}

	return err.Error()
}
