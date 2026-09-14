// SPDX-License-Identifier: MIT

package logger_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/umesh0492/go-libs/logger"
)

func ExampleFromContext() {
	ctx := context.Background()
	l := logger.FromContext(ctx)
	// FromContext always returns a non-nil *slog.Logger
	_ = l.Enabled(ctx, slog.LevelInfo)
	// Output:
}

func ExampleWithField() {
	ctx := logger.WithField(context.Background(), "user_id", "u-123")
	l := logger.FromContext(ctx)
	_ = l.Enabled(ctx, slog.LevelInfo)
	// Output:
}

func ExampleNewSamplingHandler() {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	sampler := logger.NewSamplingHandler(base, 1, time.Second)
	log := slog.New(sampler)
	log.Info("hello 1")
	log.Info("hello 2")
	fmt.Print(buf.String())
	// Output:
	// level=INFO msg="hello 1"
}

func ExampleNewRedactingHandler() {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	redactor := logger.NewRedactingHandler(base)
	log := slog.New(redactor)
	log.Info("user login", slog.String("password", "supersecret"))
	fmt.Print(buf.String())
	// Output:
	// level=INFO msg="user login" password=[REDACTED]
}
