// SPDX-License-Identifier: MIT

package async

import (
	"context"
	"log/slog"
	"runtime/debug"
)

// Go executes the given function in a new goroutine, recovering from any panics
// and logging the panic details with stack trace.
func Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("async goroutine panic recovered",
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn()
	}()
}

// GoCtx executes the given function in a new goroutine passing the provided context,
// recovering from any panics and logging the panic details with stack trace.
func GoCtx(ctx context.Context, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("async goroutine panic recovered",
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn(ctx)
	}()
}
