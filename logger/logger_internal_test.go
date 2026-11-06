// SPDX-License-Identifier: MIT

package logger

import (
	"log/slog"
	"sync"
	"testing"
)

func TestDefault_AllLogLevels(t *testing.T) {
	levels := []string{"DEBUG", "WARN", "WARNING", "ERROR", "INFO", ""}
	for _, lvl := range levels {
		t.Setenv("LOG_LEVEL", lvl)
		defaultOnce = sync.Once{}
		defaultLogger = nil
		l := Default()
		if l == nil {
			t.Fatalf("expected non-nil logger for LOG_LEVEL=%q", lvl)
		}
	}
}

func TestWithSamplingKeyFunc_Option(t *testing.T) {
	var cfg samplingOptions
	fn := func(r slog.Record) string { return r.Message }
	WithSamplingKeyFunc(fn)(&cfg)
	if cfg.keyFunc == nil {
		t.Fatal("expected keyFunc to be set")
	}
}
