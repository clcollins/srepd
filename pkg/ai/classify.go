package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// ClassifyProviderError maps an error from an LLM provider call to a short,
// human-readable message for the UI. API errors are reduced to their HTTP
// status plus the message embedded in the response body — never the raw JSON
// dump the SDK puts in Error(). Unrecognized errors pass through unchanged.
// Returns "" for nil.
func ClassifyProviderError(err error) string {
	if err == nil {
		return ""
	}

	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		status := fmt.Sprintf("%d %s", apiErr.StatusCode, http.StatusText(apiErr.StatusCode))

		var hint string
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			hint = "authentication failed — check your API key or cloud credentials"
		case http.StatusForbidden:
			hint = "permission denied"
		case http.StatusNotFound:
			hint = "not found — check the llm_api.model value"
		case http.StatusTooManyRequests:
			hint = "rate limited — wait a moment and try again"
		}

		msg := extractAPIErrorMessage(apiErr.RawJSON())
		switch {
		case hint != "" && msg != "":
			return fmt.Sprintf("%s (%s): %s", hint, status, msg)
		case msg != "":
			return fmt.Sprintf("%s: %s", status, msg)
		case hint != "":
			return fmt.Sprintf("%s (%s)", hint, status)
		default:
			return status
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

// extractAPIErrorMessage pulls the human-readable message out of an API error
// response body. It understands the Anthropic ({"error":{"message":...}}),
// Google Cloud ({"error":{...}} or [{"error":{...}}]), and AWS
// ({"message":...}) shapes. Returns "" when no message can be extracted.
func extractAPIErrorMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Google Cloud wraps some errors in a single-element array.
	if strings.HasPrefix(raw, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
			return ""
		}
		raw = string(items[0])
	}

	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return ""
	}
	if envelope.Error.Message != "" {
		return envelope.Error.Message
	}
	return envelope.Message
}
