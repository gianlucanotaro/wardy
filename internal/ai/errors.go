package ai

import (
	"errors"

	"github.com/anthropics/anthropic-sdk-go"
)

// IsRetryable reports whether err might succeed on a later attempt. A 4xx
// from the API (bad request, auth, not-found) won't succeed on retry;
// rate limits, server errors, and anything else (network blips, timeouts)
// might. Per docs/ai.md: never retry on 4xx.
func IsRetryable(err error) bool {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500 || apiErr.StatusCode == 429
	}
	return true
}
