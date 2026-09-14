// SPDX-License-Identifier: MIT

package shutdown_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/shutdown"
)

func TestManager_GracefulExecution(t *testing.T) {
	mgr := shutdown.New(2 * time.Second)

	var hook1, hook2 atomic.Bool

	mgr.Register("DB", func(ctx context.Context) error {
		hook1.Store(true)
		return nil
	})

	mgr.Register("HTTP", func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond) // simulate delay
		hook2.Store(true)
		return nil
	})

	go func() {
		time.Sleep(100 * time.Millisecond)
		p, err := os.FindProcess(os.Getpid())
		assert.NoError(t, err)
		p.Signal(syscall.SIGINT)
	}()

	err := mgr.Wait()
	assert.NoError(t, err)

	assert.True(t, hook1.Load())
	assert.True(t, hook2.Load())
}

func TestManager_Timeout(t *testing.T) {
	mgr := shutdown.New(100 * time.Millisecond)

	mgr.Register("Hanging", func(ctx context.Context) error {
		<-ctx.Done() // Block until ctx cancels
		return ctx.Err()
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGINT)
	}()

	err := mgr.Wait()
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestManager_HookErrorPropagates(t *testing.T) {
	mgr := shutdown.New(1 * time.Second)
	expectedErr1 := errors.New("hook failed 1")
	expectedErr2 := errors.New("hook failed 2")

	mgr.Register("Failing1", func(ctx context.Context) error {
		return expectedErr1
	})

	mgr.Register("Failing2", func(ctx context.Context) error {
		return expectedErr2
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGINT)
	}()

	err := mgr.Wait()
	assert.ErrorIs(t, err, expectedErr1)
	assert.ErrorIs(t, err, expectedErr2)
}

func TestManager_PanicCapturedAsError(t *testing.T) {
	mgr := shutdown.New(1 * time.Second)

	mgr.Register("PanickingHook", func(ctx context.Context) error {
		panic("database connection exploded")
	})

	err := mgr.Execute(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PanickingHook")
	assert.Contains(t, err.Error(), "database connection exploded")
}

func TestManager_HangingHookRespectsDeadline(t *testing.T) {
	mgr := shutdown.New(1 * time.Second)

	mgr.Register("StubbornHook", func(ctx context.Context) error {
		// Completely ignores context and hangs
		time.Sleep(5 * time.Second)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := mgr.Execute(ctx)
	duration := time.Since(start)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, duration, 500*time.Millisecond, "Execute must return when context deadline expires, not block for stubborn hook")
}

func TestManager_Adversarial_ConcurrentRegisterAndExecute(t *testing.T) {
	mgr := shutdown.New(2 * time.Second)
	var wg sync.WaitGroup

	// Concurrently register 100 hooks
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mgr.Register("DynamicHook", func(ctx context.Context) error {
				if idx%5 == 0 {
					return errors.New("hook err")
				}
				if idx%11 == 0 {
					panic("hook panic")
				}
				return nil
			})
		}(i)
	}

	// Concurrently execute
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = mgr.Execute(context.Background())
	}()

	wg.Wait()

	// Final execution after all registrations complete
	err := mgr.Execute(context.Background())
	assert.Error(t, err) // Should contain errors from the failing/panicking hooks without deadlock
}

func TestManager_Execute_ContextCancelledWithErrors(t *testing.T) {
	mgr := shutdown.New(1 * time.Second)
	expectedErr := errors.New("early hook failure")
	mgr.Register("FailingEarly", func(ctx context.Context) error {
		return expectedErr
	})
	mgr.Register("Hanging", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := mgr.Execute(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorIs(t, err, expectedErr)
}

func TestManager_Execute_EmptyHooks(t *testing.T) {
	mgr := shutdown.New(1 * time.Second)
	err := mgr.Execute(context.Background())
	assert.NoError(t, err)
}
