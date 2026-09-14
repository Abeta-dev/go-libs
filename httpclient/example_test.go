// SPDX-License-Identifier: MIT
package httpclient_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/umesh0492/go-libs/circuitbreaker"
	"github.com/umesh0492/go-libs/httpclient"
	"github.com/umesh0492/go-libs/ratelimit"
	"github.com/umesh0492/go-libs/retry"
)

func ExampleNew() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// 15-line production resilient client setup:
	client := httpclient.New(
		httpclient.WithRateLimiter(ratelimit.NewTokenBucket(100, 10*time.Millisecond)),
		httpclient.WithCircuitBreaker(circuitbreaker.NewConsecutiveBreaker(5, 30*time.Second)),
		httpclient.WithRetry(retry.Config{
			Attempts:    3,
			InitialWait: 10 * time.Millisecond,
			Strategy:    retry.ExponentialJitter,
		}),
		httpclient.WithPerAttemptTimeout(2*time.Second),
		httpclient.WithTotalTimeout(5*time.Second),
	)

	resp, err := client.Get(server.URL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("Status:", resp.StatusCode)
	// Output:
	// Status: 200
}
