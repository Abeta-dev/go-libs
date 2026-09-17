// SPDX-License-Identifier: MIT

package idempotency_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/idempotency"
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
	return pgconn.NewCommandTag("INSERT 1"), nil
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

func TestNewPGStore_Validation(t *testing.T) {
	t.Run("nil DBTX returns error", func(t *testing.T) {
		store, err := idempotency.NewPGStore(nil)
		assert.Error(t, err)
		assert.Nil(t, store)
	})

	t.Run("invalid table name returns error", func(t *testing.T) {
		mock := &mockDBTX{}
		store, err := idempotency.NewPGStore(mock, idempotency.WithPGTableName("table; DROP TABLE users;--"))
		assert.Error(t, err)
		assert.Nil(t, store)
	})

	t.Run("valid configuration succeeds", func(t *testing.T) {
		mock := &mockDBTX{}
		store, err := idempotency.NewPGStore(
			mock,
			idempotency.WithPGTableName("custom_idempotency_keys"),
			idempotency.WithPGLockTTL(45*time.Second),
			idempotency.WithPGResponseTTL(12*time.Hour),
		)
		require.NoError(t, err)
		assert.NotNil(t, store)
	})
}

func TestPGStore_Lock_NewKey(t *testing.T) {
	ctx := context.Background()
	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			assert.Equal(t, "test-key-1", arguments[0])
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "test-key-1")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, record)
}

func TestPGStore_Lock_EmptyKey(t *testing.T) {
	ctx := context.Background()
	mock := &mockDBTX{}
	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "")
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Nil(t, record)
}

func TestPGStore_Lock_InsertError(t *testing.T) {
	ctx := context.Background()
	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db connection down")
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "key-error")
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Nil(t, record)
}

func TestPGStore_Lock_Conflict_CompletedRecord(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	headers := map[string][]string{"Content-Type": {"application/json"}}
	headersJSON, _ := json.Marshal(headers)
	statusCode := 201
	body := []byte(`{"status":"created"}`)

	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			// Insert conflict: 0 rows affected
			return pgconn.NewCommandTag("INSERT 0"), nil
		},
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*string) = string(idempotency.StatusCompleted)
					*dest[1].(**int) = &statusCode
					*dest[2].(*[]byte) = headersJSON
					*dest[3].(*[]byte) = body
					*dest[4].(*time.Time) = now.Add(time.Hour)
					*dest[5].(*time.Time) = now.Add(24 * time.Hour)
					return nil
				},
			}
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "existing-key")
	require.NoError(t, err)
	assert.True(t, exists)
	require.NotNil(t, record)
	assert.Equal(t, idempotency.StatusCompleted, record.Status)
	require.NotNil(t, record.Response)
	assert.Equal(t, 201, record.Response.StatusCode)
	assert.Equal(t, "application/json", record.Response.Headers["Content-Type"][0])
	assert.Equal(t, `{"status":"created"}`, string(record.Response.Body))
}

func TestPGStore_Lock_Conflict_InProgress(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0"), nil
		},
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*string) = string(idempotency.StatusInProgress)
					*dest[1].(**int) = nil
					*dest[2].(*[]byte) = nil
					*dest[3].(*[]byte) = nil
					*dest[4].(*time.Time) = now.Add(10 * time.Minute) // active lease
					*dest[5].(*time.Time) = now.Add(24 * time.Hour)
					return nil
				},
			}
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "in-progress-key")
	require.NoError(t, err)
	assert.True(t, exists)
	require.NotNil(t, record)
	assert.Equal(t, idempotency.StatusInProgress, record.Status)
	assert.Nil(t, record.Response)
}

func TestPGStore_Lock_Conflict_ExpiredLockReclaimed(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	expiredLease := now.Add(-5 * time.Minute)

	execCount := 0
	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount == 1 {
				// Initial INSERT conflict
				return pgconn.NewCommandTag("INSERT 0"), nil
			}
			// UPDATE reclaim lease succeeds
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*string) = string(idempotency.StatusInProgress)
					*dest[1].(**int) = nil
					*dest[2].(*[]byte) = nil
					*dest[3].(*[]byte) = nil
					*dest[4].(*time.Time) = expiredLease
					*dest[5].(*time.Time) = now.Add(24 * time.Hour)
					return nil
				},
			}
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "stale-lease-key")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, record)
	assert.Equal(t, 2, execCount)
}

func TestPGStore_Save_Unlock_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("Save succeeds", func(t *testing.T) {
		execCalled := false
		mock := &mockDBTX{
			execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
				execCalled = true
				assert.Equal(t, "key-save", arguments[0])
				assert.Equal(t, string(idempotency.StatusCompleted), arguments[1])
				assert.Equal(t, http.StatusOK, arguments[2])
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}

		store, err := idempotency.NewPGStore(mock)
		require.NoError(t, err)

		err = store.Save(ctx, "key-save", idempotency.Response{
			StatusCode: http.StatusOK,
			Headers:    map[string][]string{"X-Test": {"val"}},
			Body:       []byte("OK"),
		})
		require.NoError(t, err)
		assert.True(t, execCalled)
	})

	t.Run("Save empty key returns error", func(t *testing.T) {
		store, err := idempotency.NewPGStore(&mockDBTX{})
		require.NoError(t, err)
		err = store.Save(ctx, "", idempotency.Response{})
		assert.Error(t, err)
	})

	t.Run("Unlock succeeds", func(t *testing.T) {
		unlocked := false
		mock := &mockDBTX{
			execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
				unlocked = true
				assert.Equal(t, "key-unlock", arguments[0])
				assert.Equal(t, string(idempotency.StatusInProgress), arguments[1])
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		}

		store, err := idempotency.NewPGStore(mock)
		require.NoError(t, err)

		err = store.Unlock(ctx, "key-unlock")
		require.NoError(t, err)
		assert.True(t, unlocked)
	})

	t.Run("Unlock empty key returns error", func(t *testing.T) {
		store, err := idempotency.NewPGStore(&mockDBTX{})
		require.NoError(t, err)
		err = store.Unlock(ctx, "")
		assert.Error(t, err)
	})

	t.Run("Delete succeeds", func(t *testing.T) {
		deleted := false
		mock := &mockDBTX{
			execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
				deleted = true
				assert.Equal(t, "key-delete", arguments[0])
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		}

		store, err := idempotency.NewPGStore(mock)
		require.NoError(t, err)

		err = store.Delete(ctx, "key-delete")
		require.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("Delete empty key returns error", func(t *testing.T) {
		store, err := idempotency.NewPGStore(&mockDBTX{})
		require.NoError(t, err)
		err = store.Delete(ctx, "")
		assert.Error(t, err)
	})
}

func TestPGStore_Lock_ExpiredRecordOverwritten(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	expiredTime := now.Add(-10 * time.Minute)

	execCount := 0
	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			execCount++
			if execCount == 1 {
				// Initial INSERT conflict
				return pgconn.NewCommandTag("INSERT 0"), nil
			}
			// UPDATE overwrite expired record succeeds
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					*dest[0].(*string) = string(idempotency.StatusCompleted)
					*dest[1].(**int) = nil
					*dest[2].(*[]byte) = nil
					*dest[3].(*[]byte) = nil
					*dest[4].(*time.Time) = expiredTime
					*dest[5].(*time.Time) = expiredTime // expired
					return nil
				},
			}
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "expired-rec-key")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Nil(t, record)
	assert.Equal(t, 2, execCount)
}

func TestPGStore_Lock_ReadError(t *testing.T) {
	ctx := context.Background()
	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 0"), nil
		},
		queryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					return errors.New("read failed")
				},
			}
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	exists, record, err := store.Lock(ctx, "read-err-key")
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Nil(t, record)
}

func TestPGStore_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	mockErr := errors.New("query failed")
	mock := &mockDBTX{
		execFunc: func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, mockErr
		},
	}

	store, err := idempotency.NewPGStore(mock)
	require.NoError(t, err)

	err = store.Save(ctx, "k", idempotency.Response{StatusCode: 200})
	assert.Error(t, err)

	err = store.Unlock(ctx, "k")
	assert.Error(t, err)

	err = store.Delete(ctx, "k")
	assert.Error(t, err)
}
