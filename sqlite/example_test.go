// SPDX-License-Identifier: MIT

package sqlite_test

import (
	"fmt"

	"github.com/abeta-dev/go-libs/sqlite"
)

func ExampleCheckpointMode() {
	fmt.Println(sqlite.DefaultJournalMode, sqlite.SyncNormal, sqlite.CheckpointTruncate)
	// Output:
	// WAL NORMAL TRUNCATE
}
