// SPDX-License-Identifier: MIT

package db_test

import (
	"context"
	"fmt"

	"github.com/Abeta-dev/go-libs/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ExampleGetQuerier() {
	ctx := context.Background()
	var pool *pgxpool.Pool

	// Without active connection in context, returns pool fallback
	querier := db.GetQuerier(ctx, pool)
	fmt.Printf("fallback querier is pool: %v\n", querier == pool)

	// Output:
	// fallback querier is pool: true
}
