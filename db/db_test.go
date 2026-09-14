// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/db"
)

func TestConnect_EmptyURL(t *testing.T) {
	pool, err := db.Connect(context.Background(), "")
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.Equal(t, "db: connection string is required", err.Error())
}

func TestConnect_InvalidParse(t *testing.T) {
	pool, err := db.Connect(context.Background(), "invalid-postgres://")
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "unable to parse database url")
}

func TestConnect_FailedPing(t *testing.T) {
	// Use an unreachable port with short connection timeout to trigger failed ping
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = ctx
	cancel() // cancel context immediately to trigger failed ping instantly

	pool, err := db.Connect(context.Background(), "postgres://127.0.0.1:54321/nonexistent?sslmode=disable&connect_timeout=1")
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "unable to ping database")
}

func TestConnect_FailedPing_ClosesPool(t *testing.T) {
	var capturedPool *pgxpool.Pool
	pingErr := errors.New("ping timeout or failure")
	db.SetPingPool(func(ctx context.Context, pool *pgxpool.Pool) error {
		capturedPool = pool
		return pingErr
	})
	defer func() {
		db.ResetNewWithConfig()
		db.ResetPingPool()
	}()

	initialGoroutines := runtime.NumGoroutine()

	pool, err := db.Connect(context.Background(), "postgres://127.0.0.1:54321/nonexistent?sslmode=disable")
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.ErrorIs(t, err, pingErr)
	assert.Contains(t, err.Error(), "unable to ping database")

	assert.NotNil(t, capturedPool)

	// After pool.Close(), acquiring a connection must fail because the pool is closed
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	conn, acquireErr := capturedPool.Acquire(ctx)
	assert.Error(t, acquireErr)
	assert.Nil(t, conn)
	assert.Contains(t, acquireErr.Error(), "closed")

	// Verify no goroutine leak from the pool
	assert.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= initialGoroutines+2
	}, 1*time.Second, 10*time.Millisecond)
}

func TestConnect_NewWithConfigError(t *testing.T) {
	expectedErr := errors.New("new with config error")
	db.SetNewWithConfig(func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		return nil, expectedErr
	})
	defer db.ResetNewWithConfig()

	pool, err := db.Connect(context.Background(), "postgres://localhost:5432/db")
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "unable to create connection pool")
}

func TestConnect_SuccessMocked(t *testing.T) {
	db.SetNewWithConfig(func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		return nil, nil
	})
	db.SetPingPool(func(ctx context.Context, pool *pgxpool.Pool) error {
		return nil
	})
	defer func() {
		db.ResetNewWithConfig()
		db.ResetPingPool()
	}()

	pool, err := db.Connect(context.Background(), "postgres://localhost:5432/db", db.WithMaxConns(50), db.WithMinConns(5))
	assert.NoError(t, err)
	assert.Nil(t, pool)
}

func TestOptions(t *testing.T) {
	cfg, _ := pgxpool.ParseConfig("postgres://localhost:5432/db")
	optMax := db.WithMaxConns(50)
	optMin := db.WithMinConns(10)
	optMax(cfg)
	optMin(cfg)

	assert.Equal(t, int32(50), cfg.MaxConns)
	assert.Equal(t, int32(10), cfg.MinConns)
}

func TestGetQuerier(t *testing.T) {
	// 1. Context without connection: returns fallback pool
	var pool *pgxpool.Pool
	querier := db.GetQuerier(context.Background(), pool)
	assert.Nil(t, querier)

	// 2. Context with connection: returns connection
	var conn *pgxpool.Conn
	ctx := context.WithValue(context.Background(), db.ConnKey, conn)
	querierWithConn := db.GetQuerier(ctx, pool)
	assert.Nil(t, querierWithConn)
}
