// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/umesh0492/go-libs/ratelimit"
)

// DefaultRateLimitTableName is the default table name for distributed rate limiting.
const DefaultRateLimitTableName = "ratelimit_windows"

// RateLimitSchemaDDL defines the canonical PostgreSQL table definition for distributed rate limiting.
const RateLimitSchemaDDL = `CREATE TABLE IF NOT EXISTS ratelimit_windows (
    key VARCHAR(255) PRIMARY KEY,
    current_count INT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ratelimit_window_start ON ratelimit_windows (window_start);
`

var rateLimitTableNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// PGRateLimiter implements ratelimit.Limiter backed by a PostgreSQL database table.
// It synchronizes rate limits across multiple distributed replicas with atomic SQL execution,
// preventing the "N replicas = N times rate limit" multi-pod drift.
type PGRateLimiter struct {
	db        DBTX
	tableName string
	limit     int
	window    time.Duration
	timeout   time.Duration
	nowFunc   func() time.Time
}

// Compile-time check that PGRateLimiter implements ratelimit.Limiter.
var _ ratelimit.Limiter = (*PGRateLimiter)(nil)

// PGRateLimiterOption configures a PGRateLimiter.
type PGRateLimiterOption func(*PGRateLimiter)

// WithPGRateLimiterTableName sets a custom table name for rate limiting windows.
func WithPGRateLimiterTableName(name string) PGRateLimiterOption {
	return func(l *PGRateLimiter) {
		l.tableName = name
	}
}

// WithPGRateLimiterTimeout sets the database query timeout (default: 500ms).
func WithPGRateLimiterTimeout(d time.Duration) PGRateLimiterOption {
	return func(l *PGRateLimiter) {
		if d > 0 {
			l.timeout = d
		}
	}
}

// NewPGRateLimiter creates a new distributed rate limiter using PostgreSQL.
func NewPGRateLimiter(dbtx DBTX, limit int, window time.Duration, opts ...PGRateLimiterOption) (*PGRateLimiter, error) {
	if dbtx == nil {
		return nil, errors.New("ratelimit: db connection is required")
	}
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}

	l := &PGRateLimiter{
		db:        dbtx,
		tableName: DefaultRateLimitTableName,
		limit:     limit,
		window:    window,
		timeout:   500 * time.Millisecond,
		nowFunc:   time.Now,
	}

	for _, opt := range opts {
		opt(l)
	}

	if !rateLimitTableNameRegex.MatchString(l.tableName) {
		return nil, fmt.Errorf("ratelimit: invalid table name %q", l.tableName)
	}

	return l, nil
}

// Allow reports whether a single event may occur under key.
func (l *PGRateLimiter) Allow(key string) bool {
	return l.AllowN(key, 1)
}

// AllowN reports whether n events may occur under key.
func (l *PGRateLimiter) AllowN(key string, n int) bool {
	if key == "" || n <= 0 {
		return false
	}
	if n > l.limit {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()

	now := l.nowFunc()
	windowSeconds := fmt.Sprintf("%d seconds", int(l.window.Seconds()))
	if l.window.Seconds() < 1 {
		windowSeconds = fmt.Sprintf("%d milliseconds", l.window.Milliseconds())
	}

	queryTmpl := "INSERT INTO {{TABLE}} (key, current_count, window_start) " +
		"VALUES ($1, $2, $3) " +
		"ON CONFLICT (key) DO UPDATE SET " +
		"  current_count = CASE " +
		"    WHEN $3 - {{TABLE}}.window_start >= $4::interval THEN $2 " +
		"    WHEN {{TABLE}}.current_count + $2 <= $5 THEN {{TABLE}}.current_count + $2 " +
		"    ELSE {{TABLE}}.current_count " +
		"  END, " +
		"  window_start = CASE " +
		"    WHEN $3 - {{TABLE}}.window_start >= $4::interval THEN $3 " +
		"    ELSE {{TABLE}}.window_start " +
		"  END " +
		"RETURNING ( " +
		"  ($3 - {{TABLE}}.window_start >= $4::interval) OR " +
		"  ({{TABLE}}.current_count + $2 <= $5) " +
		");"

	query := strings.ReplaceAll(queryTmpl, "{{TABLE}}", l.tableName)

	var allowed bool
	err := l.db.QueryRow(ctx, query, key, n, now, windowSeconds, l.limit).Scan(&allowed)
	if err != nil {
		// Fail-closed on database error to protect downstream infrastructure
		return false
	}

	return allowed
}

// Remaining returns the estimated number of remaining permitted requests in the current window.
func (l *PGRateLimiter) Remaining(key string) int {
	if key == "" {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()

	now := l.nowFunc()
	query := fmt.Sprintf("SELECT current_count, window_start FROM %s WHERE key = $1", l.tableName)

	var (
		count       int
		windowStart time.Time
	)

	err := l.db.QueryRow(ctx, query, key).Scan(&count, &windowStart)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return l.limit
		}
		return 0
	}

	if now.Sub(windowStart) >= l.window {
		return l.limit
	}

	rem := l.limit - count
	if rem < 0 {
		return 0
	}
	return rem
}

// Reset clears the rate limit state for key.
func (l *PGRateLimiter) Reset(key string) {
	if key == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()

	query := fmt.Sprintf("DELETE FROM %s WHERE key = $1", l.tableName)
	_, _ = l.db.Exec(ctx, query, key)
}
