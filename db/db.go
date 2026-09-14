// SPDX-License-Identifier: MIT

// Package db manages PostgreSQL pgxpool connections and transaction contexts.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Option configures the internal pgxpool.Config connection threshold.
type Option func(*pgxpool.Config)

// WithMaxConns overrides the default maximum open connections allowed in the pool.
func WithMaxConns(maxConns int32) Option {
	return func(c *pgxpool.Config) {
		c.MaxConns = maxConns
	}
}

// WithMinConns sets the minimum background connections maintained in the pool.
func WithMinConns(minConns int32) Option {
	return func(c *pgxpool.Config) {
		c.MinConns = minConns
	}
}

var (
	newWithConfig = pgxpool.NewWithConfig
	pingPool      = func(ctx context.Context, pool *pgxpool.Pool) error {
		return pool.Ping(ctx)
	}
)

// Connect dials the database utilizing the injected Context and optional functional modifiers.
func Connect(ctx context.Context, databaseURL string, opts ...Option) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("db: connection string is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database url: %w", err)
	}

	for _, opt := range opts {
		opt(config)
	}

	pool, err := newWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pingPool(ctx, pool); err != nil {
		if pool != nil {
			pool.Close()
		}
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}
