// SPDX-License-Identifier: MIT

// Package sqlite provides a zero-dependency, driver-agnostic embedded SQLite
// WAL helper over database/sql with busy_timeout tuning, BEGIN IMMEDIATE
// write-serialized transactions, PRAGMA user_version schema migrations, WAL
// checkpointing, and atomic VACUUM INTO backups for desktop and edge Go services.
package sqlite
