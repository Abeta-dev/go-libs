# sqlite

Lightweight, zero-dependency, driver-agnostic embedded SQLite WAL helper built over Go's standard `database/sql` for desktop and edge Go services.

## Features

- **Driver-Agnostic & Zero CGo Lock-In**: Pure Go stdlib `database/sql` Depth-0 leaf package — works with `modernc.org/sqlite`, `github.com/mattn/go-sqlite3`, `github.com/ncruces/go-sqlite3`, or any registered SQLite driver.
- **WAL & `busy_timeout` Tuning**: Automatically applies `PRAGMA busy_timeout`, `PRAGMA journal_mode = WAL`, `PRAGMA synchronous = NORMAL`, `PRAGMA foreign_keys = ON`, `PRAGMA temp_store = MEMORY`, and single-writer connection pooling (`SetMaxOpenConns(1)`).
- **`BEGIN IMMEDIATE` Write Transactions**: `WithImmediateTx` pins a `*sql.Conn` and executes `BEGIN IMMEDIATE` upfront so concurrent writers wait on `busy_timeout` instead of deadlocking on deferred read-to-write lock upgrades.
- **`PRAGMA user_version` Migrations**: `Migrate` runs forward-only schema migrations atomically inside `BEGIN IMMEDIATE` transactions.
- **Atomic `VACUUM INTO` Backups**: `Backup` writes a consistent snapshot to a sibling temp file via `VACUUM INTO` and atomically renames it into place.
- **WAL Checkpointing**: `Checkpoint(ctx, sqlite.CheckpointTruncate)` cleanly compacts the WAL file during graceful shutdown.

## Usage

```go
// Blank-import your preferred SQLite driver in main.go, e.g.:
//   _ "modernc.org/sqlite"
db, err := sqlite.Open(ctx, "/path/to/app.db",
	sqlite.WithBusyTimeout(5*time.Second),
	sqlite.WithSynchronous(sqlite.SyncNormal),
)
if err != nil {
	log.Fatal(err)
}
defer db.Close()

_, err = db.Migrate(ctx, []sqlite.Migration{
	{Version: 1, Name: "create_items", SQL: `CREATE TABLE items (id TEXT PRIMARY KEY, body TEXT NOT NULL)`},
})
```
