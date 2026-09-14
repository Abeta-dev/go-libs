// SPDX-License-Identifier: MIT

package bodylimit_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umesh0492/go-libs/bodylimit"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func TestNew_AllowsBodyWithinLimit(t *testing.T) {
	mw := bodylimit.New(100)
	h := mw(http.HandlerFunc(echoHandler))

	payload := strings.Repeat("a", 50) // 50 bytes < 100 limit
	r := httptest.NewRequest("POST", "/", strings.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, payload, w.Body.String())
}

func TestNew_ExceedingLimitTruncatesBody(t *testing.T) {
	mw := bodylimit.New(10)
	// Handler that tries to read the full body
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	payload := bytes.Repeat([]byte("x"), 100) // 100 bytes > 10 limit
	r := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	// Handler should receive a read error triggered by MaxBytesReader
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestNew_NilBodyIsSkipped(t *testing.T) {
	mw := bodylimit.New(100)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest("GET", "/", nil)
	r.Body = nil
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMB_CorrectMultiplier(t *testing.T) {
	// MB(2) should allow a 1 MB body
	mw := bodylimit.MB(2)
	h := mw(http.HandlerFunc(echoHandler))
	payload := bytes.Repeat([]byte("x"), 1<<20) // 1 MB
	r := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestKB_CorrectMultiplier(t *testing.T) {
	// KB(4) = 4096 bytes
	mw := bodylimit.KB(4)
	h := mw(http.HandlerFunc(echoHandler))
	payload := bytes.Repeat([]byte("x"), 512) // 512 bytes < 4 KB
	r := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestString_FormatsCorrectly(t *testing.T) {
	cases := []struct {
		bytes    int64
		expected string
	}{
		{2 << 20, "2 MB"},
		{4 << 10, "4 KB"},
		{512, "512 B"},
	}
	for _, c := range cases {
		require.Equal(t, c.expected, bodylimit.String(c.bytes))
	}
}
