// SPDX-License-Identifier: MIT

package httpclient

import "errors"

var (
	// ErrRateLimited is returned when an outbound HTTP request is rejected by the rate limiter.
	ErrRateLimited = errors.New("httpclient: outbound rate limit exceeded")

	// ErrAttemptTimeout is returned when an individual request attempt exceeds the per-attempt timeout.
	ErrAttemptTimeout = errors.New("httpclient: per-attempt timeout exceeded")
)
