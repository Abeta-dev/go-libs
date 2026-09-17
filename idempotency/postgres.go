// SPDX-License-Identifier: MIT

package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/umesh0492/go-libs/db"
)

const (
	// DefaultTableName is the default PostgreSQL table name for idempotency records.
	DefaultTableName = "idempotency_keys"
	// DefaultLockTTL is the default lease duration for an in-progress operation.
	DefaultLockTTL = 30 * time.Second
	// DefaultResponseTTL is the default retention period for completed idempotent responses.
	DefaultResponseTTL = 24 * time.Hour
)

// IdempotencySchemaDDL defines the canonical PostgreSQL table definition for distributed idempotency.
const IdempotencySchemaDDL = `CREATE TABLE IF NOT EXISTS idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    status VARCHAR(32) NOT NULL,
    status_code INT,
    headers JSONB,
    body BYTEA,
    locked_until TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_idempotency_expires_at ON idempotency_keys (expires_at);
`

var tableNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// PGStore is a PostgreSQL-backed distributed implementation of Store.
// It leverages atomic SQL operations (INSERT ON CONFLICT, conditional UPDATE)
// ensuring distributed consistency across multi-replica services without race conditions.
type PGStore struct {
	db          db.DBTX
	tableName   string
	lockTTL     time.Duration
	responseTTL time.Duration
	nowFunc     func() time.Time
}

// PGOption configures PGStore.
type PGOption func(*PGStore)

// WithPGLockTTL configures the maximum duration an in-progress operation holds the lock before expiration.
func WithPGLockTTL(ttl time.Duration) PGOption {
	return func(s *PGStore) {
		if ttl > 0 {
			s.lockTTL = ttl
		}
	}
}

// WithPGResponseTTL configures the retention duration for completed responses.
func WithPGResponseTTL(ttl time.Duration) PGOption {
	return func(s *PGStore) {
		if ttl > 0 {
			s.responseTTL = ttl
		}
	}
}

// WithPGTableName configures a custom table name for idempotency keys.
func WithPGTableName(name string) PGOption {
	return func(s *PGStore) {
		s.tableName = name
	}
}

// NewPGStore creates a new PostgreSQL-backed Store.
func NewPGStore(dbtx db.DBTX, opts ...PGOption) (*PGStore, error) {
	if dbtx == nil {
		return nil, errors.New("idempotency: db connection is required")
	}

	s := &PGStore{
		db:          dbtx,
		tableName:   DefaultTableName,
		lockTTL:     DefaultLockTTL,
		responseTTL: DefaultResponseTTL,
		nowFunc:     time.Now,
	}

	for _, opt := range opts {
		opt(s)
	}

	if !tableNameRegex.MatchString(s.tableName) {
		return nil, fmt.Errorf("idempotency: invalid table name %q", s.tableName)
	}

	return s, nil
}

// Lock attempts to atomically start a new idempotent operation.
// If the key does not exist or has expired, it is initialized with StatusInProgress.
// If an active in-progress or completed operation exists, it returns exists = true and the existing Record.
func (s *PGStore) Lock(ctx context.Context, key string) (bool, *Record, error) {
	if key == "" {
		return false, nil, errors.New("idempotency: key cannot be empty")
	}

	now := s.nowFunc()
	lockedUntil := now.Add(s.lockTTL)
	expiresAt := now.Add(s.responseTTL)

	// Step 1: Attempt atomic insert if key does not exist
	insertQuery := fmt.Sprintf(
		"INSERT INTO %s (key, status, locked_until, expires_at, created_at, updated_at) "+
			"VALUES ($1, $2, $3, $4, $5, $5) ON CONFLICT (key) DO NOTHING",
		s.tableName,
	)

	tag, err := s.db.Exec(ctx, insertQuery, key, string(StatusInProgress), lockedUntil, expiresAt, now)
	if err != nil {
		return false, nil, fmt.Errorf("idempotency: lock insert failed: %w", err)
	}

	if tag.RowsAffected() == 1 {
		// Successfully acquired new lock
		return false, nil, nil
	}

	// Step 2: Conflict occurred. Fetch current row state
	selectQuery := fmt.Sprintf(
		"SELECT status, status_code, headers, body, locked_until, expires_at FROM %s WHERE key = $1",
		s.tableName,
	)

	var (
		statusStr      string
		statusCode     *int
		headersJSON    []byte
		body           []byte
		rowLockedUntil time.Time
		rowExpiresAt   time.Time
	)

	err = s.db.QueryRow(ctx, selectQuery, key).Scan(
		&statusStr,
		&statusCode,
		&headersJSON,
		&body,
		&rowLockedUntil,
		&rowExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Concurrent delete occurred, retry insert recursively
			return s.Lock(ctx, key)
		}
		return false, nil, fmt.Errorf("idempotency: lock read failed: %w", err)
	}

	// Check if entire record or lease has expired
	if s.tryReclaimExpired(ctx, key, statusStr, now, rowLockedUntil, rowExpiresAt, lockedUntil, expiresAt) {
		return false, nil, nil
	}

	return true, buildRecordFromRow(statusStr, statusCode, headersJSON, body), nil
}

func (s *PGStore) tryReclaimExpired(ctx context.Context, key, statusStr string, now, rowLockedUntil, rowExpiresAt, lockedUntil, expiresAt time.Time) bool {
	if now.After(rowExpiresAt) {
		updateQuery := fmt.Sprintf(
			"UPDATE %s SET status = $2, status_code = NULL, headers = NULL, body = NULL, "+
				"locked_until = $3, expires_at = $4, updated_at = $5 WHERE key = $1 AND expires_at = $6",
			s.tableName,
		)
		uTag, uErr := s.db.Exec(ctx, updateQuery, key, string(StatusInProgress), lockedUntil, expiresAt, now, rowExpiresAt)
		if uErr == nil && uTag.RowsAffected() == 1 {
			return true
		}
	}

	if Status(statusStr) == StatusInProgress && now.After(rowLockedUntil) {
		updateQuery := fmt.Sprintf(
			"UPDATE %s SET locked_until = $2, expires_at = $3, updated_at = $4 "+
				"WHERE key = $1 AND status = $5 AND locked_until = $6",
			s.tableName,
		)
		uTag, uErr := s.db.Exec(ctx, updateQuery, key, lockedUntil, expiresAt, now, string(StatusInProgress), rowLockedUntil)
		if uErr == nil && uTag.RowsAffected() == 1 {
			return true
		}
	}

	return false
}

func buildRecordFromRow(statusStr string, statusCode *int, headersJSON, body []byte) *Record {
	record := &Record{
		Status: Status(statusStr),
	}

	if record.Status == StatusCompleted && statusCode != nil {
		var headers map[string][]string
		if len(headersJSON) > 0 {
			if jsonErr := json.Unmarshal(headersJSON, &headers); jsonErr != nil {
				headers = make(map[string][]string)
			}
		}
		record.Response = &Response{
			StatusCode: *statusCode,
			Headers:    headers,
			Body:       body,
		}
	}

	return record
}

// Save atomically marks the operation as StatusCompleted and stores the response payload.
func (s *PGStore) Save(ctx context.Context, key string, response Response) error {
	if key == "" {
		return errors.New("idempotency: key cannot be empty")
	}

	headersJSON, err := json.Marshal(response.Headers)
	if err != nil {
		return fmt.Errorf("idempotency: failed to marshal headers: %w", err)
	}

	now := s.nowFunc()
	expiresAt := now.Add(s.responseTTL)

	query := fmt.Sprintf(
		"UPDATE %s SET status = $2, status_code = $3, headers = $4, body = $5, expires_at = $6, updated_at = $7 "+
			"WHERE key = $1",
		s.tableName,
	)

	_, err = s.db.Exec(ctx, query, key, string(StatusCompleted), response.StatusCode, headersJSON, response.Body, expiresAt, now)
	if err != nil {
		return fmt.Errorf("idempotency: save response failed: %w", err)
	}

	return nil
}

// Unlock releases an in-progress lock without saving a response (e.g. on operation failure or cancellation).
// If the record is already completed, Unlock is a no-op.
func (s *PGStore) Unlock(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("idempotency: key cannot be empty")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE key = $1 AND status = $2", s.tableName)
	_, err := s.db.Exec(ctx, query, key, string(StatusInProgress))
	if err != nil {
		return fmt.Errorf("idempotency: unlock failed: %w", err)
	}

	return nil
}

// Delete permanently removes an idempotency key regardless of status.
func (s *PGStore) Delete(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("idempotency: key cannot be empty")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE key = $1", s.tableName)
	_, err := s.db.Exec(ctx, query, key)
	if err != nil {
		return fmt.Errorf("idempotency: delete failed: %w", err)
	}

	return nil
}
