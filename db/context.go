// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"errors"
	"fmt"

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

// TxBeginner is implemented by *pgxpool.Pool and *pgxpool.Conn to start a database transaction.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type contextKey string

// ConnKey is the context key used to store and retrieve active database connections or transactions.
const ConnKey contextKey = "db_conn"

// WithQuerier returns a derived context carrying the given DBTX (*pgxpool.Conn or pgx.Tx).
func WithQuerier(ctx context.Context, q DBTX) context.Context {
	return context.WithValue(ctx, ConnKey, q)
}

// GetQuerier retrieves the active DBTX (*pgxpool.Conn or pgx.Tx) from the context if present,
// otherwise falls back to the provided *pgxpool.Pool.
func GetQuerier(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if conn, ok := ctx.Value(ConnKey).(*pgxpool.Conn); ok {
		return conn
	}
	if q, ok := ctx.Value(ConnKey).(DBTX); ok {
		return q
	}
	return pool
}

// WithTx executes fn inside a PostgreSQL transaction started from pool.
// The active pgx.Tx is automatically injected into the context passed to fn so downstream
// repositories calling db.GetQuerier(ctx, pool) participate in the same transaction.
// If fn returns an error or panics, the transaction is rolled back; otherwise it is committed.
func WithTx(ctx context.Context, pool TxBeginner, fn func(ctx context.Context, tx pgx.Tx) error) (err error) {
	if pool == nil {
		return errors.New("db: transaction beginner pool is nil")
	}
	if fn == nil {
		return errors.New("db: transaction callback fn is nil")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	txCtx := WithQuerier(ctx, tx)
	if err = fn(txCtx, tx); err != nil {
		return err
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("db: commit transaction: %w", commitErr)
	}
	return nil
}
