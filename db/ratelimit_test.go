// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/db"
)

type mockRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

type mockDBTX struct {
	execFunc     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	copyFromFunc func(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, sql, arguments...)
	}
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, sql, args...)
	}
	return &mockRow{}
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, sql, args...)
	}
	return nil, nil
}

func (m *mockDBTX) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	if m.copyFromFunc != nil {
		return m.copyFromFunc(ctx, tableName, columnNames, rowSrc)
	}
	return 0, nil
}

func TestNewPGRateLimiter_Validation(t *testing.T) {
	t.Run("nil DBTX returns error", func(t *testing.T) {
		l, err := db.NewPGRateLimiter(nil, 10, time.Minute)
		assert.Error(t, err)
		assert.Nil(t, l)
	})

	t.Run("invalid table name returns error", func(t *testing.T) {
		mock := &mockDBTX{}
		l, err := db.NewPGRateLimiter(mock, 10, time.Minute, db.WithPGRateLimiterTableName("table; DROP TABLE users;--"))
		assert.Error(t, err)
		assert.Nil(t, l)
	})

	t.Run("valid configuration succeeds and normalizes bounds", func(t *testing.T) {
		mock := &mockDBTX{}
		l, err := db.NewPGRateLimiter(mock, 0, 0,
			db.WithPGRateLimiterTableName("custom_ratelimit_windows"),
			db.WithPGRateLimiterTimeout(100*time.Millisecond),
		)
		require.NoError(t, err)
		assert.NotNil(t, l)
	})
}

func TestPGRateLimiter_Allow(t *testing.T) {
	mock := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*bool) = true
					return nil
				},
			}
		},
	}

	l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
	require.NoError(t, err)

	assert.False(t, l.Allow(""), "empty key should return false")
	assert.True(t, l.Allow("user-1"), "valid request should be allowed")
}

func TestPGRateLimiter_AllowN(t *testing.T) {
	t.Run("invalid inputs", func(t *testing.T) {
		l, err := db.NewPGRateLimiter(&mockDBTX{}, 10, time.Minute)
		require.NoError(t, err)

		assert.False(t, l.AllowN("", 1))
		assert.False(t, l.AllowN("key", 0))
		assert.False(t, l.AllowN("key", -1))
		assert.False(t, l.AllowN("key", 11), "n > limit should return false immediately")
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
		mock := &mockDBTX{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						*dest[0].(*bool) = false
						return nil
					},
				}
			},
		}

		l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
		require.NoError(t, err)
		assert.False(t, l.AllowN("user-heavy", 5))
	})

	t.Run("database error fails closed", func(t *testing.T) {
		mock := &mockDBTX{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						return errors.New("connection refused")
					},
				}
			},
		}

		l, err := db.NewPGRateLimiter(mock, 10, 500*time.Millisecond)
		require.NoError(t, err)
		assert.False(t, l.AllowN("user-fail", 1))
	})
}

func TestPGRateLimiter_Remaining(t *testing.T) {
	t.Run("empty key returns 0", func(t *testing.T) {
		l, err := db.NewPGRateLimiter(&mockDBTX{}, 10, time.Minute)
		require.NoError(t, err)
		assert.Equal(t, 0, l.Remaining(""))
	})

	t.Run("key not found returns full limit", func(t *testing.T) {
		mock := &mockDBTX{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						return pgx.ErrNoRows
					},
				}
			},
		}

		l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
		require.NoError(t, err)
		assert.Equal(t, 10, l.Remaining("new-user"))
	})

	t.Run("active window returns remaining count", func(t *testing.T) {
		now := time.Now()
		mock := &mockDBTX{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						*dest[0].(*int) = 4
						*dest[1].(*time.Time) = now.Add(-10 * time.Second)
						return nil
					},
				}
			},
		}

		l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
		require.NoError(t, err)
		assert.Equal(t, 6, l.Remaining("active-user"))
	})

	t.Run("expired window returns full limit", func(t *testing.T) {
		now := time.Now()
		mock := &mockDBTX{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						*dest[0].(*int) = 10
						*dest[1].(*time.Time) = now.Add(-2 * time.Minute) // expired
						return nil
					},
				}
			},
		}

		l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
		require.NoError(t, err)
		assert.Equal(t, 10, l.Remaining("expired-user"))
	})

	t.Run("database error returns 0", func(t *testing.T) {
		mock := &mockDBTX{
			queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{
					scanFunc: func(dest ...any) error {
						return errors.New("db down")
					},
				}
			},
		}

		l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
		require.NoError(t, err)
		assert.Equal(t, 0, l.Remaining("err-user"))
	})
}

func TestPGRateLimiter_Reset(t *testing.T) {
	executed := false
	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			executed = true
			assert.Equal(t, "user-to-reset", arguments[0])
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}

	l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
	require.NoError(t, err)

	l.Reset("")
	assert.False(t, executed)

	l.Reset("user-to-reset")
	assert.True(t, executed)
}

func TestPGRateLimiter_Remaining_ExceededCount(t *testing.T) {
	now := time.Now()
	mock := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*int) = 15 // count > limit (10)
					*dest[1].(*time.Time) = now.Add(-5 * time.Second)
					return nil
				},
			}
		},
	}

	l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, l.Remaining("over-limit-user"))
}

func TestPGRateLimiter_ContextSupport(t *testing.T) {
	var capturedCtx context.Context
	mock := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			capturedCtx = ctx
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*bool) = true
					return nil
				},
			}
		},
	}

	l, err := db.NewPGRateLimiter(mock, 10, time.Minute)
	require.NoError(t, err)

	// Normal context
	ctx := context.WithValue(context.Background(), "trace", "123")
	assert.True(t, l.AllowWithContext(ctx, "user-ctx"))
	assert.NotNil(t, capturedCtx)
	assert.Equal(t, "123", capturedCtx.Value("trace"))

	// Pre-cancelled context should fail closed
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	mockCancelled := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					return ctx.Err()
				},
			}
		},
	}
	lCancelled, err := db.NewPGRateLimiter(mockCancelled, 10, time.Minute)
	require.NoError(t, err)

	assert.False(t, lCancelled.AllowWithContext(cancelledCtx, "user-ctx"))
	assert.False(t, lCancelled.AllowNWithContext(cancelledCtx, "user-ctx", 2))
}

func TestPGRateLimiter_LimitOneWindowReset(t *testing.T) {
	// Test limit = 1 window reset behavior and query generation
	var executedSQL string
	var capturedArgs []any

	mock := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			executedSQL = sql
			capturedArgs = args
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*bool) = true
					return nil
				},
			}
		},
	}

	l, err := db.NewPGRateLimiter(mock, 1, time.Second)
	require.NoError(t, err)

	// First request: limit = 1 allowed
	assert.True(t, l.Allow("limit-1-user"))
	assert.Contains(t, executedSQL, "WHERE ($3 - ratelimit_windows.window_start >= $4::interval) OR (ratelimit_windows.current_count + $2 <= $5)")
	assert.Contains(t, executedSQL, "ratelimit_windows.window_start = $3")
	assert.Contains(t, executedSQL, "ratelimit_windows.current_count <= $5")
	require.Len(t, capturedArgs, 5)
	assert.Equal(t, "limit-1-user", capturedArgs[0])
	assert.Equal(t, 1, capturedArgs[1]) // n = 1
	assert.Equal(t, 1, capturedArgs[4]) // limit = 1

	// When limit exceeded and WHERE clause prevents row update, PostgreSQL returns 0 rows (ErrNoRows)
	mockNoRows := &mockDBTX{
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	}
	lNoRows, err := db.NewPGRateLimiter(mockNoRows, 1, time.Second)
	require.NoError(t, err)
	assert.False(t, lNoRows.Allow("limit-1-user"), "ErrNoRows due to WHERE clause must return false without failing open")
}
