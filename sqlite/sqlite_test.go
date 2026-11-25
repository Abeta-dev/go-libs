// SPDX-License-Identifier: MIT

package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSQLiteDriver simulates SQLite PRAGMAs, BEGIN IMMEDIATE, user_version,
// VACUUM INTO, and wal_checkpoint over stdlib database/sql/driver without CGo.
type fakeSQLiteDriver struct {
	mu           sync.Mutex
	openErr      error
	pingErr      error
	failExecSub  string
	failQuerySub string
	failCommit   bool
	failBegin    bool
	userVersion  int
	executed     []string
}

func (d *fakeSQLiteDriver) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.openErr = nil
	d.pingErr = nil
	d.failExecSub = ""
	d.failQuerySub = ""
	d.failCommit = false
	d.failBegin = false
	d.userVersion = 0
	d.executed = nil
}

func (d *fakeSQLiteDriver) Open(_ string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.openErr != nil {
		return nil, d.openErr
	}
	return &fakeSQLiteConn{drv: d}, nil
}

type fakeSQLiteConn struct {
	drv *fakeSQLiteDriver
}

func (c *fakeSQLiteConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *fakeSQLiteConn) Close() error { return nil }
func (c *fakeSQLiteConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeSQLiteConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.drv.mu.Lock()
	defer c.drv.mu.Unlock()
	if c.drv.failBegin {
		return nil, errors.New("begin boom")
	}
	return &fakeSQLiteTx{drv: c.drv}, nil
}

func (c *fakeSQLiteConn) Ping(_ context.Context) error {
	c.drv.mu.Lock()
	defer c.drv.mu.Unlock()
	return c.drv.pingErr
}

func (c *fakeSQLiteConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.drv.mu.Lock()
	defer c.drv.mu.Unlock()
	c.drv.executed = append(c.drv.executed, query)
	if c.drv.failExecSub != "" && strings.Contains(query, c.drv.failExecSub) {
		return nil, errors.New("exec failed: " + query)
	}
	if strings.HasPrefix(query, "PRAGMA user_version = ") {
		var v int
		for _, ch := range strings.TrimPrefix(query, "PRAGMA user_version = ") {
			v = v*10 + int(ch-'0')
		}
		c.drv.userVersion = v
	}
	if strings.HasPrefix(query, "VACUUM INTO '") && strings.HasSuffix(query, "'") {
		rawPath := strings.TrimSuffix(strings.TrimPrefix(query, "VACUUM INTO '"), "'")
		rawPath = strings.ReplaceAll(rawPath, "''", "'")
		if err := os.WriteFile(rawPath, []byte("SQLite format 3\x00backup"), 0o600); err != nil {
			return nil, err
		}
	}
	return driver.RowsAffected(1), nil
}

func (c *fakeSQLiteConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.drv.mu.Lock()
	defer c.drv.mu.Unlock()
	c.drv.executed = append(c.drv.executed, query)
	if c.drv.failQuerySub != "" && strings.Contains(query, c.drv.failQuerySub) {
		return nil, errors.New("query failed: " + query)
	}
	if query == "PRAGMA user_version" {
		return &fakeRows{cols: []string{"user_version"}, rows: [][]driver.Value{{int64(c.drv.userVersion)}}}, nil
	}
	if strings.HasPrefix(query, "PRAGMA wal_checkpoint(") {
		return &fakeRows{
			cols: []string{"busy", "log", "checkpointed"},
			rows: [][]driver.Value{{int64(0), int64(12), int64(12)}},
		}, nil
	}
	return &fakeRows{cols: []string{"ok"}, rows: [][]driver.Value{{int64(1)}}}, nil
}

type fakeSQLiteTx struct{ drv *fakeSQLiteDriver }

func (t *fakeSQLiteTx) Commit() error {
	t.drv.mu.Lock()
	defer t.drv.mu.Unlock()
	if t.drv.failCommit {
		return errors.New("commit boom")
	}
	return nil
}
func (t *fakeSQLiteTx) Rollback() error { return nil }

type fakeRows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("rand boom") }

var testDrv = &fakeSQLiteDriver{}

func init() {
	sql.Register("sqlite", testDrv)
}

func TestOpenWrapAndPragmas(t *testing.T) {
	testDrv.reset()
	ctx := context.Background()

	// 1. Open with auto-detected "sqlite" driver and file path (creates parent dir)
	dbPath := filepath.Join(t.TempDir(), "nested", "app.db")
	db, err := Open(ctx, dbPath,
		nil,
		WithBusyTimeout(3*time.Second),
		WithJournalMode("wal"),
		WithSynchronous(SyncNormal),
		WithForeignKeys(true),
		WithTempStoreMemory(true),
		WithCacheSizeKB(32000),
		WithMaxOpenConns(2),
		WithEnsureDir(true),
	)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if db.SQL() == nil {
		t.Fatal("SQL() returned nil")
	}
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, "SELECT 1"); err != nil {
		t.Fatalf("ExecContext failed: %v", err)
	}
	rows, err := db.QueryContext(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("QueryContext failed: %v", err)
	}
	_ = rows.Close()
	var one int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("QueryRowContext failed: err=%v one=%d", err, one)
	}

	// Verify WAL & busy_timeout PRAGMAs executed
	joined := strings.Join(testDrv.executed, "\n")
	for _, want := range []string{
		"PRAGMA busy_timeout = 3000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -32000",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing pragma %q in:\n%s", want, joined)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 2. Read-only + foreign_keys=false + :memory:
	testDrv.reset()
	roDB, err := Open(ctx, ":memory:",
		WithDriver("sqlite"),
		WithReadOnly(true),
		WithForeignKeys(false),
		WithTempStoreMemory(false),
	)
	if err != nil {
		t.Fatalf("read-only Open failed: %v", err)
	}
	_ = roDB.Close()
	roJoined := strings.Join(testDrv.executed, "\n")
	if strings.Contains(roJoined, "PRAGMA journal_mode") || !strings.Contains(roJoined, "PRAGMA foreign_keys = OFF") {
		t.Fatalf("unexpected read-only pragmas:\n%s", roJoined)
	}

	// 3. Error branches: empty path, invalid config, missing driver, Wrap nil, mkdir fail, open fail, ping fail, pragma fail
	if _, err := Open(ctx, "  "); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty path, got %v", err)
	}
	if _, err := Open(ctx, ":memory:", WithJournalMode("BOGUS")); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for bogus journal_mode, got %v", err)
	}
	if _, err := Open(ctx, ":memory:", WithSynchronous("BOGUS")); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for bogus synchronous, got %v", err)
	}
	if _, err := resolveDriver("", []string{"postgres"}); !errors.Is(err, ErrNoSQLiteDriver) {
		t.Fatalf("expected ErrNoSQLiteDriver, got %v", err)
	}
	if _, err := Open(ctx, ":memory:", WithDriver("nonexistent-driver")); err == nil {
		t.Fatal("expected sql.Open error for unknown driver")
	}

	// MkdirAll failure (parent path is a regular file)
	blocker := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(blocker, []byte("x"), 0o600)
	if _, err := Open(ctx, filepath.Join(blocker, "child.db")); err == nil {
		t.Fatal("expected MkdirAll failure")
	}

	// Wrap nil and invalid config
	if _, err := Wrap(ctx, nil); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for nil sqlDB, got %v", err)
	}
	rawDB, _ := sql.Open("sqlite", ":memory:")
	defer func() { _ = rawDB.Close() }()
	if _, err := Wrap(ctx, rawDB, WithJournalMode("NOPE")); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig on Wrap invalid config, got %v", err)
	}

	// Ping error
	testDrv.reset()
	testDrv.pingErr = errors.New("ping fail")
	if _, err := Open(ctx, ":memory:"); err == nil {
		t.Fatal("expected ping failure")
	}

	// Pragma exec error
	testDrv.reset()
	testDrv.failExecSub = "PRAGMA journal_mode"
	if _, err := Open(ctx, ":memory:"); err == nil {
		t.Fatal("expected pragma exec failure")
	}

	// Auto-detect error via Open when candidates don't match
	origCandidates := defaultDriverCandidates
	defaultDriverCandidates = []string{"nonexistent_sqlite"}
	if _, err := Open(ctx, ":memory:"); !errors.Is(err, ErrNoSQLiteDriver) {
		defaultDriverCandidates = origCandidates
		t.Fatalf("expected ErrNoSQLiteDriver from Open, got %v", err)
	}
	defaultDriverCandidates = origCandidates
}

func TestTransactionsAndContextQuerier(t *testing.T) {
	testDrv.reset()
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// GetQuerier fallback when unset
	if got := GetQuerier(ctx, db); got != db {
		t.Fatal("expected fallback db from GetQuerier")
	}

	// 1. WithImmediateTx success + context propagation
	err = db.WithImmediateTx(ctx, func(txCtx context.Context, q Querier) error {
		if GetQuerier(txCtx, db) != q {
			t.Fatal("expected context querier to equal active immediate conn")
		}
		_, execErr := q.ExecContext(txCtx, "INSERT INTO items VALUES (1)")
		return execErr
	})
	if err != nil {
		t.Fatalf("WithImmediateTx failed: %v", err)
	}

	// 2. WithImmediateTx nil fn, callback error, panic rollback, commit error, begin error, conn error
	if err := db.WithImmediateTx(ctx, nil); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for nil fn, got %v", err)
	}
	wantErr := errors.New("business rollback")
	if err := db.WithImmediateTx(ctx, func(_ context.Context, _ Querier) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != "tx-panic" {
				t.Fatalf("expected panic propagation, got %v", r)
			}
		}()
		_ = db.WithImmediateTx(ctx, func(_ context.Context, _ Querier) error {
			panic("tx-panic")
		})
	}()

	testDrv.failExecSub = "COMMIT"
	if err := db.WithImmediateTx(ctx, func(_ context.Context, _ Querier) error { return nil }); err == nil {
		t.Fatal("expected commit error")
	}
	testDrv.failExecSub = "BEGIN IMMEDIATE"
	if err := db.WithImmediateTx(ctx, func(_ context.Context, _ Querier) error { return nil }); err == nil {
		t.Fatal("expected begin immediate error")
	}
	testDrv.failExecSub = ""

	closedDB, _ := Open(ctx, ":memory:")
	_ = closedDB.Close()
	if err := closedDB.WithImmediateTx(ctx, func(_ context.Context, _ Querier) error { return nil }); err == nil {
		t.Fatal("expected conn acquire error on closed DB")
	}

	// 3. WithTx success, nil fn, error rollback, panic rollback, begin error, commit error
	if err := db.WithTx(ctx, nil); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for nil WithTx fn, got %v", err)
	}
	if err := db.WithTx(ctx, func(txCtx context.Context, tx *sql.Tx) error {
		if GetQuerier(txCtx, db) != tx {
			t.Fatal("expected context querier to equal sql.Tx")
		}
		return nil
	}); err != nil {
		t.Fatalf("WithTx failed: %v", err)
	}
	if err := db.WithTx(ctx, func(_ context.Context, _ *sql.Tx) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr in WithTx, got %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != "std-tx-panic" {
				t.Fatalf("expected panic propagation in WithTx, got %v", r)
			}
		}()
		_ = db.WithTx(ctx, func(_ context.Context, _ *sql.Tx) error { panic("std-tx-panic") })
	}()

	testDrv.failBegin = true
	if err := db.WithTx(ctx, func(_ context.Context, _ *sql.Tx) error { return nil }); err == nil {
		t.Fatal("expected BeginTx error")
	}
	testDrv.failBegin = false
	testDrv.failCommit = true
	if err := db.WithTx(ctx, func(_ context.Context, _ *sql.Tx) error { return nil }); err == nil {
		t.Fatal("expected Commit error")
	}
	testDrv.failCommit = false
}

func TestMigrationsBackupAndCheckpoint(t *testing.T) {
	testDrv.reset()
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// 1. Migrations validation errors
	if _, err := db.Migrate(ctx, []Migration{{Version: 0, SQL: "CREATE TABLE t(id INT)"}}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected version <= 0 error, got %v", err)
	}
	if _, err := db.Migrate(ctx, []Migration{
		{Version: 1, SQL: "SELECT 1"},
		{Version: 1, SQL: "SELECT 2"},
	}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected duplicate version error, got %v", err)
	}
	if _, err := db.Migrate(ctx, []Migration{{Version: 1, Name: "empty"}}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected empty migration error, got %v", err)
	}

	// 2. Apply out-of-order migrations -> sorted & applied atomically, idempotent on re-run
	var callbackRan bool
	migrations := []Migration{
		{
			Version: 2,
			Name:    "seed_settings",
			Up: func(txCtx context.Context, q Querier) error {
				callbackRan = true
				_, e := q.ExecContext(txCtx, "INSERT INTO settings VALUES ('k', 'v')")
				return e
			},
		},
		{
			Version: 1,
			Name:    "init_schema",
			SQL:     "CREATE TABLE settings (k TEXT PRIMARY KEY, v TEXT)",
		},
	}

	res1, err := db.Migrate(ctx, migrations)
	if err != nil || res1.PreviousVersion != 0 || res1.CurrentVersion != 2 || res1.AppliedCount != 2 || !callbackRan {
		t.Fatalf("unexpected Migrate result: res=%+v err=%v callback=%v", res1, err, callbackRan)
	}

	// Idempotent second run applies 0 migrations
	res2, err := db.Migrate(ctx, migrations)
	if err != nil || res2.AppliedCount != 0 || res2.CurrentVersion != 2 {
		t.Fatalf("expected idempotent 0 applied, got res=%+v err=%v", res2, err)
	}

	// Migration error paths: UserVersion query error, SQL error, callback error, set user_version error
	testDrv.failQuerySub = "PRAGMA user_version"
	if _, err := db.Migrate(ctx, migrations); err == nil {
		t.Fatal("expected UserVersion error")
	}
	testDrv.failQuerySub = ""

	testDrv.failExecSub = "CREATE TABLE broken"
	if _, err := db.Migrate(ctx, []Migration{{Version: 3, Name: "bad_sql", SQL: "CREATE TABLE broken"}}); err == nil {
		t.Fatal("expected migration SQL error")
	}
	testDrv.failExecSub = ""

	if _, err := db.Migrate(ctx, []Migration{{
		Version: 3,
		Name:    "bad_cb",
		Up:      func(_ context.Context, _ Querier) error { return errors.New("cb boom") },
	}}); err == nil {
		t.Fatal("expected migration callback error")
	}

	testDrv.failExecSub = "PRAGMA user_version = 3"
	if _, err := db.Migrate(ctx, []Migration{{Version: 3, Name: "bad_uv", SQL: "SELECT 1"}}); err == nil {
		t.Fatal("expected set user_version error")
	}
	testDrv.failExecSub = ""

	// 3. Atomic VACUUM INTO Backup (success + error paths)
	backupPath := filepath.Join(t.TempDir(), "backups", "snap'1.db")
	if err := db.Backup(ctx, backupPath); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	data, err := os.ReadFile(backupPath)
	if err != nil || !strings.HasPrefix(string(data), "SQLite format 3") {
		t.Fatalf("backup file invalid: err=%v data=%q", err, string(data))
	}

	if err := db.Backup(ctx, ""); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty backup path, got %v", err)
	}
	if err := db.Backup(ctx, "bad\x00path.db"); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for null-byte backup path, got %v", err)
	}
	blocker := filepath.Join(t.TempDir(), "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0o600)
	if err := db.Backup(ctx, filepath.Join(blocker, "b.db")); err == nil {
		t.Fatal("expected backup mkdir error")
	}

	origRand := randReader
	randReader = errReader{}
	if err := db.Backup(ctx, filepath.Join(t.TempDir(), "r.db")); err == nil {
		randReader = origRand
		t.Fatal("expected randReader error in Backup")
	}
	randReader = origRand

	testDrv.failExecSub = "VACUUM INTO"
	if err := db.Backup(ctx, filepath.Join(t.TempDir(), "v.db")); err == nil {
		t.Fatal("expected VACUUM INTO error")
	}
	testDrv.failExecSub = ""

	origRename := renameFile
	renameFile = func(_, _ string) error { return errors.New("rename boom") }
	if err := db.Backup(ctx, filepath.Join(t.TempDir(), "ren.db")); err == nil {
		renameFile = origRename
		t.Fatal("expected rename error")
	}
	renameFile = origRename

	// 4. WAL Checkpoint (success + invalid mode + query error)
	stats, err := db.Checkpoint(ctx, CheckpointTruncate)
	if err != nil || stats.Busy != 0 || stats.LogFrames != 12 || stats.CheckpointedFrames != 12 {
		t.Fatalf("unexpected Checkpoint stats: %+v err=%v", stats, err)
	}
	if _, err := db.Checkpoint(ctx, CheckpointMode("INVALID")); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for invalid checkpoint mode, got %v", err)
	}
	testDrv.failQuerySub = "PRAGMA wal_checkpoint"
	if _, err := db.Checkpoint(ctx, CheckpointPassive); err == nil {
		t.Fatal("expected checkpoint query error")
	}
	testDrv.failQuerySub = ""
}
