// SPDX-License-Identifier: MIT

package uuidutil_test

import (
	"fmt"

	"github.com/Abeta-dev/go-libs/uuidutil"
)

func ExampleParse() {
	id, err := uuidutil.Parse("550e8400-e29b-41d4-a716-446655440000")
	fmt.Printf("valid=%t nil=%t err=%v\n", uuidutil.IsValid(id.String()), uuidutil.IsNil(id), err)
	// Output:
	// valid=true nil=false err=<nil>
}
