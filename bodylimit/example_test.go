// SPDX-License-Identifier: MIT

package bodylimit_test

import (
	"fmt"

	"github.com/umesh0492/go-libs/bodylimit"
)

func ExampleString() {
	fmt.Println(bodylimit.String(2 << 20))
	fmt.Println(bodylimit.String(64 << 10))
	fmt.Println(bodylimit.String(512))
	// Output:
	// 2 MB
	// 64 KB
	// 512 B
}
