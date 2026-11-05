// SPDX-License-Identifier: MIT

package retry

import (
	"math"
	"testing"
	"time"
)

func TestComputeWait_OverflowAndBounds(t *testing.T) {
	// Linear cap at 1,000,000 attempts
	d := computeWait(Config{
		InitialWait: time.Millisecond,
		Strategy:    Linear,
	}, 1000005)
	if d != time.Millisecond*1000001 {
		t.Fatalf("expected capped linear wait, got %v", d)
	}

	// Linear int64 overflow wrapping to negative -> clamped to 0
	dOverflow := computeWait(Config{
		InitialWait: time.Duration(math.MaxInt64),
		Strategy:    Linear,
	}, 1)
	if dOverflow != 0 {
		t.Fatalf("expected negative overflow to clamp to 0, got %v", dOverflow)
	}

	// Exponential > 30 attempts & float64 >= MaxInt64 cap
	dExp := computeWait(Config{
		InitialWait: time.Duration(math.MaxInt64 / 2),
		Strategy:    Exponential,
	}, 35)
	if dExp != time.Duration(math.MaxInt64) {
		t.Fatalf("expected MaxInt64 cap on exponential overflow, got %v", dExp)
	}

	// ExponentialJitter > 30 attempts & ceiling >= MaxInt64 cap
	dJitter := computeWait(Config{
		InitialWait: time.Duration(math.MaxInt64 / 2),
		MaxWait:     time.Second,
		Strategy:    ExponentialJitter,
	}, 35)
	if dJitter < 0 || dJitter > time.Second {
		t.Fatalf("expected jitter bounded in [0, 1s], got %v", dJitter)
	}
}
