// SPDX-License-Identifier: MIT

// Package shutdown provides graceful shutdown management for microservices and background workers.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Hook represents a registered callback to execute during shutdown.
type Hook struct {
	Name string
	Exec func(ctx context.Context) error
}

// Manager orchestrates a graceful degradation of bounded services simultaneously.
type Manager struct {
	hooks   []Hook
	mu      sync.Mutex
	timeout time.Duration
}

// New creates a generalized graceful shutdown manager bounded by a global timeout.
func New(timeout time.Duration) *Manager {
	return &Manager{timeout: timeout}
}

// Register appends a teardown hook to the shutdown queue natively.
func (m *Manager) Register(name string, exec func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, Hook{Name: name, Exec: exec})
}

// Execute executes all registered hooks concurrently within the provided context.
func (m *Manager) Execute(ctx context.Context) error {
	m.mu.Lock()
	hooks := append([]Hook(nil), m.hooks...) // thread-safe snapshot
	m.mu.Unlock()

	if len(hooks) == 0 {
		return ctx.Err()
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(hooks))

	for _, h := range hooks {
		wg.Add(1)
		hook := h
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errs <- fmt.Errorf("shutdown hook %q panicked: %v", hook.Name, r)
				}
			}()
			if err := hook.Exec(ctx); err != nil {
				errs <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errs)
		var finalErr error
		for err := range errs {
			if finalErr == nil {
				finalErr = err
			} else {
				finalErr = errors.Join(finalErr, err)
			}
		}
		return finalErr
	case <-ctx.Done():
		finalErr := ctx.Err()
		for {
			select {
			case err := <-errs:
				if err != nil {
					finalErr = errors.Join(finalErr, err)
				}
			default:
				return finalErr
			}
		}
	}
}

// Wait traps OS interrupt signals or SIGTERM, then calls Execute bounded by the manager's timeout.
func (m *Manager) Wait() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	return m.Execute(ctx)
}
