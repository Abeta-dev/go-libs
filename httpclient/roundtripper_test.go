// SPDX-License-Identifier: MIT

package httpclient_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/httpclient"
	"github.com/umesh0492/go-libs/retry"
)

func TestRoundTrip_DrainedBodyRewoundOnRetry(t *testing.T) {
	const expectedPayload = `{"action":"transfer","amount":1000}`
	var attemptCount int32
	var receivedBodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentAttempt := atomic.AddInt32(&attemptCount, 1)
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			receivedBodies = append(receivedBodies, string(bodyBytes))
		}

		if currentAttempt == 1 {
			// First attempt fails with 500 to trigger retry
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Second attempt succeeds
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	rt := httpclient.NewRoundTripper(
		httpclient.WithRetry(retry.Config{
			Attempts:    2,
			Strategy:    retry.Constant,
			InitialWait: 5 * time.Millisecond,
		}),
	)

	// Create request with body and explicitly ensure GetBody is nil (simulating streaming/non-rewindable body)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, io.NopCloser(strings.NewReader(expectedPayload)))
	require.NoError(t, err)
	req.GetBody = nil

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attemptCount))
	require.Len(t, receivedBodies, 2)
	assert.Equal(t, expectedPayload, receivedBodies[0], "attempt 1 must receive full body")
	assert.Equal(t, expectedPayload, receivedBodies[1], "attempt 2 must receive full rewound body")
}

type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read broken pipe")
}
func (errorReader) Close() error {
	return nil
}

func TestRoundTrip_BodyReadError(t *testing.T) {
	rt := httpclient.NewRoundTripper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost", errorReader{})
	require.NoError(t, err)
	req.GetBody = nil

	_, err = rt.RoundTrip(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read broken pipe")
}

func TestRoundTrip_BodyExceedingMaxRetrySizeNotRewound(t *testing.T) {
	// Generate payload exceeding DefaultMaxRetryBodySize (10MB + 1024 bytes)
	largePayloadSize := httpclient.DefaultMaxRetryBodySize + 1024
	largePayload := make([]byte, largePayloadSize)
	for i := range largePayload {
		largePayload[i] = 'A'
	}

	var attemptCount int32
	var receivedBytes int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		n, err := io.Copy(io.Discard, r.Body)
		if err == nil {
			atomic.StoreInt64(&receivedBytes, n)
		}
		// Server returns 500
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	rt := httpclient.NewRoundTripper(
		httpclient.WithRetry(retry.Config{
			Attempts:    3,
			Strategy:    retry.Constant,
			InitialWait: 5 * time.Millisecond,
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, io.NopCloser(bytes.NewReader(largePayload)))
	require.NoError(t, err)
	req.GetBody = nil

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Because body exceeded DefaultMaxRetryBodySize, only 1 attempt should have been made
	assert.Equal(t, int32(1), atomic.LoadInt32(&attemptCount))
	assert.Equal(t, int64(largePayloadSize), atomic.LoadInt64(&receivedBytes))
}
