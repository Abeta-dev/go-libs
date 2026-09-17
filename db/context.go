// SPDX-License-Identifier: MIT

package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is an interface that both *pgxpool.Pool, *pgxpool.Conn, and pgx.Tx implement for query execution.
//
//nolint:revive // DBTX is the standard interface name in the Go PostgreSQL ecosystem
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CopyDBTX extends DBTX with high-throughput batch CopyFrom operations.
type CopyDBTX interface {
	DBTX
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

type contextKey string

// ConnKey is the context key used to store and retrieve active database connections.
const ConnKey contextKey = "db_conn"

// GetQuerier retrieves the tenant-aware *pgxpool.Conn from the context if it exists,
// otherwise falls back to the global *pgxpool.Pool.
func GetQuerier(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if conn, ok := ctx.Value(ConnKey).(*pgxpool.Conn); ok {
		return conn
	}
	return pool
}
