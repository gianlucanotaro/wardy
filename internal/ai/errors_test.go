package ai

import (
	"errors"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"400 bad request", &anthropic.Error{StatusCode: 400}, false},
		{"401 unauthorized", &anthropic.Error{StatusCode: 401}, false},
		{"404 not found", &anthropic.Error{StatusCode: 404}, false},
		{"429 rate limited", &anthropic.Error{StatusCode: 429}, true},
		{"500 server error", &anthropic.Error{StatusCode: 500}, true},
		{"529 overloaded", &anthropic.Error{StatusCode: 529}, true},
		{"generic network error", errors.New("connection reset"), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsRetryable(c.err); got != c.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
