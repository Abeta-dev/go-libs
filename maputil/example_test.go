// SPDX-License-Identifier: MIT

package maputil_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/maputil"
)

func ExampleMerge() {
	defaults := map[string]string{"host": "localhost", "port": "8080"}
	overrides := map[string]string{"port": "9000"}
	config := maputil.Merge(defaults, overrides)
	fmt.Printf("host=%s port=%s\n", config["host"], config["port"])
	// Output:
	// host=localhost port=9000
}

func ExampleFilter() {
	scores := map[string]int{"alice": 95, "bob": 60, "carol": 88}
	passing := maputil.Filter(scores, func(_ string, score int) bool {
		return score >= 70
	})
	fmt.Printf("passing count: %d\n", len(passing))
	// Output:
	// passing count: 2
}
