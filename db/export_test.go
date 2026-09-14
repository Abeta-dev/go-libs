// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetNewWithConfig(f func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error)) {
	newWithConfig = f
}

func SetPingPool(f func(ctx context.Context, pool *pgxpool.Pool) error) {
	pingPool = f
}

func ResetNewWithConfig() {
	newWithConfig = pgxpool.NewWithConfig
}

func ResetPingPool() {
	pingPool = func(ctx context.Context, pool *pgxpool.Pool) error {
		return pool.Ping(ctx)
	}
}
