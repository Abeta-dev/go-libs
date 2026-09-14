// SPDX-License-Identifier: MIT

package ginmw

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/umesh0492/go-libs/idempotency"
	"github.com/umesh0492/go-libs/logger"
)

// responseBuffer is a custom gin.ResponseWriter that captures the response body and status.
type responseBuffer struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write captures the body data while writing to the original writer.
func (w *responseBuffer) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteString captures the string data while writing to the original writer.
func (w *responseBuffer) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// Idempotency creates a middleware that ensures operations are idempotent
// based on the "Idempotency-Key" header.
func Idempotency(store idempotency.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		exists, record, err := store.Lock(ctx, key)
		if err != nil {
			logger.FromContext(ctx).Error("failed to acquire idempotency lock",
				slog.String("key", key),
				slog.Any("error", err),
			)
			// If the store fails, we should abort to be safe, preventing non-idempotent execution
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "idempotency store error"})
			return
		}

		if exists {
			if record.Status == idempotency.StatusInProgress {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": idempotency.ErrAlreadyInProgress.Error()})
				return
			}

			if record.Status == idempotency.StatusCompleted && record.Response != nil {
				replayIdempotentResponse(c, record.Response)
				return
			}
		}

		// Track completion to ensure in-progress locks are unlocked if the handler fails or panics
		var completed bool
		defer func() {
			if r := recover(); r != nil {
				unlockIdempotencyLock(ctx, store, key, "on panic")
				panic(r)
			}
			if !completed {
				unlockIdempotencyLock(ctx, store, key, "")
			}
		}()

		// It's a new request, intercept the response
		resBuffer := &responseBuffer{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = resBuffer

		c.Next()

		// If aborted before completion or returned a server error (5xx), do not cache response
		if c.IsAborted() || resBuffer.Status() >= 500 {
			return
		}

		res := idempotency.Response{
			StatusCode: resBuffer.Status(),
			Headers:    resBuffer.Header().Clone(),
			Body:       resBuffer.body.Bytes(),
		}

		// Save completed response
		if err := store.Save(ctx, key, res); err != nil {
			logger.FromContext(ctx).Error("failed to save idempotency response",
				slog.String("key", key),
				slog.Any("error", err),
			)
		} else {
			completed = true
		}
	}
}

func replayIdempotentResponse(c *gin.Context, res *idempotency.Response) {
	for k, values := range res.Headers {
		for _, v := range values {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Data(res.StatusCode, c.Writer.Header().Get("Content-Type"), res.Body)
	c.Abort()
}

func unlockIdempotencyLock(ctx context.Context, store idempotency.Store, key, suffix string) {
	if err := store.Unlock(ctx, key); err != nil {
		msg := "failed to release idempotency lock"
		if suffix != "" {
			msg += " " + suffix
		}
		logger.FromContext(ctx).Error(msg,
			slog.String("key", key),
			slog.Any("error", err),
		)
	}
}
