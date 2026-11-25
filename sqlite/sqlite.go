// SPDX-License-Identifier: MIT

package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// SynchronousMode controls SQLite's PRAGMA synchronous setting.
type SynchronousMode string

const (
	// SyncOff disables fsync calls (fastest, unsafe on OS crash).
	SyncOff SynchronousMode = "OFF"
	// SyncNormal is the recommended durable mode when journal_mode=WAL.
	SyncNormal SynchronousMode = "NORMAL"
	// SyncFull fsyncs every transaction commit.
	SyncFull SynchronousMode = "FULL"
)

// CheckpointMode controls SQLite's PRAGMA wal_checkpoint(<mode>) behavior.
type CheckpointMode string

const (
	// CheckpointPassive checkpoints as many frames as possible without waiting.
	CheckpointPassive CheckpointMode = "PASSIVE"
	// CheckpointFull waits for readers and checkpoints all frames.
	CheckpointFull CheckpointMode = "FULL"
	// CheckpointRestart waits for readers and resets the WAL write pointer.
	CheckpointRestart CheckpointMode = "RESTART"
	// CheckpointTruncate checkpoints all frames and truncates the WAL file to zero bytes.
	CheckpointTruncate CheckpointMode = "TRUNCATE"
)

const (
	// DefaultBusyTimeout is the default PRAGMA busy_timeout duration (5 seconds).
	DefaultBusyTimeout = 5 * time.Second
	// DefaultJournalMode is the default SQLite journal mode ("WAL").
	DefaultJournalMode = "WAL"
)

var (
	// ErrNoSQLiteDriver is returned when no registered SQLite database/sql driver can be auto-detected.
	ErrNoSQLiteDriver = errors.New("sqlite: no registered SQLite driver found (register e.g. modernc.org/sqlite or mattn/go-sqlite3, or pass WithDriver)")
	// ErrInvalidConfig is returned when a configuration option or migration specification is invalid.
	ErrInvalidConfig = errors.New("sqlite: invalid configuration")
)

var (
	defaultDriverCandidates           = []string{"sqlite", "sqlite3", "sqlite3_custom"}
	randReader              io.Reader = rand.Reader
	renameFile                        = os.Rename
)

// Querier is implemented by *sql.DB, *sql.Conn, *sql.Tx, and *sqlite.DB.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type querierContextKey struct{}

// WithQuerier returns a derived context carrying q (typically an active
// BEGIN IMMEDIATE *sql.Conn or *sql.Tx).
func WithQuerier(ctx context.Context, q Querier) context.Context {
	return context.WithValue(ctx, querierContextKey{}, q)
}

// GetQuerier returns the context-bound Querier if present, otherwise fallback.
func GetQuerier(ctx context.Context, fallback Querier) Querier {
	if q, ok := ctx.Value(querierContextKey{}).(Querier); ok && q != nil {
		return q
	}
	return fallback
}

// Config controls SQLite connection tuning and PRAGMA initialization.
type Config struct {
	Driver          string
	BusyTimeout     time.Duration
	JournalMode     string
	Synchronous     SynchronousMode
	ForeignKeys     bool
	TempStoreMemory bool
	CacheSizeKB     int
	MaxOpenConns    int
	ReadOnly        bool
	EnsureDir       bool
}

// Option modifies Config for Open and Wrap.
type Option func(*Config)

// WithDriver explicitly selects the registered database/sql driver name
// (for example "sqlite" for modernc.org/sqlite or "sqlite3" for mattn/go-sqlite3).
func WithDriver(name string) Option {
	return func(c *Config) {
		c.Driver = strings.TrimSpace(name)
	}
}

// WithBusyTimeout configures PRAGMA busy_timeout. Defaults to 5 seconds.
func WithBusyTimeout(d time.Duration) Option {
	return func(c *Config) {
		if d >= 0 {
			c.BusyTimeout = d
		}
	}
}

// WithJournalMode configures PRAGMA journal_mode (WAL, DELETE, TRUNCATE, PERSIST, MEMORY, OFF).
func WithJournalMode(mode string) Option {
	return func(c *Config) {
		c.JournalMode = strings.ToUpper(strings.TrimSpace(mode))
	}
}

// WithSynchronous configures PRAGMA synchronous (OFF, NORMAL, FULL). Defaults to SyncNormal.
func WithSynchronous(mode SynchronousMode) Option {
	return func(c *Config) {
		c.Synchronous = SynchronousMode(strings.ToUpper(strings.TrimSpace(string(mode))))
	}
}

// WithForeignKeys enables or disables PRAGMA foreign_keys. Defaults to true.
func WithForeignKeys(enabled bool) Option {
	return func(c *Config) {
		c.ForeignKeys = enabled
	}
}

// WithTempStoreMemory sets PRAGMA temp_store = MEMORY when enabled. Defaults to true.
func WithTempStoreMemory(enabled bool) Option {
	return func(c *Config) {
		c.TempStoreMemory = enabled
	}
}

// WithCacheSizeKB configures PRAGMA cache_size = -<kb> (KiB) when kb > 0.
func WithCacheSizeKB(kb int) Option {
	return func(c *Config) {
		if kb > 0 {
			c.CacheSizeKB = kb
		}
	}
}

// WithMaxOpenConns overrides sql.DB.SetMaxOpenConns. Defaults to 1 to prevent
// SQLITE_BUSY write contention on single-file SQLite databases.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.MaxOpenConns = n
		}
	}
}

// WithReadOnly skips write-mode journal/synchronous PRAGMAs during initialization.
func WithReadOnly(ro bool) Option {
	return func(c *Config) {
		c.ReadOnly = ro
	}
}

// WithEnsureDir controls automatic creation of missing parent directories for file-backed DBs.
// Defaults to true.
func WithEnsureDir(enabled bool) Option {
	return func(c *Config) {
		c.EnsureDir = enabled
	}
}

// DB wraps a tuned SQLite *sql.DB providing WAL helpers, BEGIN IMMEDIATE
// transactions, PRAGMA user_version migrations, and atomic VACUUM INTO backups.
type DB struct {
	sqlDB *sql.DB
}

var _ Querier = (*DB)(nil)

func defaultConfig() Config {
	return Config{
		BusyTimeout:     DefaultBusyTimeout,
		JournalMode:     DefaultJournalMode,
		Synchronous:     SyncNormal,
		ForeignKeys:     true,
		TempStoreMemory: true,
		MaxOpenConns:    1,
		EnsureDir:       true,
	}
}

func validateConfig(cfg Config) error {
	switch cfg.JournalMode {
	case "WAL", "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "OFF":
	default:
		return fmt.Errorf("%w: unsupported journal_mode %q", ErrInvalidConfig, cfg.JournalMode)
	}
	switch cfg.Synchronous {
	case SyncOff, SyncNormal, SyncFull:
	default:
		return fmt.Errorf("%w: unsupported synchronous mode %q", ErrInvalidConfig, cfg.Synchronous)
	}
	return nil
}

func resolveDriver(explicit string, registered []string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	for _, candidate := range defaultDriverCandidates {
		if slices.Contains(registered, candidate) {
			return candidate, nil
		}
	}
	return "", ErrNoSQLiteDriver
}

func isMemoryPath(dsn string) bool {
	lower := strings.ToLower(strings.TrimSpace(dsn))
	return lower == ":memory:" || strings.HasPrefix(lower, "file::memory:") || strings.Contains(lower, "mode=memory")
}

// Open opens a SQLite database at path using the configured or auto-detected
// database/sql driver, creates parent directories when appropriate, tunes
// connection pooling, and applies WAL/busy_timeout/foreign_keys PRAGMAs.
func Open(ctx context.Context, path string, opts ...Option) (*DB, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: database path is required", ErrInvalidConfig)
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	driverName, err := resolveDriver(cfg.Driver, sql.Drivers())
	if err != nil {
		return nil, err
	}

	if cfg.EnsureDir && !cfg.ReadOnly && !isMemoryPath(trimmed) && !strings.HasPrefix(trimmed, "file:") {
		if dir := filepath.Dir(trimmed); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return nil, fmt.Errorf("sqlite: create parent directory: %w", err)
			}
		}
	}

	rawDB, err := sql.Open(driverName, trimmed)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", driverName, err)
	}

	wrapped, err := Wrap(ctx, rawDB, opts...)
	if err != nil {
		_ = rawDB.Close()
		return nil, err
	}
	return wrapped, nil
}

// Wrap applies SQLite connection pooling and WAL/PRAGMA initialization to an
// existing *sql.DB.
func Wrap(ctx context.Context, sqlDB *sql.DB, opts ...Option) (*DB, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("%w: sql.DB is nil", ErrInvalidConfig)
	}
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxOpenConns)

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	d := &DB{sqlDB: sqlDB}
	if err := d.applyPragmas(ctx, cfg); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) applyPragmas(ctx context.Context, cfg Config) error {
	pragmas := buildPragmas(cfg)
	for _, stmt := range pragmas {
		if _, err := d.sqlDB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("sqlite: apply %s: %w", stmt, err)
		}
	}
	return nil
}

func buildPragmas(cfg Config) []string {
	pragmas := make([]string, 0, 6)
	pragmas = append(pragmas, fmt.Sprintf("PRAGMA busy_timeout = %d", cfg.BusyTimeout.Milliseconds()))
	if cfg.ForeignKeys {
		pragmas = append(pragmas, "PRAGMA foreign_keys = ON")
	} else {
		pragmas = append(pragmas, "PRAGMA foreign_keys = OFF")
	}
	if cfg.TempStoreMemory {
		pragmas = append(pragmas, "PRAGMA temp_store = MEMORY")
	}
	if !cfg.ReadOnly {
		pragmas = append(pragmas,
			"PRAGMA journal_mode = "+cfg.JournalMode,
			"PRAGMA synchronous = "+string(cfg.Synchronous),
		)
	}
	if cfg.CacheSizeKB > 0 {
		pragmas = append(pragmas, fmt.Sprintf("PRAGMA cache_size = -%d", cfg.CacheSizeKB))
	}
	return pragmas
}

// SQL returns the underlying *sql.DB handle.
func (d *DB) SQL() *sql.DB { return d.sqlDB }

// Close closes the underlying *sql.DB pool.
func (d *DB) Close() error { return d.sqlDB.Close() }

// Ping verifies database connectivity.
func (d *DB) Ping(ctx context.Context) error { return d.sqlDB.PingContext(ctx) }

// ExecContext implements Querier.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.sqlDB.ExecContext(ctx, query, args...)
}

// QueryContext implements Querier.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.sqlDB.QueryContext(ctx, query, args...)
}

// QueryRowContext implements Querier.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.sqlDB.QueryRowContext(ctx, query, args...)
}

// WithImmediateTx executes fn inside a SQLite BEGIN IMMEDIATE transaction on a
// dedicated connection, acquiring the RESERVED write lock immediately so
// concurrent writers honor busy_timeout instead of deadlocking on read->write
// upgrades. The active connection is bound to ctx via WithQuerier.
func (d *DB) WithImmediateTx(ctx context.Context, fn func(ctx context.Context, q Querier) error) error {
	if fn == nil {
		return fmt.Errorf("%w: transaction callback is nil", ErrInvalidConfig)
	}

	conn, err := d.sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlite: acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("sqlite: begin immediate: %w", err)
	}

	rollbackCtx := context.WithoutCancel(ctx)
	defer func() {
		if p := recover(); p != nil {
			_, _ = conn.ExecContext(rollbackCtx, "ROLLBACK")
			panic(p)
		}
	}()

	txCtx := WithQuerier(ctx, conn)
	if err := fn(txCtx, conn); err != nil {
		_, _ = conn.ExecContext(rollbackCtx, "ROLLBACK")
		return err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		_, _ = conn.ExecContext(rollbackCtx, "ROLLBACK")
		return fmt.Errorf("sqlite: commit immediate tx: %w", err)
	}
	return nil
}

// WithTx executes fn inside a standard database/sql transaction with context
// querier propagation and automatic error/panic rollback.
func (d *DB) WithTx(ctx context.Context, fn func(ctx context.Context, tx *sql.Tx) error) error {
	if fn == nil {
		return fmt.Errorf("%w: transaction callback is nil", ErrInvalidConfig)
	}

	tx, err := d.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(WithQuerier(ctx, tx), tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit tx: %w", err)
	}
	return nil
}

// UserVersion reads SQLite's built-in integer PRAGMA user_version.
func (d *DB) UserVersion(ctx context.Context) (int, error) {
	var version int
	if err := d.sqlDB.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("sqlite: read user_version: %w", err)
	}
	return version, nil
}

// Migration defines a single forward-only schema migration tracked via SQLite
// PRAGMA user_version.
type Migration struct {
	Version int
	Name    string
	SQL     string
	Up      func(ctx context.Context, q Querier) error
}

// MigrationResult summarizes the outcome of DB.Migrate.
type MigrationResult struct {
	PreviousVersion int
	CurrentVersion  int
	AppliedCount    int
}

// Migrate validates and applies pending migrations in ascending Version order,
// executing each migration and updating PRAGMA user_version atomically inside
// a BEGIN IMMEDIATE transaction.
func (d *DB) Migrate(ctx context.Context, migrations []Migration) (MigrationResult, error) {
	sorted, err := validateAndSortMigrations(migrations)
	if err != nil {
		return MigrationResult{}, err
	}

	current, err := d.UserVersion(ctx)
	if err != nil {
		return MigrationResult{}, err
	}

	res := MigrationResult{
		PreviousVersion: current,
		CurrentVersion:  current,
	}

	for _, m := range sorted {
		if m.Version <= res.CurrentVersion {
			continue
		}
		if err := d.applySingleMigration(ctx, m); err != nil {
			return res, err
		}
		res.CurrentVersion = m.Version
		res.AppliedCount++
	}
	return res, nil
}

func validateAndSortMigrations(migrations []Migration) ([]Migration, error) {
	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	slices.SortFunc(sorted, func(a, b Migration) int {
		return a.Version - b.Version
	})

	seen := make(map[int]struct{}, len(sorted))
	for _, m := range sorted {
		if m.Version <= 0 {
			return nil, fmt.Errorf("%w: migration version must be > 0, got %d", ErrInvalidConfig, m.Version)
		}
		if _, dup := seen[m.Version]; dup {
			return nil, fmt.Errorf("%w: duplicate migration version %d", ErrInvalidConfig, m.Version)
		}
		if strings.TrimSpace(m.SQL) == "" && m.Up == nil {
			return nil, fmt.Errorf("%w: migration %d must define SQL or Up callback", ErrInvalidConfig, m.Version)
		}
		seen[m.Version] = struct{}{}
	}
	return sorted, nil
}

func (d *DB) applySingleMigration(ctx context.Context, m Migration) error {
	return d.WithImmediateTx(ctx, func(txCtx context.Context, q Querier) error {
		if strings.TrimSpace(m.SQL) != "" {
			if _, err := q.ExecContext(txCtx, m.SQL); err != nil {
				return fmt.Errorf("sqlite: migration %d (%s) SQL: %w", m.Version, m.Name, err)
			}
		}
		if m.Up != nil {
			if err := m.Up(txCtx, q); err != nil {
				return fmt.Errorf("sqlite: migration %d (%s) callback: %w", m.Version, m.Name, err)
			}
		}
		if _, err := q.ExecContext(txCtx, fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
			return fmt.Errorf("sqlite: set user_version=%d: %w", m.Version, err)
		}
		return nil
	})
}

// Backup creates a consistent, atomic online backup at destPath using SQLite's
// VACUUM INTO command written to a sibling temporary file and atomically renamed
// into place. Existing files at destPath are never partially overwritten on error.
func (d *DB) Backup(ctx context.Context, destPath string) error {
	trimmed := strings.TrimSpace(destPath)
	if trimmed == "" || strings.ContainsRune(trimmed, '\x00') {
		return fmt.Errorf("%w: invalid backup destination path", ErrInvalidConfig)
	}

	dir := filepath.Dir(trimmed)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("sqlite: create backup directory: %w", err)
	}

	var suffix [8]byte
	if _, err := io.ReadFull(randReader, suffix[:]); err != nil {
		return fmt.Errorf("sqlite: generate backup temp suffix: %w", err)
	}
	tmpPath := filepath.Join(dir, "."+filepath.Base(trimmed)+".tmp-"+hex.EncodeToString(suffix[:]))

	escaped := strings.ReplaceAll(tmpPath, "'", "''")
	// #nosec G202 -- SQLite VACUUM INTO literal path has null bytes rejected and single quotes escaped per SQL standard.
	if _, err := d.sqlDB.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sqlite: vacuum into backup: %w", err)
	}

	if err := renameFile(tmpPath, trimmed); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sqlite: atomic backup rename: %w", err)
	}
	return nil
}

// CheckpointStats holds the three columns returned by PRAGMA wal_checkpoint.
type CheckpointStats struct {
	Busy               int
	LogFrames          int
	CheckpointedFrames int
}

// Checkpoint executes PRAGMA wal_checkpoint(<mode>) and returns the checkpoint statistics.
func (d *DB) Checkpoint(ctx context.Context, mode CheckpointMode) (CheckpointStats, error) {
	normalized := CheckpointMode(strings.ToUpper(strings.TrimSpace(string(mode))))
	switch normalized {
	case CheckpointPassive, CheckpointFull, CheckpointRestart, CheckpointTruncate:
	default:
		return CheckpointStats{}, fmt.Errorf("%w: unsupported checkpoint mode %q", ErrInvalidConfig, mode)
	}

	var stats CheckpointStats
	err := d.sqlDB.QueryRowContext(ctx, "PRAGMA wal_checkpoint("+string(normalized)+")").
		Scan(&stats.Busy, &stats.LogFrames, &stats.CheckpointedFrames)
	if err != nil {
		return CheckpointStats{}, fmt.Errorf("sqlite: wal_checkpoint(%s): %w", normalized, err)
	}
	return stats, nil
}
