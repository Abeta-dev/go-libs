// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/Abeta-dev/go-libs/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
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

type fakeTx struct {
	pgx.Tx
	committed  bool
	rolledBack bool
	commitErr  error
}

func (f *fakeTx) Commit(ctx context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	f.rolledBack = true
	return nil
}

func (f *fakeTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

type fakeBeginner struct {
	tx       *fakeTx
	beginErr error
}

func (b *fakeBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	return b.tx, nil
}

func TestWithTx(t *testing.T) {
	ctx := context.Background()

	// 1. Nil pool or nil fn
	assert.Error(t, db.WithTx(ctx, nil, func(ctx context.Context, tx pgx.Tx) error { return nil }))
	assert.Error(t, db.WithTx(ctx, &fakeBeginner{tx: &fakeTx{}}, nil))

	// 2. Begin error
	assert.Error(t, db.WithTx(ctx, &fakeBeginner{beginErr: errors.New("begin fail")}, func(ctx context.Context, tx pgx.Tx) error {
		return nil
	}))

	// 3. Successful commit & GetQuerier returns injected tx
	tx1 := &fakeTx{}
	err := db.WithTx(ctx, &fakeBeginner{tx: tx1}, func(txCtx context.Context, tx pgx.Tx) error {
		assert.Equal(t, tx1, db.GetQuerier(txCtx, nil))
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, tx1.committed)
	assert.False(t, tx1.rolledBack)

	// 4. Callback error triggers Rollback
	tx2 := &fakeTx{}
	err = db.WithTx(ctx, &fakeBeginner{tx: tx2}, func(txCtx context.Context, tx pgx.Tx) error {
		return errors.New("business error")
	})
	assert.Error(t, err)
	assert.False(t, tx2.committed)
	assert.True(t, tx2.rolledBack)

	// 5. Commit error
	tx3 := &fakeTx{commitErr: errors.New("commit fail")}
	err = db.WithTx(ctx, &fakeBeginner{tx: tx3}, func(txCtx context.Context, tx pgx.Tx) error {
		return nil
	})
	assert.Error(t, err)

	// 6. Panic inside callback rolls back and re-panics
	tx4 := &fakeTx{}
	assert.Panics(t, func() {
		_ = db.WithTx(ctx, &fakeBeginner{tx: tx4}, func(txCtx context.Context, tx pgx.Tx) error {
			panic("boom")
		})
	})
	assert.True(t, tx4.rolledBack)
}
