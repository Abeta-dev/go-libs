// SPDX-License-Identifier: MIT

package jwks_test

import (
	"fmt"

	"github.com/Abeta-dev/go-libs/jwks"
)

func ExampleNewResolver() {
	resolver := jwks.NewResolver()
	fmt.Printf("resolver initialized: %t\n", resolver != nil)
	// Output:
	// resolver initialized: true
}
